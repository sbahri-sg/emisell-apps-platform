package bootstrap_test

import (
	"connectrpc.com/connect"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/bootstrap"
	"emisell.app/platform/internal/review"
	"emisell.app/platform/pkg/sdk"
	testingv1 "emisell.app/platform/pkg/sdk/gen/emisell/testing/v1"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"sync"
	"testing"
)

func testingFixture(t *testing.T) (*fixture, *fixture, *fixture, *fixture, review.IntegrationSigner) {
	f := setup(t)
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	signer := review.IntegrationSigner{Key: key}
	f.server.Close()
	f.server = httptest.NewServer(bootstrap.HandlerWithReleases(f.pool, f.caps, origin, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, signer))
	t.Cleanup(f.server.Close)
	return f, portalAccount(t, f, "developer", "developer"), portalAccount(t, f, "developer", "developer"), portalAccount(t, f, "admin", "administrator"), signer
}
func approvedTestAssignment(t *testing.T, dev, admin *fixture, merchant string) string {
	release := signedClientRelease(t, dev, admin)
	a := pexpect(t, dev, "POST", "/api/v1/developer/test-assignments", service.AssignmentInput{ReleaseID: release, MerchantID: merchant, Reason: "Isolated merchant test"}, key(), 200)["assignment"].(map[string]any)
	id := a["id"].(string)
	pexpect(t, admin, "POST", "/api/v1/admin/test-assignments/"+id+"/status", service.AssignmentAction{Status: "approved", Revision: 1, Reason: "Known isolated merchant"}, key(), 200)
	return id
}
func TestTestingDistribution(t *testing.T) {
	f, dev, other, admin, signer := testingFixture(t)
	ctx := context.Background()
	reviewer := portalAccount(t, f, "admin", "reviewer")
	operator := portalAccount(t, f, "admin", "operator")
	release := signedClientRelease(t, dev, admin)
	base := "/api/v1/developer/test-assignments"
	adminBase := "/api/v1/admin/test-assignments"
	body := service.AssignmentInput{ReleaseID: release, MerchantID: f.tenant, Reason: "Test this immutable version"}
	requestKey := key()
	pexpect(t, other, "POST", base, body, key(), 404)
	pexpect(t, dev, "POST", base, body, "", 400)
	a := pexpect(t, dev, "POST", base, body, requestKey, 200)["assignment"].(map[string]any)
	id := a["id"].(string)
	if pexpect(t, dev, "POST", base, body, requestKey, 200)["assignment"].(map[string]any)["id"] != id {
		t.Fatal("duplicate request")
	}
	pexpect(t, dev, "POST", base, body, key(), 409)
	changed := body
	changed.MerchantID = f.other
	pexpect(t, dev, "POST", base, changed, requestKey, 409)
	pexpect(t, other, "GET", base+"/"+id, nil, "", 404)
	if len(pexpect(t, other, "GET", base, nil, "", 200)["assignments"].([]any)) != 0 {
		t.Fatal("cross organization leak")
	}
	pexpect(t, dev, "GET", base+"?merchantId="+f.tenant, nil, "", 400)
	pexpect(t, dev, "GET", base+"?pageSize=21", nil, "", 400)
	pexpect(t, f, "GET", adminBase, nil, "", 401)
	action := service.AssignmentAction{Status: "approved", Revision: 1, Reason: "Known merchant approved"}
	approvalKey := key()
	pexpect(t, reviewer, "POST", adminBase+"/"+id+"/status", action, key(), 403)
	pexpect(t, operator, "POST", adminBase+"/"+id+"/status", action, key(), 403)
	if code, _, _, _ := dev.call("POST", base+"/"+id+"/status", action, key(), developerOrigin); code != 404 {
		t.Fatal("developer can approve", code)
	}
	_, _, secret, _ := lifecycleClient(t, f)
	rpc := httptest.NewServer(bootstrap.InternalHandlerWithReleases(f.pool, f.caps, slog.New(slog.NewTextHandler(io.Discard, nil)), signer))
	defer rpc.Close()
	client, err := sdk.NewLocalClient(rpc.URL, secret)
	if err != nil {
		t.Fatal(err)
	}
	list := func(merchant string) *testingv1.ListAssignmentsResponse {
		t.Helper()
		v, e := client.TestDistribution.ListAssignments(ctx, connect.NewRequest(&testingv1.ListAssignmentsRequest{MerchantId: merchant, CoreActorId: "merchant-manager"}))
		if e != nil {
			t.Fatal(e)
		}
		return v.Msg
	}
	if len(list(f.tenant).Apps) != 0 {
		t.Fatal("unapproved distribution")
	}
	// Concurrent duplicate decisions produce one receipt and audit, no double approval.
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, _, _, _ := admin.call("POST", adminBase+"/"+id+"/status", action, approvalKey, origin)
			codes <- code
		}()
	}
	wg.Wait()
	close(codes)
	for c := range codes {
		if c != 200 {
			t.Fatal("decision replay", c)
		}
	}
	page := list(f.tenant)
	if len(page.Apps) != 1 || page.Apps[0].AssignmentId != id || page.Apps[0].Readiness.Installable || !page.Apps[0].Readiness.ConfigurationReady {
		t.Fatal("assignment readiness", page)
	}
	if len(list(f.other).Apps) != 0 {
		t.Fatal("merchant isolation")
	}
	_, err = client.TestDistribution.ListAssignments(ctx, connect.NewRequest(&testingv1.ListAssignmentsRequest{MerchantId: f.tenant}))
	rpcCode(t, err, connect.CodeInvalidArgument)
	_, err = client.TestDistribution.ListAssignments(ctx, connect.NewRequest(&testingv1.ListAssignmentsRequest{MerchantId: "unknown-merchant", CoreActorId: "staff"}))
	rpcCode(t, err, connect.CodeNotFound)
	legacy, _, _ := serviceClient(t, f, f.tenant, intentScopes())
	_, err = legacy.TestDistribution.ListAssignments(ctx, connect.NewRequest(&testingv1.ListAssignmentsRequest{MerchantId: f.tenant, CoreActorId: "staff"}))
	rpcCode(t, err, connect.CodePermissionDenied)
	// Current release status changes readiness, not approval or runtime authorization.
	pexpect(t, admin, "POST", "/api/v1/admin/integration-releases/"+release+"/status", service.IntegrationAction{Status: "suspended", Revision: 3, Reason: "Suspend test configuration"}, key(), 200)
	if list(f.tenant).Apps[0].Readiness.ConfigurationReady {
		t.Fatal("stale release readiness")
	}
	pexpect(t, admin, "POST", adminBase+"/"+id+"/status", service.AssignmentAction{Status: "revoked", Revision: 1, Reason: "stale"}, key(), 409)
	pexpect(t, admin, "POST", adminBase+"/"+id+"/status", service.AssignmentAction{Status: "revoked", Revision: 2, Reason: "End test distribution"}, key(), 200)
	if len(list(f.tenant).Apps) != 0 {
		t.Fatal("revoked assignment leaked")
	}
	if pexpect(t, admin, "POST", adminBase+"/"+id+"/status", action, approvalKey, 200)["assignment"].(map[string]any)["status"] != "revoked" {
		t.Fatal("approval retry resurrected")
	}
	if len(pexpect(t, dev, "GET", base+"/"+id, nil, "", 200)["history"].([]any)) != 3 {
		t.Fatal("duplicate audit")
	}
	// New signed release; developer can request a known-provided string, Admin cannot approve nonexistent merchant.
	release = signedClientRelease(t, dev, admin)
	body.ReleaseID = release
	body.MerchantID = "not-registered"
	unknown := pexpect(t, dev, "POST", base, body, key(), 200)["assignment"].(map[string]any)["id"].(string)
	pexpect(t, admin, "POST", adminBase+"/"+unknown+"/status", action, key(), 404)
	pexpect(t, admin, "POST", adminBase+"/"+unknown+"/status", service.AssignmentAction{Status: "rejected", Revision: 1, Reason: "Merchant unknown"}, key(), 200)
	for i := 0; i < 21; i++ {
		body.MerchantID = fmt.Sprintf("provided-merchant-%02d", i)
		pexpect(t, dev, "POST", base, body, key(), 200)
	}
	first := pexpect(t, dev, "GET", base, nil, "", 200)
	next := first["nextAfterId"].(string)
	if len(first["assignments"].([]any)) != 20 || next == "" {
		t.Fatal("bounded pagination")
	}
	second := pexpect(t, dev, "GET", base+"?afterId="+next, nil, "", 200)
	if len(second["assignments"].([]any)) != 3 || second["nextAfterId"] != "" {
		t.Fatal("pagination tail")
	}
	if _, err = f.pool.Exec(ctx, `UPDATE platform_app.test_assignments SET merchant_id=$2 WHERE id=$1`, id, f.other); err == nil {
		t.Fatal("mutable assignment target")
	}
	var count int
	err = f.pool.QueryRow(ctx, `SELECT count(*) FROM platform_installation.installations WHERE tenant_id=$1`, f.tenant).Scan(&count)
	if err != nil || count != 0 {
		t.Fatal("assignment installed an app", err, count)
	}
}
