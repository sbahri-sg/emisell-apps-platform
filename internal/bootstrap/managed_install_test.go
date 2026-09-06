package bootstrap_test

import (
	"bytes"
	"connectrpc.com/connect"
	"context"
	"emisell.app/platform/internal/bootstrap"
	"emisell.app/platform/internal/review"
	"emisell.app/platform/pkg/sdk"
	engine "emisell.app/platform/pkg/sdk/gen/emisell/engine/v1"
	engineconnect "emisell.app/platform/pkg/sdk/gen/emisell/engine/v1/enginev1connect"
	v1 "emisell.app/platform/pkg/sdk/gen/emisell/installation/v1"
	testingv1 "emisell.app/platform/pkg/sdk/gen/emisell/testing/v1"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Called with an approved signed release in the disposable assignment test.
// Uses the real API-Kurir local binary and an independent empty PostgreSQL DB.
func exerciseManagedInstall(t *testing.T, f *fixture, signer review.ManagedShippingSigner, coreKey, appID string, suspend func()) {
	t.Helper()
	binary, dsn := os.Getenv("MANAGED_ENGINE_TEST_BINARY"), os.Getenv("MANAGED_ENGINE_TEST_DATABASE_URL")
	if binary == "" || dsn == "" {
		t.Fatal("managed assignment integration requires isolated engine runner")
	}
	ctx := context.Background()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	engineURL := "http://" + listener.Addr().String()
	listener.Close()
	serviceKey, engineKey := strings.Repeat("b", 64), strings.Repeat("a", 64)
	local, err := (bootstrap.LocalEngineConfig{EngineURL: engineURL, ServiceKey: serviceKey, EngineKey: engineKey}).LocalManaged()
	if err != nil {
		t.Fatal(err)
	}
	handler, err := bootstrap.InternalHandlerWithLocalManaged(f.pool, f.caps, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, signer, local)
	if err != nil {
		t.Fatal(err)
	}
	var unavailable atomic.Bool
	rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if unavailable.Load() {
			http.Error(w, "test outage", http.StatusServiceUnavailable)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	defer rpc.Close()
	cfg, _ := json.Marshal(map[string]string{"EngineURL": engineURL, "PlatformURL": rpc.URL, "DatabaseURL": dsn, "ServiceKey": serviceKey, "EngineKey": engineKey})
	path := filepath.Join(t.TempDir(), "engine.json")
	if err = os.WriteFile(path, cfg, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "-config", path)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cmd.Process.Signal(os.Interrupt); cmd.Wait() }()
	ready := false
	for i := 0; i < 100; i++ {
		req, _ := http.NewRequest("GET", engineURL+"/internal/readiness", nil)
		req.Header.Set("key", serviceKey)
		res, e := http.DefaultClient.Do(req)
		if e == nil {
			res.Body.Close()
			if res.StatusCode == 200 {
				ready = true
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		t.Fatal("isolated engine did not become ready")
	}
	client, err := sdk.NewLocalClient(rpc.URL, coreKey)
	if err != nil {
		t.Fatal(err)
	}
	checkClient := engineconnect.NewEngineGrantServiceClient(rpc.Client(), rpc.URL)
	grant := func(merchant, provider, operation, key string) bool {
		req := connect.NewRequest(&engine.CheckRequest{MerchantId: merchant, ProviderCode: provider, Operation: operation, Environment: "local-isolated"})
		req.Header().Set("Authorization", "Bearer "+key)
		res, e := checkClient.Check(ctx, req)
		return e == nil && res.Msg.Allowed
	}
	if grant(f.tenant, "emisell", "rates.read", engineKey) {
		t.Fatal("assignment granted execution")
	}
	page, err := client.TestDistribution.ListAssignments(ctx, connect.NewRequest(&testingv1.ListAssignmentsRequest{MerchantId: f.tenant, CoreActorId: "staff"}))
	if err != nil || len(page.Msg.Apps) != 1 || !page.Msg.Apps[0].Readiness.Installable {
		t.Fatal("not installable", err)
	}
	prepare := func(merchant string) (*v1.InstallIntent, error) {
		r, e := client.InstallIntents.Prepare(ctx, connect.NewRequest(&v1.PrepareRequest{MerchantId: merchant, CoreActorId: "staff", AppId: appID, Version: "1.0.0", IdempotencyKey: key()}))
		if e != nil {
			return nil, e
		}
		return r.Msg.Intent, nil
	}
	if _, err = prepare(f.other); err == nil {
		t.Fatal("cross merchant preparation")
	}
	v, err := prepare(f.tenant)
	if err != nil {
		t.Fatal("prepare", err)
	}
	if v.InstallationPolicy != "managed-shipping-local/v1" || v.ExecutionProfile != "managed-shipping-local" {
		t.Fatal("fixture fallback")
	}
	req := connect.NewRequest(&v1.ConsumeRequest{MerchantId: f.tenant, CoreActorId: "staff", IntentId: v.Id, ConsentDigest: v.ConsentDigest, IdempotencyKey: key()})
	if _, err = client.Installations.Consume(ctx, req); err == nil {
		t.Fatal("unconsented consume")
	}
	decision := connect.NewRequest(&v1.DecideRequest{MerchantId: f.tenant, CoreActorId: "staff", IntentId: v.Id, ConsentDigest: v.ConsentDigest, Decision: v1.ConsentDecision_CONSENT_DECISION_CONSENT, IdempotencyKey: key()})
	if _, err = client.InstallIntents.Decide(ctx, decision); err != nil {
		t.Fatal("consent", err)
	}
	consumed, err := client.Installations.Consume(ctx, req)
	if err != nil {
		t.Fatal("consume", err)
	}
	id := consumed.Msg.Result.Installation.InstallationId
	if grant(f.tenant, "emisell", "rates.read", engineKey) {
		t.Fatal("pending installation granted execution")
	}
	activated, err := client.Installations.Activate(ctx, activateRequest(f.tenant, id))
	if err != nil {
		t.Fatal("activate", err)
	}
	if len(activated.Msg.Result.Installation.Capabilities) != 0 {
		t.Fatal("install selected checkout route")
	}
	if !grant(f.tenant, "emisell", "rates.read", engineKey) {
		t.Fatal("active grant denied")
	}
	for _, bad := range []struct{ merchant, provider, operation, key string }{{f.other, "emisell", "rates.read", engineKey}, {f.tenant, "rajaongkir", "rates.read", engineKey}, {f.tenant, "emisell", "shipments.write", engineKey}, {f.tenant, "emisell", "rates.read", coreKey}} {
		if grant(bad.merchant, bad.provider, bad.operation, bad.key) {
			t.Fatal("engine boundary failed")
		}
	}
	if _, err = client.Installations.IssueToken(ctx, issueTokenRequest(f.tenant, id)); err == nil {
		t.Fatal("managed grant escaped as app token")
	}
	listed, err := client.Installations.ListInstallations(ctx, connect.NewRequest(&v1.ListInstallationsRequest{MerchantId: f.tenant, CoreActorId: "staff"}))
	if err != nil || len(listed.Msg.Installations) != 1 || listed.Msg.Installations[0].InstallationId != id {
		t.Fatal("installed list", err)
	}
	call := func(method, path string, body any, want int) map[string]any {
		t.Helper()
		raw, _ := json.Marshal(body)
		r, _ := http.NewRequest(method, engineURL+path, bytes.NewReader(raw))
		r.Header.Set("key", serviceKey)
		r.Header.Set("X-Emisell-Merchant-ID", f.tenant)
		r.Header.Set("Content-Type", "application/json")
		res, e := http.DefaultClient.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(io.LimitReader(res.Body, 65536))
		if res.StatusCode != want {
			t.Fatalf("engine %s status %d want %d: %s", path, res.StatusCode, want, data)
		}
		var out map[string]any
		json.Unmarshal(data, &out)
		return out
	}
	catalog := call("GET", "/api/v1/integrations/providers", nil, 200)["data"].(map[string]any)
	if catalog["active_provider_code"] != nil {
		t.Fatal("installation auto-selected provider")
	}
	if len(catalog["providers"].([]any)) != 1 {
		t.Fatal("unmanaged provider leaked")
	}
	call("POST", "/api/v1/integrations/providers/emisell/activate", map[string]any{"expected_version": catalog["version"]}, 200)
	call("PUT", "/api/v1/integrations/shipping-services", map[string]any{"mode": "custom", "services": []map[string]string{{"courier_code": "jne", "service_code": "JTR"}}}, 200)
	input := map[string]any{"origin": "loc_test_jakarta", "destination": "loc_test_bandung", "weight": 10000, "courier": "jne", "options": map[string]any{"include_unverified": true}}
	call("POST", "/api/v1/calculate/district/domestic-cost", input, 200)
	call("POST", "/api/v1/integrations/shipments", map[string]any{}, 404)
	// A warm rate result must not bypass a fresh authorization check.
	unavailable.Store(true)
	call("POST", "/api/v1/calculate/district/domestic-cost", input, 503)
	unavailable.Store(false)
	call("POST", "/api/v1/calculate/district/domestic-cost", input, 200)
	if _, err = client.Installations.Uninstall(ctx, uninstallRequest(f.tenant, id)); err != nil {
		t.Fatal("first uninstall", err)
	}
	call("POST", "/api/v1/calculate/district/domestic-cost", input, 403)
	oldReplay, err := client.Installations.Consume(ctx, req)
	if err != nil || oldReplay.Msg.Result.Installation.Status != "uninstalled" {
		t.Fatal("old consent resurrected installation", err)
	}
	newIntent, err := prepare(f.tenant)
	if err != nil || newIntent.Id == v.Id {
		t.Fatal("reinstall requires fresh intent", err)
	}
	newConsume := connect.NewRequest(&v1.ConsumeRequest{MerchantId: f.tenant, CoreActorId: "staff", IntentId: newIntent.Id, ConsentDigest: newIntent.ConsentDigest, IdempotencyKey: key()})
	if _, err = client.Installations.Consume(ctx, newConsume); err == nil {
		t.Fatal("reinstall skipped consent")
	}
	if _, err = client.InstallIntents.Decide(ctx, connect.NewRequest(&v1.DecideRequest{MerchantId: f.tenant, CoreActorId: "staff", IntentId: newIntent.Id, ConsentDigest: newIntent.ConsentDigest, Decision: v1.ConsentDecision_CONSENT_DECISION_CONSENT, IdempotencyKey: key()})); err != nil {
		t.Fatal("reinstall consent", err)
	}
	reinstalled, err := client.Installations.Consume(ctx, newConsume)
	if err != nil {
		t.Fatal("reinstall consume", err)
	}
	newID := reinstalled.Msg.Result.Installation.InstallationId
	if newID == id {
		t.Fatal("reinstall reused installation")
	}
	call("POST", "/api/v1/calculate/district/domestic-cost", input, 403)
	if _, err = client.Installations.Activate(ctx, activateRequest(f.tenant, newID)); err != nil {
		t.Fatal("reinstall activate", err)
	}
	// Replaying an old uninstall must not revoke the replacement installation.
	if _, err = client.Installations.Uninstall(ctx, uninstallRequest(f.tenant, id)); err != nil {
		t.Fatal("old uninstall replay", err)
	}
	call("POST", "/api/v1/calculate/district/domestic-cost", input, 200)
	id = newID
	suspend()
	if grant(f.tenant, "emisell", "rates.read", engineKey) {
		t.Fatal("suspended release kept effective engine access")
	}
	call("POST", "/api/v1/calculate/district/domestic-cost", input, 403)
	if _, err = client.Installations.Uninstall(ctx, uninstallRequest(f.tenant, id)); err != nil {
		t.Fatal("uninstall", err)
	}
	if grant(f.tenant, "emisell", "rates.read", engineKey) {
		t.Fatal("uninstall kept grant")
	}
	call("POST", "/api/v1/calculate/district/domestic-cost", input, 403)
	replay, err := client.Installations.Consume(ctx, req)
	if err != nil || replay.Msg.Result.Installation.Status != "uninstalled" {
		t.Fatal("replay resurrected installation", err)
	}
	t.Log("Isolated lifecycle: outage denies warm rates, recovery succeeds, uninstall denies, fresh consent/reinstall restores rates, old uninstall cannot revoke replacement; sample rates only")
}
