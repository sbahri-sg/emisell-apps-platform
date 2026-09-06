package bootstrap_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"emisell.app/platform/internal/bootstrap"
	"emisell.app/platform/internal/identity"
	identityrepo "emisell.app/platform/internal/identity/postgres"
	"emisell.app/platform/pkg/sdk"
	intent "emisell.app/platform/pkg/sdk/gen/emisell/installation/v1"
	integration "emisell.app/platform/pkg/sdk/gen/emisell/integration/v1"
	pay "emisell.app/platform/pkg/sdk/gen/emisell/payment/v1"
	ship "emisell.app/platform/pkg/sdk/gen/emisell/shipping/v1"
)

// Uses only setup's guarded, isolated test DB; never generates a live credential.
func TestPlatformKeyFullAccessLifecycleAndTenantIsolation(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := portalAccount(t, f, "admin", "administrator")
	operator := portalAccount(t, f, "admin", "operator")
	path := "/api/v1/admin/platform-keys"
	in := map[string]any{"name": "Emisell backend"}
	for _, method := range []string{"GET", "POST"} {
		pexpect(t, operator, method, path, in, key(), 403)
		pexpect(t, f, method, path, in, key(), 401)
	}
	status, _, _, err := admin.call("POST", path, in, key(), developerOrigin)
	if err != nil || status != 403 {
		t.Fatal("cross-origin platform key management")
	}
	pexpect(t, admin, "POST", path, in, "", 400)
	for _, field := range []string{"tenantId", "validDays", "scopes", "access"} {
		pexpect(t, admin, "POST", path, map[string]any{"name": "Core", field: "untrusted"}, key(), 400)
	}
	request := key()
	created := pexpect(t, admin, "POST", path, in, request, 200)
	secret := created["secret"].(string)
	metadata := created["key"].(map[string]any)
	id := metadata["id"].(string)
	if len(secret) != 47 || !strings.HasPrefix(secret, "epk_") || metadata["access"] != "platform_full" || created["secretAvailable"] != true {
		t.Fatal("wrong platform credential contract")
	}
	for _, field := range []string{"tenantId", "expiresAt", "scopes"} {
		if _, ok := metadata[field]; ok {
			t.Fatal("platform key contains legacy binding")
		}
	}
	replay := pexpect(t, admin, "POST", path, in, request, 200)
	if replay["secret"] != "" || replay["secretAvailable"] != false || replay["key"].(map[string]any)["id"] != id {
		t.Fatal("secret replay or duplicate")
	}
	pexpect(t, admin, "POST", path, map[string]any{"name": "changed"}, request, 409)
	listing := pexpect(t, admin, "GET", path, nil, "", 200)
	raw, _ := json.Marshal(listing)
	if strings.Contains(string(raw), secret) || strings.Contains(string(raw), "secret") || strings.Contains(string(raw), "token_hash") {
		t.Fatal("list secret leak")
	}
	var stored string
	if err = f.pool.QueryRow(ctx, `SELECT token_hash FROM platform_identity.core_platform_keys WHERE id=$1`, id).Scan(&stored); err != nil || len(stored) != 64 || stored == secret {
		t.Fatal("credential not hash-only")
	}
	// Fresh HTTP/RPC handlers must read persisted state, not process memory.
	fresh := httptest.NewServer(bootstrap.Handler(f.pool, f.caps, origin, slog.New(slog.NewTextHandler(io.Discard, nil))))
	defer fresh.Close()
	admin.server = fresh
	pexpect(t, admin, "GET", path, nil, "", 200)
	rpc := httptest.NewServer(bootstrap.InternalHandler(f.pool, f.caps, slog.New(slog.NewTextHandler(io.Discard, nil))))
	defer rpc.Close()
	c, err := sdk.NewLocalClient(rpc.URL, secret)
	if err != nil {
		t.Fatal(err)
	}
	check, err := c.Connection.Check(ctx, connect.NewRequest(&integration.CheckRequest{}))
	if err != nil || !check.Msg.PlatformFullAccess || check.Msg.ServiceId != id || check.Msg.TenantId != "" || len(check.Msg.Scopes) != 0 || check.Msg.ExpiresAt != nil {
		t.Fatal("platform check failed", err)
	}
	for _, header := range []string{"Cookie", "Origin"} {
		r := connect.NewRequest(&integration.CheckRequest{})
		r.Header().Set(header, "browser")
		if _, e := c.Connection.Check(ctx, r); e == nil {
			t.Fatal("browser credential accepted")
		}
	}
	bad, err := sdk.NewLocalClient(rpc.URL, "epk_"+strings.Repeat("x", 43))
	if err != nil {
		t.Fatal(err)
	}
	_, err = bad.Connection.Check(ctx, connect.NewRequest(&integration.CheckRequest{}))
	rpcCode(t, err, connect.CodeUnauthenticated)
	for _, tenant := range []string{"", "not_registered", "bad/tenant"} {
		_, err = c.Payment.Create(ctx, connect.NewRequest(&pay.CreateRequest{TenantId: tenant, IdempotencyKey: key(), Reference: "blocked", AmountMinor: 10, Currency: "IDR"}))
		want := connect.CodeInvalidArgument
		if tenant == "not_registered" {
			want = connect.CodeNotFound
		}
		rpcCode(t, err, want)
	}
	// Full service auth cannot replace an active installation/grant.
	_, err = c.Payment.Create(ctx, connect.NewRequest(&pay.CreateRequest{TenantId: f.tenant, IdempotencyKey: key(), Reference: "no-install", AmountMinor: 10, Currency: "IDR"}))
	rpcCode(t, err, connect.CodeNotFound)
	var firstPayment, firstShipment string
	for _, tenant := range []string{f.tenant, f.other} {
		// Consent is per tenant and actor, and does not install/activate anything.
		prepare := connect.NewRequest(&intent.PrepareRequest{TenantId: tenant, CoreActorId: "core-staff", IdempotencyKey: key(), AppId: "emisell-pay", Version: "1.0.0"})
		v, e := c.InstallIntents.Prepare(ctx, prepare)
		if e != nil || v.Msg.Intent.ExecutionAllowed || v.Msg.Intent.TenantId != tenant {
			t.Fatal("intent tenant context", e)
		}
		decision := intentDecision(v.Msg.Intent, intent.ConsentDecision_CONSENT_DECISION_CONSENT)
		decision.Msg.TenantId = tenant
		consented, e := c.InstallIntents.Decide(ctx, decision)
		if e != nil || consented.Msg.Intent.ExecutionAllowed {
			t.Fatal("consent bypassed grant", e)
		}
		for _, requested := range []string{"", "not_registered"} {
			_, e = c.InstallIntents.Get(ctx, connect.NewRequest(&intent.GetRequest{TenantId: requested, CoreActorId: "core-staff", IntentId: v.Msg.Intent.Id}))
			if e == nil {
				t.Fatal("intent lacks valid tenant")
			}
		}
		otherTenant := f.other
		if tenant == f.other {
			otherTenant = f.tenant
		}
		_, e = c.InstallIntents.Get(ctx, connect.NewRequest(&intent.GetRequest{TenantId: otherTenant, CoreActorId: "core-staff", IntentId: v.Msg.Intent.Id}))
		rpcCode(t, e, connect.CodeNotFound)
		_, e = c.InstallIntents.Get(ctx, connect.NewRequest(&intent.GetRequest{TenantId: tenant, CoreActorId: "foreign-actor", IntentId: v.Msg.Intent.Id}))
		rpcCode(t, e, connect.CodeNotFound)
		for _, app := range []struct {
			id     string
			scopes []string
		}{{"emisell-pay", payScopes}, {"parcel", shippingScopes}} {
			for _, kind := range []string{"install", "activate"} {
				f.expect(t, "POST", f.installPath(tenant), action(kind, app.id, app.scopes), key(), 200)
			}
		}
		p, e := c.Payment.Create(ctx, connect.NewRequest(&pay.CreateRequest{TenantId: tenant, IdempotencyKey: key(), Reference: "same-reference", AmountMinor: 1000, Currency: "IDR"}))
		if e != nil {
			t.Fatal(e)
		}
		if _, e = c.Payment.Status(ctx, connect.NewRequest(&pay.StatusRequest{TenantId: tenant, IdempotencyKey: key(), ResourceId: p.Msg.Payment.Id})); e != nil {
			t.Fatal(e)
		}
		s, e := c.Shipping.Create(ctx, connect.NewRequest(&ship.CreateRequest{TenantId: tenant, IdempotencyKey: key(), Reference: "same-reference", WeightGrams: 1000, DestinationZone: "ID-JKT"}))
		if e != nil {
			t.Fatal(e)
		}
		if _, e = c.Shipping.Track(ctx, connect.NewRequest(&ship.TrackRequest{TenantId: tenant, IdempotencyKey: key(), ResourceId: s.Msg.Shipment.Id})); e != nil {
			t.Fatal(e)
		}
		if tenant == f.tenant {
			firstPayment = p.Msg.Payment.Id
			firstShipment = s.Msg.Shipment.Id
		}
	}
	_, err = c.Payment.Status(ctx, connect.NewRequest(&pay.StatusRequest{TenantId: f.other, IdempotencyKey: key(), ResourceId: firstPayment}))
	rpcCode(t, err, connect.CodeNotFound)
	_, err = c.Shipping.Track(ctx, connect.NewRequest(&ship.TrackRequest{TenantId: f.other, IdempotencyKey: key(), ResourceId: firstShipment}))
	rpcCode(t, err, connect.CodeNotFound)
	legacy, _, _ := serviceClient(t, f, f.tenant, intentScopes())
	_, err = legacy.InstallIntents.Prepare(ctx, connect.NewRequest(&intent.PrepareRequest{TenantId: f.other, CoreActorId: "core-staff", IdempotencyKey: key(), AppId: "parcel", Version: "1.0.0"}))
	rpcCode(t, err, connect.CodeNotFound)
	lc, err := legacy.Connection.Check(ctx, connect.NewRequest(&integration.CheckRequest{}))
	if err != nil || lc.Msg.PlatformFullAccess || lc.Msg.ExpiresAt == nil || lc.Msg.TenantId != f.tenant {
		t.Fatal("legacy silently elevated")
	}
	pexpect(t, operator, "POST", path+"/"+id+"/revoke", map[string]any{}, "", 403)
	for range 2 {
		pexpect(t, admin, "POST", path+"/"+id+"/revoke", map[string]any{}, "", 200)
	}
	_, err = c.Connection.Check(ctx, connect.NewRequest(&integration.CheckRequest{}))
	rpcCode(t, err, connect.CodeUnauthenticated)
	_, err = c.Payment.Status(ctx, connect.NewRequest(&pay.StatusRequest{TenantId: f.tenant, IdempotencyKey: key(), ResourceId: firstPayment}))
	rpcCode(t, err, connect.CodeUnauthenticated)
	replay = pexpect(t, admin, "POST", path, in, request, 200)
	if replay["secret"] != "" || replay["key"].(map[string]any)["status"] != "revoked" {
		t.Fatal("replay revived credential")
	}
	var issued, revoked int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FILTER(WHERE action='issued'),count(*) FILTER(WHERE action='revoked') FROM platform_identity.core_platform_key_audit WHERE key_id=$1`, id).Scan(&issued, &revoked); err != nil || issued != 1 || revoked != 1 {
		t.Fatal("non-atomic audit", err)
	}
}

func TestPlatformKeyConcurrentSecretOnce(t *testing.T) {
	f := setup(t)
	admin := portalAccount(t, f, "admin", "administrator")
	s := identity.PlatformKeys{Repo: identityrepo.Repository{Pool: f.pool}}
	p := identity.PortalPrincipal{ID: admin.user, Surface: "admin", Role: "administrator"}
	k := key()
	type result struct {
		id, secret string
		err        error
	}
	results := make(chan result, 4)
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			v, secret, err := s.Create(context.Background(), p, k, identity.CreatePlatformKey{Name: "Concurrent"})
			results <- result{v.ID, secret, err}
		})
	}
	wg.Wait()
	close(results)
	id, secrets := "", 0
	for r := range results {
		if r.err != nil {
			t.Fatal(r.err)
		}
		if id != "" && id != r.id {
			t.Fatal("concurrent duplicate")
		}
		id = r.id
		if r.secret != "" {
			secrets++
		}
	}
	if secrets != 1 {
		t.Fatal("secret must be returned exactly once")
	}
}
