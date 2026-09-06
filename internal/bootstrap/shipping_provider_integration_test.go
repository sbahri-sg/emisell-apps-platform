package bootstrap_test

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect"
	"emisell.app/platform/internal/bootstrap"
	"emisell.app/platform/internal/runtime/kurir"
	"emisell.app/platform/pkg/appmanifest"
	"emisell.app/platform/pkg/sdk"
	v1 "emisell.app/platform/pkg/sdk/gen/emisell/installation/v1"
	ship "emisell.app/platform/pkg/sdk/gen/emisell/shipping/v1"
)

// Real persistent lifecycle and RPC; only the API-Kurir response is simulated.
// It never calls production or installs/disables an upstream provider credential.
func TestShippingProviderLifecycle(t *testing.T) {
	for _, otherProvider := range []string{"kiriminaja", "emisell"} {
		t.Run("rajaongkir_and_"+otherProvider, func(t *testing.T) {
			testShippingProviderLifecycle(t, otherProvider)
		})
	}
}

func testShippingProviderLifecycle(t *testing.T, otherProvider string) {
	f := setup(t)
	ctx := context.Background()
	for _, provider := range []string{"rajaongkir", otherProvider} {
		m, err := appmanifest.ShippingProviderFixture(provider)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(m)
		k := appmanifest.LocalFixtureKey()
		_, err = f.pool.Exec(ctx, `INSERT INTO platform_app.releases(app_id,version,manifest,signature,public_key)
 VALUES($1,$2,$3::json,$4,$5) ON CONFLICT DO NOTHING`, m.ID, m.Version, string(raw), base64.StdEncoding.EncodeToString(ed25519.Sign(k, raw)), base64.StdEncoding.EncodeToString(k.Public().(ed25519.PublicKey)))
		if err != nil {
			t.Fatal(err)
		}
	}
	_, _, coreKey, _ := lifecycleClient(t, f)
	var rajaInstalled, outage atomic.Bool
	var reads, unexpected atomic.Int64
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		w.Header().Set(kurir.FixtureHeader, appmanifest.KurirFixtureProfile)
		if r.Header.Get("key") != kurir.FixtureKey || r.Header.Get("X-Emisell-Merchant-ID") != f.tenant || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" || r.Method != "GET" || r.URL.RawQuery != "" {
			unexpected.Add(1)
			w.WriteHeader(403)
			return
		}
		if outage.Load() {
			w.WriteHeader(503)
			return
		}
		provider := strings.TrimPrefix(r.URL.Path, "/api/v1/integrations/providers/")
		if provider != "rajaongkir" && provider != otherProvider {
			unexpected.Add(1)
			w.WriteHeader(404)
			return
		}
		// Another provider may be selected while RajaOngkir is separately installed.
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"code": provider, "built_in": provider == "emisell", "available": true, "installed": provider == otherProvider || rajaInstalled.Load(), "active": provider == otherProvider}})
	}))
	t.Cleanup(engine.Close)
	bridge, err := kurir.NewLocalFixture(engine.URL, map[string]int{"origin": 442})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(bridge.Close)
	client := func(enabled bool) *sdk.Client {
		var handler http.Handler
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		if enabled {
			handler = bootstrap.InternalHandlerWithLocalShippingProviders(f.pool, f.caps, logger, bridge)
		} else {
			handler = bootstrap.InternalHandler(f.pool, f.caps, logger)
		}
		server := httptest.NewServer(handler)
		t.Cleanup(server.Close)
		c, err := sdk.NewLocalClient(server.URL, coreKey)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	c := client(true)
	target := func(id string) *v1.InstallationRequest {
		return &v1.InstallationRequest{MerchantId: f.tenant, CoreActorId: "staff", InstallationId: id, IdempotencyKey: key()}
	}
	prepare := func(provider string) *v1.InstallIntent {
		m, _ := appmanifest.ShippingProviderFixture(provider)
		r, err := c.InstallIntents.Prepare(ctx, connect.NewRequest(&v1.PrepareRequest{MerchantId: f.tenant, CoreActorId: "staff", AppId: m.ID, Version: m.Version, IdempotencyKey: key()}))
		if err != nil {
			t.Fatal(err)
		}
		return r.Msg.Intent
	}
	consumeRequest := func(v *v1.InstallIntent) *connect.Request[v1.ConsumeRequest] {
		return connect.NewRequest(&v1.ConsumeRequest{MerchantId: f.tenant, CoreActorId: "staff", IntentId: v.Id, ConsentDigest: v.ConsentDigest, IdempotencyKey: key()})
	}
	consent := func(v *v1.InstallIntent) {
		r := intentDecision(v, v1.ConsentDecision_CONSENT_DECISION_CONSENT)
		r.Msg.MerchantId = f.tenant
		if _, err := c.InstallIntents.Decide(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	list := func(merchant string) []*v1.InstalledApp {
		r, err := c.Installations.ListInstallations(ctx, connect.NewRequest(&v1.ListInstallationsRequest{MerchantId: merchant, CoreActorId: "staff"}))
		if err != nil {
			t.Fatal(err)
		}
		return r.Msg.Installations
	}
	get := func(id string) *v1.InstallationAccess {
		r, err := c.Installations.GetInstallation(ctx, connect.NewRequest(&v1.GetInstallationRequest{Target: target(id)}))
		if err != nil {
			t.Fatal(err)
		}
		return r.Msg.Result.Installation
	}
	v := prepare("rajaongkir")
	rq := consumeRequest(v)
	_, err = c.Installations.Consume(ctx, rq)
	rpcCode(t, err, connect.CodeAlreadyExists)
	if len(list(f.tenant)) != 0 || reads.Load() != 0 {
		t.Fatal("preview installed app or reached engine")
	}
	consent(v)
	consumed, err := c.Installations.Consume(ctx, rq)
	if err != nil {
		t.Fatal(err)
	}
	id := consumed.Msg.Result.Installation.InstallationId
	if a := get(id); a.Status != "pending" || len(a.GrantedScopes) != 0 || len(a.Capabilities) != 0 {
		t.Fatal("premature access/routing")
	}
	replay, err := c.Installations.Consume(ctx, rq)
	if err != nil || !replay.Msg.Result.Replayed || replay.Msg.Result.Installation.InstallationId != id {
		t.Fatal("duplicate consume", err)
	}
	_, err = c.Installations.Activate(ctx, connect.NewRequest(&v1.ActivateRequest{Target: target(id)}))
	rpcCode(t, err, connect.CodeAlreadyExists) // Another provider is installed/active, but RajaOngkir is not installed.
	if get(id).Status != "pending" {
		t.Fatal("another provider activated RajaOngkir")
	}
	rajaInstalled.Store(true)
	_, err = client(false).Installations.Activate(ctx, connect.NewRequest(&v1.ActivateRequest{Target: target(id)}))
	rpcCode(t, err, connect.CodeUnavailable)
	activeRequest := connect.NewRequest(&v1.ActivateRequest{Target: target(id)})
	_, err = c.Installations.Activate(ctx, activeRequest)
	if err != nil {
		t.Fatal(err)
	}
	if a := get(id); a.Status != "active" || a.GrantState != "active" || !slices.Equal(a.GrantedScopes, []string{"shipping.read"}) || len(a.Capabilities) != 0 {
		t.Fatal("wrong provider lifecycle state")
	}
	before := reads.Load()
	_, err = c.Installations.Activate(ctx, activeRequest)
	if err != nil || reads.Load() != before {
		t.Fatal("activation receipt repeated upstream", err)
	}
	second := prepare(otherProvider)
	consent(second)
	r2, err := c.Installations.Consume(ctx, consumeRequest(second))
	if err != nil {
		t.Fatal(err)
	}
	id2 := r2.Msg.Result.Installation.InstallationId
	_, err = c.Installations.Activate(ctx, connect.NewRequest(&v1.ActivateRequest{Target: target(id2)}))
	if err != nil {
		t.Fatal("provider apps must be independently installable", err)
	}
	c = client(true) // no process-local binding; survives recomposition/reload
	if items := list(f.tenant); len(items) != 2 || items[0].AppId == items[1].AppId || items[0].Status != "active" || items[1].Status != "active" {
		t.Fatal("provider identities merged", items)
	}
	if len(list(f.other)) != 0 {
		t.Fatal("foreign merchant leak")
	}
	for _, patch := range []func(*v1.InstallationRequest){func(v *v1.InstallationRequest) { v.MerchantId = f.other }, func(v *v1.InstallationRequest) { v.CoreActorId = "another" }} {
		v := target(id)
		patch(v)
		_, err = c.Installations.Uninstall(ctx, connect.NewRequest(&v1.UninstallRequest{Target: v}))
		rpcCode(t, err, connect.CodeNotFound)
	}
	_, err = c.Installations.IssueToken(ctx, connect.NewRequest(&v1.IssueTokenRequest{Target: target(id)}))
	rpcCode(t, err, connect.CodePermissionDenied)
	_, err = c.Shipping.GetRates(ctx, connect.NewRequest(&ship.GetRatesRequest{MerchantId: f.tenant, WeightGrams: 1000, OriginZone: "origin", DestinationZone: "destination", IdempotencyKey: key()}))
	rpcCode(t, err, connect.CodeNotFound) // lifecycle permission is not checkout routing
	f.expect(t, "POST", f.installPath(f.tenant), action("install", v.AppId, []string{"shipping.read"}), key(), 403)
	// An existing checkout route can coexist with the new provider app grants.
	// Neither installing nor uninstalling a provider app selects/replaces it.
	f.expect(t, "POST", f.installPath(f.tenant), action("install", "parcel", shippingScopes), key(), 200)
	f.expect(t, "POST", f.installPath(f.tenant), action("activate", "parcel", nil), key(), 200)
	checkout := func() string {
		t.Helper()
		r, err := c.Shipping.GetRates(ctx, connect.NewRequest(&ship.GetRatesRequest{MerchantId: f.tenant, WeightGrams: 1000, DestinationZone: "ID-JKT", IdempotencyKey: key()}))
		if err != nil || !r.Msg.Simulation || r.Msg.InstallationId == id || r.Msg.InstallationId == id2 {
			t.Fatal("provider lifecycle changed the existing checkout route", err)
		}
		return r.Msg.InstallationId
	}
	previousCheckout := checkout()
	outage.Store(true)
	before = reads.Load()
	uninstallKey := key()
	var wg sync.WaitGroup
	for range 3 {
		wg.Go(func() {
			r := target(id)
			r.IdempotencyKey = uninstallKey
			result, err := c.Installations.Uninstall(ctx, connect.NewRequest(&v1.UninstallRequest{Target: r}))
			if err != nil || result.Msg.Result.Installation.GrantState != "revoked" {
				t.Error("uninstall retry", err)
			}
		})
	}
	wg.Wait()
	if reads.Load() != before || unexpected.Load() != 0 {
		t.Fatal("uninstall mutated provider/credential or depended on engine availability")
	}
	if get(id).Status != "uninstalled" || get(id2).Status != "active" {
		t.Fatal("uninstall crossed provider boundary")
	}
	if checkout() != previousCheckout {
		t.Fatal("uninstall changed checkout selection")
	}
	if items := list(f.tenant); len(items) != 1 || items[0].InstallationId != id2 {
		t.Fatal("installed list did not reflect uninstall")
	}
	_, err = c.Installations.Activate(ctx, connect.NewRequest(&v1.ActivateRequest{Target: target(id)}))
	rpcCode(t, err, connect.CodeAlreadyExists)
	outage.Store(false)
	newIntent := prepare("rajaongkir")
	consent(newIntent)
	reinstall, err := c.Installations.Consume(ctx, consumeRequest(newIntent))
	if err != nil || reinstall.Msg.Result.Installation.InstallationId == id {
		t.Fatal("reinstall reused old identity", err)
	}
	old, err := c.Installations.Consume(ctx, rq)
	if err != nil || old.Msg.Result.Installation.Status != "uninstalled" || old.Msg.Result.Installation.InstallationId != id {
		t.Fatal("old receipt revived replacement", err)
	}
	var tokens, revoked int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM platform_installation.app_tokens WHERE tenant_id=$1`, f.tenant).Scan(&tokens); err != nil || tokens != 0 {
		t.Fatal("unexpected app token", err)
	}
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM platform_installation.access_grants WHERE tenant_id=$1 AND installation_id=$2 AND state='revoked'`, f.tenant, id).Scan(&revoked); err != nil || revoked != 1 {
		t.Fatal("grant not revoked", err)
	}
	for _, action := range []string{"consumed", "activated", "uninstalled"} {
		var count int
		if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM platform_installation.access_audit WHERE tenant_id=$1 AND installation_id=$2 AND action=$3`, f.tenant, id, action).Scan(&count); err != nil || count != 1 {
			t.Fatal("missing/duplicate lifecycle audit", action, count, err)
		}
	}
	// Built-in installed=true is upstream readiness, not an irrevocable app grant.
	// Revoking its test app must not disable the engine or an existing checkout.
	outage.Store(true)
	before = reads.Load()
	_, err = c.Installations.Uninstall(ctx, connect.NewRequest(&v1.UninstallRequest{Target: target(id2)}))
	if err != nil || get(id2).GrantState != "revoked" || get(id2).Status != "uninstalled" {
		t.Fatal("second provider grant not revoked", err)
	}
	if reads.Load() != before || unexpected.Load() != 0 || checkout() != previousCheckout {
		t.Fatal("second provider uninstall changed upstream or legacy checkout")
	}
}
