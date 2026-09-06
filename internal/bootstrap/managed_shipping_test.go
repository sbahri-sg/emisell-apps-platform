package bootstrap_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/bootstrap"
	"emisell.app/platform/internal/review"
	"emisell.app/platform/pkg/managedshipping"
	intent "emisell.app/platform/pkg/sdk/gen/emisell/installation/v1"
)

func TestManagedShippingReleasePipeline(t *testing.T) {
	f := setup(t)
	public, keypair, _ := ed25519.GenerateKey(rand.Reader)
	f.server.Close()
	f.server = httptest.NewServer(bootstrap.HandlerWithManagedShipping(f.pool, f.caps, nil, origin, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, review.ManagedShippingSigner{Key: keypair}, nil))
	t.Cleanup(f.server.Close)
	dev := portalAccount(t, f, "developer", "developer")
	other := portalAccount(t, f, "developer", "developer")
	admin := portalAccount(t, f, "admin", "administrator")
	reviewer := portalAccount(t, f, "admin", "reviewer")
	operator := portalAccount(t, f, "admin", "operator")
	doc := portalDocument()
	doc.Endpoint = ""
	doc.Scopes = []string{"shipping.read"}
	draft := pexpect(t, dev, "POST", "/api/v1/developer/apps", service.SaveDraft{Document: doc}, key(), 200)["app"].(map[string]any)
	appID := draft["id"].(string)
	input := service.ManagedShippingInput{AppID: appID, DraftRevision: 1, Binding: managedshipping.Binding{Engine: "api-kurir", ProviderCode: "emisell"}, Reason: "Local provider binding review; no engine execution"}
	path := "/api/v1/developer/managed-shipping-releases"
	pexpect(t, other, "POST", path, input, key(), 404)
	bad := input
	bad.Binding.ProviderCode = "rajaongkir"
	pexpect(t, dev, "POST", path, bad, key(), 400)
	bad = input
	bad.DraftRevision = 2
	pexpect(t, dev, "POST", path, bad, key(), 409)
	createKey := key()
	result := pexpect(t, dev, "POST", path, input, createKey, 200)
	id := result["release"].(map[string]any)["id"].(string)
	adminPath := "/api/v1/admin/managed-shipping-releases/" + id
	pexpect(t, dev, "POST", path, input, key(), 409)
	pexpect(t, other, "GET", path+"/"+id, nil, "", 404)
	if len(pexpect(t, other, "GET", path, nil, "", 200)["releases"].([]any)) != 0 {
		t.Fatal("foreign list disclosure")
	}
	pexpect(t, operator, "GET", adminPath, nil, "", 200)
	pexpect(t, f, "GET", adminPath, nil, "", 401)
	status, _, _, _ := admin.call("GET", adminPath, nil, "", developerOrigin)
	if status != 403 {
		t.Fatal("cross-surface origin")
	}
	act := service.CatalogAction{Status: "approved", Revision: 1, Reason: "Reviewed bound built-in provider"}
	pexpect(t, operator, "POST", adminPath+"/status", act, key(), 403)
	// Developer cannot approve itself. HTTP 404 response may not be JSON.
	status, _, _, _ = dev.call("POST", path+"/"+id+"/status", act, key(), developerOrigin)
	if status != 404 {
		t.Fatal("developer review route")
	}
	pexpect(t, admin, "POST", adminPath+"/status", service.CatalogAction{Status: "signed", Revision: 1, Reason: "Cannot skip review"}, key(), 409)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, _, _, _ := reviewer.call("POST", adminPath+"/status", act, key(), origin)
			codes <- code
		}()
	}
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for c := range codes {
		counts[c]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatal("review race", counts)
	}
	act = service.CatalogAction{Status: "signed", Revision: 2, Reason: "Sign managed provider release"}
	pexpect(t, reviewer, "POST", adminPath+"/status", act, key(), 403)
	missing := httptest.NewServer(bootstrap.Handler(f.pool, f.caps, origin, slog.New(slog.NewTextHandler(io.Discard, nil))))
	defer missing.Close()
	offline := *admin
	offline.server = missing
	pexpect(t, &offline, "POST", adminPath+"/status", act, key(), 503)
	signKey := key()
	signed := pexpect(t, admin, "POST", adminPath+"/status", act, signKey, 200)
	var release service.ManagedShippingRelease
	raw, _ := json.Marshal(signed["release"])
	if json.Unmarshal(raw, &release) != nil || release.Package == nil || managedshipping.Verify(*release.Package, public) != nil {
		t.Fatal("persisted attestation invalid")
	}
	if signed["readiness"].(map[string]any)["configurationReady"] != true || signed["readiness"].(map[string]any)["installable"] != false {
		t.Fatal("release must not bypass engine/distribution gate")
	}
	if pexpect(t, &offline, "GET", adminPath, nil, "", 200)["readiness"].(map[string]any)["configurationReady"] != false {
		t.Fatal("missing trust root accepted")
	}
	for _, column := range []string{"manifest", "package"} {
		if _, err := f.pool.Exec(context.Background(), "UPDATE platform_app.managed_shipping_releases SET "+column+"='{}' WHERE id=$1", id); err == nil {
			t.Fatal("mutable release", column)
		}
	}
	// Changes to a draft cannot change its already signed release or break retries.
	doc.Name = "Edited after submission"
	pexpect(t, dev, "PUT", "/api/v1/developer/apps/"+appID, service.SaveDraft{Revision: 1, Document: doc}, key(), 200)
	if pexpect(t, dev, "POST", path, input, createKey, 200)["release"].(map[string]any)["status"] != "signed" {
		t.Fatal("submit replay did not resolve current state")
	}
	pexpect(t, &offline, "POST", adminPath+"/status", service.CatalogAction{Status: "suspended", Revision: 3, Reason: "Suspend without signer"}, key(), 200)
	if pexpect(t, admin, "POST", adminPath+"/status", act, signKey, 200)["release"].(map[string]any)["status"] != "suspended" {
		t.Fatal("retry resurrected signed release")
	}
	if len(pexpect(t, admin, "GET", adminPath, nil, "", 200)["history"].([]any)) != 4 {
		t.Fatal("audit not atomic/exactly once")
	}
	// Stage 2 does not authorize installations or inject developer releases into fixture registry.
	core, _, _ := serviceClient(t, f, f.tenant, intentScopes())
	_, err := core.InstallIntents.Prepare(context.Background(), connect.NewRequest(&intent.PrepareRequest{CoreActorId: "managed-test", IdempotencyKey: key(), AppId: appID, Version: "1.0.0"}))
	rpcCode(t, err, connect.CodeNotFound)
}
