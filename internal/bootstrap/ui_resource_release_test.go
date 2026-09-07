package bootstrap_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	apprepo "emisell.app/platform/internal/app/repository"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/bootstrap"
	"emisell.app/platform/internal/developer"
	devrepo "emisell.app/platform/internal/developer/postgres"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/oauth/appclient"
	"emisell.app/platform/pkg/uiresource"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
)

func TestUIResourcePortalRoutes(t *testing.T) {
	f := setup(t)
	dev := portalAccount(t, f, "developer", "developer")
	admin := portalAccount(t, f, "admin", "administrator")
	_, signer, _ := ed25519.GenerateKey(rand.Reader)
	base := f.server.Config.Handler
	f.server.Close()
	f.server = httptest.NewServer(bootstrap.AddUIResourceReleaseRoutes(base, f.pool, origin, slog.New(slog.NewTextHandler(io.Discard, nil)), signer))
	t.Cleanup(f.server.Close)
	dev.server = f.server
	admin.server = f.server
	body := service.UIResourceReleaseInput{Version: "0.1.0", Name: "Products", Summary: "Read-only", Mode: "embedded", URL: "https://app.example.com/", Reason: "Test", RequiredScopes: []string{"read_products"}}
	path := "/api/v1/developer/ui-resource-releases"
	created := pexpect(t, dev, "POST", path, body, key(), 200)
	if created["installable"] != false {
		t.Fatal("submission grants access")
	}
	id := created["release"].(map[string]any)["id"].(string)
	pexpect(t, dev, "GET", path+"/"+id, nil, "", 200)
	pexpect(t, admin, "POST", "/api/v1/admin/ui-resource-releases/"+id+"/status", service.CatalogAction{Revision: 1, Status: "approved", Reason: "reviewed"}, key(), 200)
	result := pexpect(t, admin, "POST", "/api/v1/admin/ui-resource-releases/"+id+"/status", service.CatalogAction{Revision: 2, Status: "signed", Reason: "signed"}, key(), 200)
	if result["installable"] != false {
		t.Fatal("signature grants access")
	}
	body.RequiredScopes = []string{"read_orders"}
	pexpect(t, dev, "POST", path, body, key(), 400)
}

