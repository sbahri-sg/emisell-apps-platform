package bootstrap_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	apprepo "emisell.app/platform/internal/app/repository"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/bootstrap"
	"emisell.app/platform/internal/identity"
	identityrepo "emisell.app/platform/internal/identity/postgres"
	"emisell.app/platform/internal/installation/domain"
	installrepo "emisell.app/platform/internal/installation/postgres"
	installservice "emisell.app/platform/internal/installation/service"
	"emisell.app/platform/internal/oauth/appclient"
	clientrepo "emisell.app/platform/internal/oauth/appclient/postgres"
	"emisell.app/platform/internal/oauth/embedded"
	launchrepo "emisell.app/platform/internal/oauth/embedded/postgres"
	"emisell.app/platform/internal/transport/connectapi"
	"emisell.app/platform/pkg/uirelease"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
)

func TestUIReleasePortalLifecycle(t *testing.T) {
	for _, mode := range []string{"embedded", "external"} {
		t.Run(mode, func(t *testing.T) { testUIReleasePortalLifecycle(t, mode) })
	}
}

func testUIReleasePortalLifecycle(t *testing.T, mode string) {
	pub, signer, _ := ed25519.GenerateKey(rand.Reader)
	f, dev, other, admin := clientFixture(t, &proofVerifier{}, bootstrap.EmbeddedReviewConfig{Key: signer, ParentOrigin: "https://core.example", UIReleaseKey: signer})
	base := f.server.Config.Handler
	f.server.Close()
	f.server = httptest.NewServer(bootstrap.AddUIReleaseRoutes(base, f.pool, origin, slog.New(slog.NewTextHandler(io.Discard, nil)), signer))
	t.Cleanup(f.server.Close)
	dev.server = f.server
	other.server = f.server
	admin.server = f.server
	b := service.UIReleaseInput{Version: "1.0.0", Name: "UI demo", Summary: "No business data", Mode: "embedded", URL: "https://app.example.com/dashboard", Reason: "Test submission"}
	b.Mode = mode
	path := "/api/v1/developer/ui-releases"
	requestKey := key()
	created := pexpect(t, dev, "POST", path, b, requestKey, 200)
	release := created["release"].(map[string]any)
	id := release["id"].(string)
	if created["installable"] != false {
		t.Fatal("release grants installability")
	}
	if pexpect(t, dev, "POST", path, b, requestKey, 200)["release"].(map[string]any)["id"] != id {
		t.Fatal("retry creates second app")
	}
	pexpect(t, other, "GET", path+"/"+id, nil, "", 404)
	if len(pexpect(t, other, "GET", path, nil, "", 200)["releases"].([]any)) != 0 {
		t.Fatal("list leaks another organization")
	}
	if len(pexpect(t, dev, "GET", path, nil, "", 200)["releases"].([]any)) != 1 {
		t.Fatal("owner list missing release")
	}
	b.AppID = release["manifest"].(map[string]any)["appId"].(string)
	pexpect(t, other, "POST", path, b, key(), 404)
	pexpect(t, dev, "POST", path, b, key(), 409)
	b.Version = "1.0.1"
	pexpect(t, dev, "POST", path, b, key(), 200)
	b.Version = "1.0.0"
	actionPath := "/api/v1/admin/ui-releases/" + id + "/status"
	decision := service.CatalogAction{Revision: 1, Status: "signed", Reason: "Cannot skip review"}
	pexpect(t, admin, "POST", actionPath, decision, key(), 409)
	decision.Status = "approved"
	decision.Reason = "Reviewed"
	approvalKey := key()
	pexpect(t, admin, "POST", actionPath, decision, approvalKey, 200)
	_, missingKeyErr := (service.UIReleases{Repo: apprepo.Postgres{Pool: f.pool}}).Decide(context.Background(), identity.PortalPrincipal{ID: "test-admin", Surface: "admin", Role: "administrator"}, id, key(), service.CatalogAction{Revision: 2, Status: "signed", Reason: "No key"})
	if missingKeyErr == nil {
		t.Fatal("signing without key accepted")
	}
	decision.Status = "signed"
	decision.Revision = 2
	signed := pexpect(t, admin, "POST", actionPath, decision, key(), 200)
	raw, _ := json.Marshal(signed["release"].(map[string]any)["package"])
	var pkg uirelease.Package
	if json.Unmarshal(raw, &pkg) != nil || uirelease.Verify(pkg, pub) != nil {
		t.Fatal("invalid signed package")
	}
	decision.Status = "suspended"
	client := clientView(t, pexpect(t, dev, "POST", "/api/v1/developer/app-clients", appclient.Input{ReleaseID: id}, key(), 200))
	pexpect(t, other, "GET", "/api/v1/developer/app-clients/"+client.Client.ID, nil, "", 404)
	assignment := pexpect(t, dev, "POST", "/api/v1/developer/test-assignments", service.AssignmentInput{ReleaseKind: "ui", ReleaseID: id, MerchantID: f.tenant, Reason: "UI test"}, key(), 200)
	assignmentID := assignment["assignment"].(map[string]any)["id"].(string)
	pexpect(t, admin, "POST", "/api/v1/admin/test-assignments/"+assignmentID+"/status", service.AssignmentAction{Revision: 1, Status: "approved", Reason: "Approved UI test"}, key(), 200)
	client = clientView(t, pexpect(t, dev, "POST", "/api/v1/developer/app-clients/"+client.Client.ID+"/actions", appclient.Action{Revision: 1, Action: "verify", Reason: "Prove UI origin"}, key(), 200))
	client = clientView(t, pexpect(t, dev, "POST", "/api/v1/developer/app-clients/"+client.Client.ID+"/actions", appclient.Action{Revision: client.Client.Revision, Action: "rotate_secret", Reason: "Ready client"}, key(), 200))
	launch := pexpect(t, dev, "POST", "/api/v1/developer/embedded-launches", embedded.LaunchInput{ClientID: client.Client.ID, URL: b.URL, Mode: b.Mode, Reason: "UI launch"}, key(), 200)
	launchID := launch["launch"].(map[string]any)["id"].(string)
	source := bootstrap.ReviewedUIInstallSource{Releases: apprepo.Postgres{Pool: f.pool}, Clients: clientrepo.Repository{Pool: f.pool}, ReleaseKey: pub,
		Current: embedded.CurrentBinding{Clients: appclient.Service{Repo: clientrepo.Repository{Pool: f.pool}}, Launches: launchrepo.Repository{Pool: f.pool}, Key: pub, ParentOrigin: "https://core.example"}}
	ctx := context.Background()
	app := pkg.Manifest.AppID
	if source.WithRelease(ctx, f.tenant, app, b.Version, func(domain.IntentRelease) error { t.Fatal("unreviewed launch allowed"); return nil }) == nil {
		t.Fatal("missing review accepted")
	}
	pexpect(t, admin, "POST", "/api/v1/admin/embedded-launches/"+launchID+"/status", map[string]any{"revision": 1, "status": "approved", "reason": "Reviewed UI"}, key(), 200)
	_, principal, _, _ := lifecycleClient(t, f)
	intents := installservice.Intents{Repo: installrepo.Repository{Pool: f.pool}, Auth: identity.ServiceAccounts{Repo: identityrepo.Repository{Pool: f.pool}}, Managed: source}
	lifecycle := installservice.Lifecycle{Repo: installrepo.Repository{Pool: f.pool}, Intents: intents, ReviewedUIKey: pub}
	composed := connectapi.Server{Intents: intents, Lifecycle: lifecycle, Testing: service.Testing{Repo: apprepo.Postgres{Pool: f.pool}}}
	if err := bootstrap.EnableReviewedUI(&composed, source, signer); err != nil {
		t.Fatal(err)
	}
	intents = composed.Intents
	lifecycle = composed.Lifecycle
	available, _, listErr := composed.Testing.ForMerchant(ctx, f.tenant, "", 20)
	if listErr != nil || len(available) != 1 || !available[0].Readiness.Installable || available[0].ExecutionProfile != domain.ReviewedUIPolicy {
		t.Fatal("UI distribution", available, listErr)
	}
	intent, err := intents.Prepare(ctx, principal, "ui-staff", key(), installservice.PrepareIntent{AppID: app, Version: b.Version})
	if err != nil {
		t.Fatal("real source prepare", err)
	}
	if intent.Release.UIBinding.AssignmentID != assignmentID {
		t.Fatal("assignment provenance missing")
	}
	if _, err = lifecycle.Consume(ctx, principal, "ui-staff", key(), intent.ID, intent.ConsentDigest); err == nil {
		t.Fatal("consent bypass")
	}
	if _, err = intents.Decide(ctx, principal, "ui-staff", key(), installservice.DecideIntent{ID: intent.ID, Digest: intent.ConsentDigest, Decision: "consent"}); err != nil {
		t.Fatal(err)
	}
	installed, err := lifecycle.Consume(ctx, principal, "ui-staff", key(), intent.ID, intent.ConsentDigest)
	if err != nil {
		t.Fatal(err)
	}
	installationID := installed.Access.Installation.ID
	if _, err = lifecycle.Execute(ctx, principal, "ui-staff", key(), installationID, "activate"); err != nil {
		t.Fatal("real source activate", err)
	}
	check := func() error {
		return lifecycle.WithReviewedUIAccess(ctx, principal, "ui-staff", installationID, func(binding domain.UIBinding) error {
			if binding.Launch.URL != b.URL {
				t.Fatal("URL mismatch")
			}
			return nil
		})
	}
	if err = check(); err != nil {
		t.Fatal("real source access", err)
	}
	currentAccess, err := lifecycle.Get(ctx, principal, "ui-staff", installationID)
	if err != nil || currentAccess.ReviewedUILaunch == nil {
		t.Fatal("missing ephemeral launch", err)
	}
	pexpect(t, admin, "POST", "/api/v1/admin/test-assignments/"+assignmentID+"/status", service.AssignmentAction{Revision: 2, Status: "revoked", Reason: "Replace distribution"}, key(), 200)
	if check() == nil {
		t.Fatal("revoked assignment allowed access")
	}
	currentAccess, err = lifecycle.Get(ctx, principal, "ui-staff", installationID)
	if err != nil || currentAccess.ReviewedUILaunch != nil {
		t.Fatal("revoked source exposed launch", err)
	}
	replacement := pexpect(t, dev, "POST", "/api/v1/developer/test-assignments", service.AssignmentInput{ReleaseKind: "ui", ReleaseID: id, MerchantID: f.tenant, Reason: "New assignment requires new consent"}, key(), 200)
	assignmentID = replacement["assignment"].(map[string]any)["id"].(string)
	pexpect(t, admin, "POST", "/api/v1/admin/test-assignments/"+assignmentID+"/status", service.AssignmentAction{Revision: 1, Status: "approved", Reason: "New distribution"}, key(), 200)
	if check() == nil {
		t.Fatal("replacement revived old consent")
	}
	if source.WithRelease(ctx, "other-merchant", app, b.Version, func(domain.IntentRelease) error { t.Fatal("merchant leak"); return nil }) == nil {
		t.Fatal("unassigned merchant accepted")
	}
	decision.Revision = 3
	pexpect(t, admin, "POST", actionPath, decision, key(), 200)
	if check() == nil {
		t.Fatal("suspended source allowed installed access")
	}
	if _, err = lifecycle.Execute(ctx, principal, "ui-staff", key(), installationID, "uninstall"); err != nil {
		t.Fatal("uninstall blocked by suspended source", err)
	}
	pexpect(t, dev, "POST", "/api/v1/developer/app-clients/"+client.Client.ID+"/actions", appclient.Action{Revision: client.Client.Revision, Action: "rotate_secret", Reason: "Suspended must fail"}, key(), 409)
	pexpect(t, admin, "POST", "/api/v1/admin/test-assignments/"+assignmentID+"/status", service.AssignmentAction{Revision: 2, Status: "revoked", Reason: "Revoke during suspension"}, key(), 200)
	decision.Status = "approved"
	decision.Revision = 1
	decision.Reason = "Reviewed"
	replay := pexpect(t, admin, "POST", actionPath, decision, approvalKey, 200)
	if replay["release"].(map[string]any)["status"] != "suspended" {
		t.Fatal("retry resurrected release")
	}
	var count int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM platform_app.ui_release_audit WHERE release_id=$1`, id).Scan(&count); err != nil || count != 4 {
		t.Fatal("audit", count, err)
	}
	if _, err := f.pool.Exec(context.Background(), `UPDATE platform_app.ui_releases SET document=jsonb_set(document,'{manifest,name}','"tampered"') WHERE id=$1`, id); err == nil {
		t.Fatal("immutable manifest changed")
	}
}
