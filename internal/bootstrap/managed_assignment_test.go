package bootstrap_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"log/slog"
	"net/http/httptest"
	"slices"
	"testing"

	"connectrpc.com/connect"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/bootstrap"
	"emisell.app/platform/internal/review"
	"emisell.app/platform/pkg/managedshipping"
	"emisell.app/platform/pkg/sdk"
	intentv1 "emisell.app/platform/pkg/sdk/gen/emisell/installation/v1"
	testingv1 "emisell.app/platform/pkg/sdk/gen/emisell/testing/v1"
)

func TestManagedShippingAssignments(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	_, private, _ := ed25519.GenerateKey(rand.Reader)
	signer := review.ManagedShippingSigner{Key: private}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	f.server.Close()
	f.server = httptest.NewServer(bootstrap.HandlerWithManagedShipping(f.pool, f.caps, nil, origin, logger, nil, nil, signer, nil))
	t.Cleanup(f.server.Close)
	dev, other, admin := portalAccount(t, f, "developer", "developer"), portalAccount(t, f, "developer", "developer"), portalAccount(t, f, "admin", "administrator")
	reviewer := portalAccount(t, f, "admin", "reviewer")
	doc := portalDocument()
	doc.Endpoint = ""
	doc.Scopes = []string{"shipping.read"}
	appID := pexpect(t, dev, "POST", "/api/v1/developer/apps", service.SaveDraft{Document: doc}, key(), 200)["app"].(map[string]any)["id"].(string)
	releaseID := pexpect(t, dev, "POST", "/api/v1/developer/managed-shipping-releases", service.ManagedShippingInput{AppID: appID, DraftRevision: 1, Binding: managedshipping.Binding{Engine: "api-kurir", ProviderCode: "emisell"}, Reason: "Isolated assignment test"}, key(), 200)["release"].(map[string]any)["id"].(string)
	base := "/api/v1/developer/test-assignments"
	adminBase := "/api/v1/admin/test-assignments"
	releasePath := "/api/v1/admin/managed-shipping-releases/" + releaseID + "/status"
	input := service.AssignmentInput{ReleaseKind: "managed_shipping", ReleaseID: releaseID, MerchantID: f.tenant, Reason: "Merchant requested this test"}
	pexpect(t, dev, "POST", base, input, key(), 409) // not signed
	pexpect(t, admin, "POST", releasePath, service.CatalogAction{Status: "approved", Revision: 1, Reason: "Binding reviewed"}, key(), 200)
	pexpect(t, admin, "POST", releasePath, service.CatalogAction{Status: "signed", Revision: 2, Reason: "Sign for testing"}, key(), 200)
	pexpect(t, other, "POST", base, input, key(), 404)
	invalid := input
	invalid.ReleaseKind = "remote"
	pexpect(t, dev, "POST", base, invalid, key(), 400)
	invalid = input
	invalid.ReleaseKind = ""
	pexpect(t, dev, "POST", base, invalid, key(), 404) // no kind/name guessing
	requestKey := key()
	a := pexpect(t, dev, "POST", base, input, requestKey, 200)["assignment"].(map[string]any)
	id := a["id"].(string)
	if a["releaseKind"] != "managed_shipping" || a["releaseId"] != releaseID {
		t.Fatal("lost source binding", a)
	}
	pexpect(t, dev, "POST", base, input, key(), 409)
	pexpect(t, other, "GET", base+"/"+id, nil, "", 404)
	if len(pexpect(t, other, "GET", base, nil, "", 200)["assignments"].([]any)) != 0 {
		t.Fatal("cross-org list")
	}
	decision := service.AssignmentAction{Status: "approved", Revision: 1, Reason: "Known merchant"}
	pexpect(t, reviewer, "POST", adminBase+"/"+id+"/status", decision, key(), 403)
	unknown := input
	unknown.MerchantID = "unknown-merchant"
	unknownID := pexpect(t, dev, "POST", base, unknown, key(), 200)["assignment"].(map[string]any)["id"].(string)
	pexpect(t, admin, "POST", adminBase+"/"+unknownID+"/status", decision, key(), 404)
	_, _, secret, _ := lifecycleClient(t, f)
	rpc := httptest.NewServer(bootstrap.InternalHandlerWithManagedReleases(f.pool, f.caps, logger, nil, signer))
	defer rpc.Close()
	client, err := sdk.NewLocalClient(rpc.URL, secret)
	if err != nil {
		t.Fatal(err)
	}
	list := func(merchant string) *testingv1.ListAssignmentsResponse {
		t.Helper()
		r, e := client.TestDistribution.ListAssignments(ctx, connect.NewRequest(&testingv1.ListAssignmentsRequest{MerchantId: merchant, CoreActorId: "staff"}))
		if e != nil {
			t.Fatal(e)
		}
		return r.Msg
	}
	if len(list(f.tenant).Apps) != 0 {
		t.Fatal("unapproved assignment exposed")
	}
	approvalKey := key()
	pexpect(t, admin, "POST", adminBase+"/"+id+"/status", decision, approvalKey, 200)
	apps := list(f.tenant).Apps
	if len(apps) != 1 || apps[0].AppId != appID || apps[0].Readiness.Installable || !apps[0].Readiness.ConfigurationReady || !slices.Contains(apps[0].Readiness.Blockers, "engine_grant_enforcement_not_available") {
		t.Fatal("managed readiness", apps)
	}
	if len(list(f.other).Apps) != 0 {
		t.Fatal("cross-merchant assignment")
	}
	_, err = client.InstallIntents.Prepare(ctx, connect.NewRequest(&intentv1.PrepareRequest{MerchantId: f.tenant, CoreActorId: "staff", AppId: appID, Version: "1.0.0", IdempotencyKey: key()}))
	if err == nil {
		t.Fatal("distribution became an executable fixture")
	}
	exerciseManagedInstall(t, f, signer, secret, appID, func() {
		pexpect(t, admin, "POST", releasePath, service.CatalogAction{Status: "suspended", Revision: 3, Reason: "Test current signed status"}, key(), 200)
	})
	// Both original and managed release targets are immutable; the FK is real.
	if _, err = f.pool.Exec(ctx, "UPDATE platform_app.test_assignments SET managed_release_id=NULL,revision=revision+1,status='revoked' WHERE id=$1", id); err == nil {
		t.Fatal("mutable source")
	}
	if _, err = f.pool.Exec(ctx, "INSERT INTO platform_app.test_assignments(id,organization_id,managed_release_id,release_sha256,merchant_id,status) VALUES('invalid-source','org','missing',$1,'m','requested')", a["releaseSha256"]); err == nil {
		t.Fatal("missing release accepted")
	}
	if list(f.tenant).Apps[0].Readiness.ConfigurationReady {
		t.Fatal("stale signature readiness")
	}
	// Replays return current durable state even during release/signing outages.
	pexpect(t, dev, "POST", base, input, requestKey, 200)
	pexpect(t, admin, "POST", adminBase+"/"+id+"/status", service.AssignmentAction{Status: "revoked", Revision: 2, Reason: "Test revoked distribution"}, key(), 200)
	replay := pexpect(t, admin, "POST", adminBase+"/"+id+"/status", decision, approvalKey, 200)["assignment"].(map[string]any)
	if replay["status"] != "revoked" {
		t.Fatal("approval replay revived assignment")
	}
	if pexpect(t, dev, "POST", base, input, requestKey, 200)["assignment"].(map[string]any)["status"] != "revoked" {
		t.Fatal("request replay stale")
	}
	if len(list(f.tenant).Apps) != 0 {
		t.Fatal("revoked distribution listed")
	}
	if len(pexpect(t, admin, "GET", adminBase+"/"+id, nil, "", 200)["history"].([]any)) != 3 {
		t.Fatal("duplicate audits")
	}
	var grants int
	if err = f.pool.QueryRow(ctx, "SELECT count(*) FROM platform_installation.installations WHERE app_id=$1 AND status!='uninstalled'", appID).Scan(&grants); err != nil || grants != 0 {
		t.Fatal("managed lifecycle left active installation", err)
	}
}