// Exercise the actual portal composition, not only the release adapter. Resource
// clients must bind the permission-bearing digest and must not become verified
// merely because an administrator signed the release.
func TestUIResourceClientPortalComposition(t *testing.T) {
	f := setup(t)
	dev := portalAccount(t, f, "developer", "developer")
	admin := portalAccount(t, f, "admin", "administrator")
	_, signer, _ := ed25519.GenerateKey(rand.Reader)
	f.server.Close()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	base := bootstrap.HandlerWithReviewedUIRuntime(f.pool, f.pool, f.pool, origin, logger, nil, nil, nil, nil, false, bootstrap.EmbeddedReviewConfig{ResourceReleaseKey: signer})
	f.server = httptest.NewServer(bootstrap.AddUIResourceReleaseRoutes(base, f.pool, origin, logger, signer))
	t.Cleanup(f.server.Close)
	dev.server, admin.server = f.server, f.server
	created := pexpect(t, dev, "POST", "/api/v1/developer/ui-resource-releases", service.UIResourceReleaseInput{Version: "0.1.0", Name: "Products", Summary: "Read only", Mode: "embedded", URL: "https://app.example.com/", Reason: "test", RequiredScopes: []string{"read_products"}}, key(), 200)
	id := created["release"].(map[string]any)["id"].(string)
	path := "/api/v1/developer/app-clients"
	pexpect(t, dev, "POST", path, appclient.Input{ReleaseID: id}, key(), 409)
	pexpect(t, admin, "POST", "/api/v1/admin/ui-resource-releases/"+id+"/status", service.CatalogAction{Revision: 1, Status: "approved", Reason: "review"}, key(), 200)
	signed := pexpect(t, admin, "POST", "/api/v1/admin/ui-resource-releases/"+id+"/status", service.CatalogAction{Revision: 2, Status: "signed", Reason: "sign"}, key(), 200)
	client := clientView(t, pexpect(t, dev, "POST", path, appclient.Input{ReleaseID: id}, key(), 200))
	digest := signed["release"].(map[string]any)["package"].(map[string]any)["sha256"].(string)
	if client.Client.Binding.Digest != digest || client.Client.Binding.ReleaseID != id || client.Client.Status == "verified" {
		t.Fatal("resource client binding or proof boundary incorrect")
	}
	input := service.AssignmentInput{ReleaseKind: "ui_resource", ReleaseID: id, MerchantID: f.tenant, Reason: "Local product test"}
	requestKey := key()
	requested := pexpect(t, dev, "POST", "/api/v1/developer/test-assignments", input, requestKey, 200)["assignment"].(map[string]any)
	assignmentID := requested["id"].(string)
	if requested["releaseKind"] != "ui_resource" || requested["releaseSha256"] != digest {
		t.Fatal("wrong assignment source")
	}
	again := pexpect(t, dev, "POST", "/api/v1/developer/test-assignments", input, requestKey, 200)["assignment"].(map[string]any)
	if again["id"] != assignmentID {
		t.Fatal("duplicate assignment on retry")
	}
	testingService := service.Testing{Repo: apprepo.Postgres{Pool: f.pool}, Resources: service.UIResourceReleases{Repo: apprepo.Postgres{Pool: f.caps}, Key: signer}}
	list := func(merchant string) []service.TestApp {
		rows, _, err := testingService.ForMerchant(t.Context(), merchant, "", 20)
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	if len(list(f.tenant)) != 0 {
		t.Fatal("unapproved assignment visible")
	}
	pexpect(t, admin, "POST", "/api/v1/admin/test-assignments/"+assignmentID+"/status", service.AssignmentAction{Revision: 1, Status: "approved", Reason: "Approve local test"}, key(), 200)
	rows := list(f.tenant)
	if len(rows) != 1 || rows[0].AppName != "Products" || rows[0].ExecutionProfile != "resource-app/v1" || rows[0].Readiness.Installable || !rows[0].Readiness.ConfigurationReady || !rows[0].Readiness.RequiredScopesReady {
		t.Fatal("incorrect resource distribution", rows)
	}
	if len(list(f.other)) != 0 {
		t.Fatal("cross merchant assignment leak")
	}
	if _, err := f.pool.Exec(t.Context(), `UPDATE platform_app.test_assignments SET resource_release_id=NULL WHERE id=$1`, assignmentID); err == nil {
		t.Fatal("mutable resource source")
	}
	page, _, err := testingService.ForMerchant(t.Context(), f.tenant, assignmentID, 1)
	if err != nil || len(page) != 0 {
		t.Fatal("cursor not applied", err)
	}
	pexpect(t, admin, "POST", "/api/v1/admin/test-assignments/"+assignmentID+"/status", service.AssignmentAction{Revision: 2, Status: "revoked", Reason: "Revoke test"}, key(), 200)
	if len(list(f.tenant)) != 0 {
		t.Fatal("revoked assignment visible")
	}
	pexpect(t, admin, "POST", "/api/v1/admin/test-assignments/"+assignmentID+"/status", service.AssignmentAction{Revision: 3, Status: "approved", Reason: "Cannot revive"}, key(), 409)
}

func TestUIResourceAuthoring(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	dev := portalAccount(t, f, "developer", "developer")
	other := portalAccount(t, f, "developer", "developer")
	admin := portalAccount(t, f, "admin", "administrator")
	_, keypair, _ := ed25519.GenerateKey(rand.Reader)
	s := service.UIResourceReleases{Repo: apprepo.Postgres{Pool: f.pool}, Developers: developer.Service{Repo: devrepo.Repository{Pool: f.pool}}, Key: keypair}
	p := identity.PortalPrincipal{ID: dev.user, Surface: "developer", Role: "developer"}
	input := service.UIResourceReleaseInput{Version: "0.1.0", Name: "Products demo", Summary: "Read-only", Mode: "embedded", URL: "https://app.example.com/", Reason: "Test", RequiredScopes: []string{"read_products"}}
	k := key()
	v, err := s.Submit(ctx, p, k, input)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Submit(ctx, p, k, input)
	if err != nil || again.ID != v.ID {
		t.Fatal("idempotency", err)
	}
	if _, err = s.Get(ctx, identity.PortalPrincipal{ID: other.user, Surface: "developer", Role: "developer"}, v.ID); err == nil {
		t.Fatal("tenant leak")
	}
	bad := input
	bad.RequiredScopes = []string{"read_orders"}
	if _, err = s.Submit(ctx, p, key(), bad); err == nil {
		t.Fatal("unsupported scope accepted")
	}
	a := identity.PortalPrincipal{ID: admin.user, Surface: "admin", Role: "administrator"}
	if _, err = s.Decide(ctx, a, v.ID, key(), service.CatalogAction{Revision: 1, Status: "signed", Reason: "skip"}); err == nil {
		t.Fatal("review bypass")
	}
	v, err = s.Decide(ctx, a, v.ID, key(), service.CatalogAction{Revision: 1, Status: "approved", Reason: "reviewed"})
	if err != nil {
		t.Fatal(err)
	}
	v, err = s.Decide(ctx, a, v.ID, key(), service.CatalogAction{Revision: 2, Status: "signed", Reason: "signed"})
	if err != nil {
		t.Fatal(err)
	}
	if v.Package == nil || uiresource.Verify(*v.Package, keypair.Public().(ed25519.PublicKey)) != nil {
		t.Fatal("invalid signature")
	}
	if _, err = s.Decide(ctx, a, v.ID, key(), service.CatalogAction{Revision: 2, Status: "suspended", Reason: "stale"}); err == nil {
		t.Fatal("stale accepted")
	}
	// The database forbids changing scopes after submission, even through raw SQL.
	if _, err = f.pool.Exec(ctx, `UPDATE platform_app.ui_resource_releases SET document=jsonb_set(document,'{manifest,requiredScopes}','["read_orders"]'::jsonb) WHERE id=$1`, v.ID); err == nil {
		t.Fatal("mutable permissions")
	}
}
