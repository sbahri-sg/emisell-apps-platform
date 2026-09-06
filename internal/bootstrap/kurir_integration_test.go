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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"emisell.app/platform/internal/bootstrap"
	"emisell.app/platform/internal/runtime/kurir"
	"emisell.app/platform/pkg/appmanifest"
	"emisell.app/platform/pkg/sdk"
	v1 "emisell.app/platform/pkg/sdk/gen/emisell/installation/v1"
	ship "emisell.app/platform/pkg/sdk/gen/emisell/shipping/v1"
)

// No production key, real API-Kurir, provider credential or merchant is used.
func TestKurirReferenceConsentRatesAndRevocation(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	m := appmanifest.KurirFixture()
	raw, _ := json.Marshal(m)
	fixtureKey := appmanifest.LocalFixtureKey()
	_, err := f.pool.Exec(ctx, `INSERT INTO platform_app.releases(app_id,version,manifest,signature,public_key)
 VALUES($1,$2,$3::json,$4,$5) ON CONFLICT DO NOTHING`, m.ID, m.Version, string(raw), base64.StdEncoding.EncodeToString(ed25519.Sign(fixtureKey, raw)), base64.StdEncoding.EncodeToString(fixtureKey.Public().(ed25519.PublicKey)))
	if err != nil {
		t.Fatal(err)
	}
	_, _, coreKey, _ := lifecycleClient(t, f)
	var configured, outage atomic.Bool
	var reads, rates, unexpected atomic.Int64
	var blockMu sync.Mutex
	var entered, release chan struct{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("key") != kurir.FixtureKey || r.Header.Get("Authorization") != "" || r.Header.Get("X-Emisell-Merchant-ID") != f.tenant || r.Header.Get(kurir.FixtureHeader) != appmanifest.KurirFixtureProfile {
			t.Error("gateway identity leaked or changed")
			w.WriteHeader(403)
			return
		}
		w.Header().Set(kurir.FixtureHeader, appmanifest.KurirFixtureProfile)
		if outage.Load() {
			w.WriteHeader(503)
			return
		}
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/integrations/providers":
			reads.Add(1)
			if !configured.Load() {
				_, _ = io.WriteString(w, `{"data":{"version":0,"active_provider_code":null,"providers":[]}}`)
				return
			}
			_, _ = io.WriteString(w, `{"data":{"version":7,"active_provider_code":"rajaongkir","providers":[{"code":"rajaongkir","available":true,"installed":true,"active":true}]}}`)
		case r.Method == "POST" && r.URL.Path == "/api/v1/calculate/district/domestic-cost":
			rates.Add(1)
			if r.ParseForm() != nil || len(r.PostForm) != 4 || r.PostForm.Get("origin") != "442" || r.PostForm.Get("destination") != "1354" || r.PostForm.Get("include_group") != "true" {
				t.Error("wrong gateway translation")
			}
			blockMu.Lock()
			started, resume := entered, release
			blockMu.Unlock()
			if started != nil {
				close(started)
				select {
				case <-resume:
				case <-r.Context().Done():
					return
				}
			}
			_, _ = io.WriteString(w, `{"meta":{"code":200,"status":"success"},"data":[{"code":"jne","service":"native-hidden","canonical_service":"REG","cost":12000}]}`)
		default:
			unexpected.Add(1)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(upstream.Close)
	bridge, err := kurir.NewLocalFixture(upstream.URL, map[string]int{"ID-ORIGIN": 442, "ID-DESTINATION": 1354})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(bridge.Close)
	newClient := func(withBridge bool) *sdk.Client {
		var h http.Handler
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		if withBridge {
			h = bootstrap.InternalHandlerWithLocalShipping(f.pool, f.caps, logger, bridge)
		} else {
			h = bootstrap.InternalHandler(f.pool, f.caps, logger)
		}
		s := httptest.NewServer(h)
		t.Cleanup(s.Close)
		c, e := sdk.NewLocalClient(s.URL, coreKey)
		if e != nil {
			t.Fatal(e)
		}
		return c
	}
	c := newClient(true)
	prepare := func() *v1.InstallIntent {
		r, e := c.InstallIntents.Prepare(ctx, connect.NewRequest(&v1.PrepareRequest{MerchantId: f.tenant, CoreActorId: "staff", IdempotencyKey: key(), AppId: m.ID, Version: m.Version}))
		if e != nil {
			t.Fatal(e)
		}
		return r.Msg.Intent
	}
	consent := func(v *v1.InstallIntent) *connect.Request[v1.ConsumeRequest] {
		d := intentDecision(v, v1.ConsentDecision_CONSENT_DECISION_CONSENT)
		d.Msg.MerchantId = f.tenant
		if _, e := c.InstallIntents.Decide(ctx, d); e != nil {
			t.Fatal(e)
		}
		return connect.NewRequest(&v1.ConsumeRequest{MerchantId: f.tenant, CoreActorId: "staff", IdempotencyKey: key(), IntentId: v.Id, ConsentDigest: v.ConsentDigest})
	}
	target := func(id string) *v1.InstallationRequest {
		return &v1.InstallationRequest{MerchantId: f.tenant, CoreActorId: "staff", InstallationId: id, IdempotencyKey: key()}
	}
	getRates := func(merchant, k string) (*connect.Response[ship.GetRatesResponse], error) {
		return c.Shipping.GetRates(ctx, connect.NewRequest(&ship.GetRatesRequest{MerchantId: merchant, IdempotencyKey: k, WeightGrams: 1200, OriginZone: "ID-ORIGIN", DestinationZone: "ID-DESTINATION"}))
	}
	list := func(merchant, status string, count int) {
		t.Helper()
		r, e := c.Installations.ListInstallations(ctx, connect.NewRequest(&v1.ListInstallationsRequest{MerchantId: merchant, CoreActorId: "staff"}))
		if e != nil || len(r.Msg.Installations) != count {
			t.Fatal("installed list", e, r)
		}
		if count == 1 && (r.Msg.Installations[0].AppId != m.ID || r.Msg.Installations[0].Status != status || r.Msg.Installations[0].ExecutionProfile != m.ExecutionProfile) {
			t.Fatal("wrong installed app metadata")
		}
	}
	_, err = getRates(f.tenant, key())
	rpcCode(t, err, connect.CodeNotFound)
	v := prepare()
	_, err = c.Installations.Consume(ctx, connect.NewRequest(&v1.ConsumeRequest{MerchantId: f.tenant, CoreActorId: "staff", IdempotencyKey: key(), IntentId: v.Id, ConsentDigest: v.ConsentDigest}))
	rpcCode(t, err, connect.CodeAlreadyExists)
	list(f.tenant, "", 0)
	if reads.Load() != 0 || rates.Load() != 0 {
		t.Fatal("upstream called before consent")
	}
	request := consent(v)
	installed, err := c.Installations.Consume(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	id := installed.Msg.Result.Installation.InstallationId
	if installed.Msg.Result.Installation.GrantState != "pending" || len(installed.Msg.Result.Installation.GrantedScopes) != 0 {
		t.Fatal("premature grant")
	}
	retry, err := c.Installations.Consume(ctx, request)
	if err != nil || !retry.Msg.Result.Replayed || retry.Msg.Result.Installation.InstallationId != id {
		t.Fatal("duplicate install", err)
	}
	list(f.tenant, "pending", 1)
	list(f.other, "", 0)
	f.expect(t, "POST", f.installPath(f.tenant), action("install", m.ID, m.Scopes), key(), 403)
	activate := connect.NewRequest(&v1.ActivateRequest{Target: target(id)})
	_, err = c.Installations.Activate(ctx, activate)
	rpcCode(t, err, connect.CodeAlreadyExists)
	list(f.tenant, "pending", 1)
	configured.Store(true)
	// A normal server has no implicit simulator fallback for this profile.
	_, err = newClient(false).Installations.Activate(ctx, activate)
	rpcCode(t, err, connect.CodeUnavailable)
	active, err := c.Installations.Activate(ctx, activate)
	if err != nil || !slices.Equal(active.Msg.Result.Installation.GrantedScopes, []string{"shipping.read"}) {
		t.Fatal("activation", err)
	}
	c = newClient(true) // durable binding survives a fresh HTTP composition
	list(f.tenant, "active", 1)
	_, err = newClient(false).Shipping.GetRates(ctx, connect.NewRequest(&ship.GetRatesRequest{MerchantId: f.tenant, IdempotencyKey: key(), WeightGrams: 1200, OriginZone: "ID-ORIGIN", DestinationZone: "ID-DESTINATION"}))
	rpcCode(t, err, connect.CodeUnavailable)
	token, err := c.Installations.IssueToken(ctx, connect.NewRequest(&v1.IssueTokenRequest{Target: target(id)}))
	if err != nil {
		t.Fatal(err)
	}
	if appCheck(t, f, token.Msg.Result.AppToken, f.tenant, m.ID, id, nil) != 200 {
		t.Fatal("reference token binding")
	}
	k := key()
	result, err := getRates(f.tenant, k)
	if err != nil || !result.Msg.Simulation || result.Msg.InstallationId != id || len(result.Msg.Rates) != 1 || result.Msg.Rates[0].Service != "jne:REG" || result.Msg.Rates[0].AmountMinor != 12000 {
		t.Fatal("rates through Core RPC", err, result)
	}
	_, err = getRates(f.tenant, k)
	if err != nil || rates.Load() != 1 {
		t.Fatal("rates replay repeated upstream", err)
	}
	_, err = c.Shipping.GetRates(ctx, connect.NewRequest(&ship.GetRatesRequest{MerchantId: f.tenant, IdempotencyKey: k, WeightGrams: 1500, OriginZone: "ID-ORIGIN", DestinationZone: "ID-DESTINATION"}))
	rpcCode(t, err, connect.CodeAlreadyExists)
	_, err = getRates(f.other, key())
	rpcCode(t, err, connect.CodeNotFound)
	_, err = c.Installations.GetInstallation(ctx, connect.NewRequest(&v1.GetInstallationRequest{Target: &v1.InstallationRequest{MerchantId: f.other, CoreActorId: "staff", InstallationId: id}}))
	rpcCode(t, err, connect.CodeNotFound)
	_, err = c.Shipping.Create(ctx, connect.NewRequest(&ship.CreateRequest{MerchantId: f.tenant, IdempotencyKey: key(), Reference: "order-1", WeightGrams: 1200, DestinationZone: "ID-DESTINATION"}))
	rpcCode(t, err, connect.CodePermissionDenied)
	_, err = c.Shipping.Track(ctx, connect.NewRequest(&ship.TrackRequest{MerchantId: f.tenant, IdempotencyKey: key(), ResourceId: "shipment-1"}))
	rpcCode(t, err, connect.CodePermissionDenied)
	configured.Store(false)
	_, err = getRates(f.tenant, k)
	rpcCode(t, err, connect.CodeAlreadyExists) // replay still checks readiness
	configured.Store(true)
	outage.Store(true)
	_, err = getRates(f.tenant, key())
	rpcCode(t, err, connect.CodeUnavailable)
	outage.Store(false)
	if rates.Load() != 1 {
		t.Fatal("denied calls reached rate endpoint")
	}
	// A corrupted/revoked grant cannot access an already-cached response.
	_, err = f.pool.Exec(ctx, `UPDATE platform_installation.access_grants SET state='revoked',revoked_at=clock_timestamp() WHERE tenant_id=$1 AND installation_id=$2`, f.tenant, id)
	if err != nil {
		t.Fatal(err)
	}
	before := reads.Load()
	_, err = getRates(f.tenant, k)
	rpcCode(t, err, connect.CodePermissionDenied)
	if reads.Load() != before {
		t.Fatal("revoked grant reached gateway")
	}
	_, err = f.pool.Exec(ctx, `UPDATE platform_installation.access_grants SET state='active',revoked_at=NULL WHERE tenant_id=$1 AND installation_id=$2`, f.tenant, id)
	if err != nil {
		t.Fatal(err)
	}
	// Uninstall waits for the in-flight invocation gate; no later call slips past.
	blockMu.Lock()
	entered = make(chan struct{})
	release = make(chan struct{})
	started, resume := entered, release
	blockMu.Unlock()
	defer func() {
		select {
		case <-resume:
		default:
			close(resume)
		}
	}()
	invokeDone := make(chan error, 1)
	go func() { _, e := getRates(f.tenant, key()); invokeDone <- e }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("invocation did not start")
	}
	uninstallReq := connect.NewRequest(&v1.UninstallRequest{Target: target(id)})
	uninstallDone := make(chan error, 1)
	go func() { _, e := c.Installations.Uninstall(ctx, uninstallReq); uninstallDone <- e }()
	select {
	case e := <-uninstallDone:
		t.Fatal("uninstall passed in-flight gate", e)
	case <-time.After(50 * time.Millisecond):
	}
	close(resume)
	if e := <-invokeDone; e != nil {
		t.Fatal(e)
	}
	if e := <-uninstallDone; e != nil {
		t.Fatal(e)
	}
	blockMu.Lock()
	entered = nil
	release = nil
	blockMu.Unlock()
	list(f.tenant, "", 0)
	if appCheck(t, f, token.Msg.Result.AppToken, f.tenant, m.ID, id, nil) != 401 {
		t.Fatal("token survived uninstall")
	}
	before = reads.Load()
	_, err = getRates(f.tenant, k)
	rpcCode(t, err, connect.CodeNotFound)
	if reads.Load() != before {
		t.Fatal("uninstalled app reached gateway")
	}
	u, err := c.Installations.Uninstall(ctx, uninstallReq)
	if err != nil || !u.Msg.Result.Replayed || u.Msg.Result.Installation.GrantState != "revoked" {
		t.Fatal("uninstall replay", err)
	}
	retry, err = c.Installations.Consume(ctx, request)
	if err != nil || retry.Msg.Result.Installation.Status != "uninstalled" {
		t.Fatal("old consume resurrected installation", err)
	}
	fresh, err := c.Installations.Consume(ctx, consent(prepare()))
	if err != nil || fresh.Msg.Result.Installation.InstallationId == id {
		t.Fatal("reinstall identity", err)
	}
	_, err = c.Installations.Activate(ctx, connect.NewRequest(&v1.ActivateRequest{Target: target(fresh.Msg.Result.Installation.InstallationId)}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = getRates(f.tenant, k)
	rpcCode(t, err, connect.CodeAlreadyExists) // old cache is installation-bound
	_, err = c.Installations.Uninstall(ctx, uninstallReq)
	if err != nil {
		t.Fatal(err)
	}
	list(f.tenant, "active", 1) // old uninstall cannot revoke replacement
	_, err = getRates(f.tenant, key())
	if err != nil {
		t.Fatal(err)
	}
	var audit, events int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM platform_installation.access_audit WHERE tenant_id=$1 AND installation_id=$2`, f.tenant, id).Scan(&audit); err != nil {
		t.Fatal(err)
	}
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM platform_capability.events WHERE tenant_id=$1`, f.tenant).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if audit != 4 || events != 3 {
		t.Fatal("missing or duplicated audit", audit, events)
	}
	if unexpected.Load() != 0 || !configured.Load() {
		t.Fatal("lifecycle mutated API-Kurir provider configuration")
	}
}
