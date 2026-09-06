package bootstrap_test

import (
	"connectrpc.com/connect"
	"context"
	"emisell.app/platform/internal/bootstrap"
	"emisell.app/platform/internal/identity"
	identityrepo "emisell.app/platform/internal/identity/postgres"
	"emisell.app/platform/internal/platform/fault"
	integration "emisell.app/platform/pkg/sdk/gen/emisell/integration/v1"
	integrationconnect "emisell.app/platform/pkg/sdk/gen/emisell/integration/v1/integrationv1connect"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestManagedCoreKeysLifecycleAndRPC(t *testing.T) {
	f := setup(t)
	admin := portalAccount(t, f, "admin", "administrator")
	operator := portalAccount(t, f, "admin", "operator")
	in := identity.CreateManagedKey{Name: "Core test", TenantID: f.tenant, Scopes: []string{"payments.read"}, ValidDays: 7}
	path := "/api/v1/admin/api-keys"
	pexpect(t, operator, "GET", path, nil, "", 403)
	pexpect(t, operator, "POST", path, in, key(), 403)
	pexpect(t, f, "GET", path, nil, "", 401)
	status, _, _, err := admin.call("GET", path, nil, "", developerOrigin)
	if err != nil || status != 403 {
		t.Fatal("cross origin key access")
	}
	pexpect(t, admin, "POST", path, in, "", 400)
	bad := in
	bad.Scopes = []string{"read_products"}
	pexpect(t, admin, "POST", path, bad, key(), 400)
	bad = in
	bad.TenantID = "unknown_tenant"
	pexpect(t, admin, "POST", path, bad, key(), 404)
	request := key()
	created := pexpect(t, admin, "POST", path, in, request, 200)
	secret := created["secret"].(string)
	keyID := created["key"].(map[string]any)["id"].(string)
	if len(secret) != 43 || created["secretAvailable"] != true {
		t.Fatal("secret missing")
	}
	replay := pexpect(t, admin, "POST", path, in, request, 200)
	if replay["secret"] != "" || replay["secretAvailable"] != false || replay["key"].(map[string]any)["id"] != keyID {
		t.Fatal("replay disclosed secret or created duplicate")
	}
	bad = in
	bad.Name = "changed"
	pexpect(t, admin, "POST", path, bad, request, 409)
	listing := pexpect(t, admin, "GET", path, nil, "", 200)
	raw, _ := json.Marshal(listing)
	if strings.Contains(string(raw), secret) || strings.Contains(string(raw), "token_hash") || strings.Contains(string(raw), "secret") {
		t.Fatal("key list leaked secret")
	}
	ctx := context.Background()
	auth := identity.ServiceAccounts{Repo: identityrepo.Repository{Pool: f.pool}}
	principal, err := auth.Authenticate(ctx, secret)
	if err != nil || principal.ID != keyID || principal.TenantID != f.tenant {
		t.Fatal("key not accepted")
	}
	if err = auth.Authorize(ctx, keyID, f.other); err != fault.NotFound {
		t.Fatal("cross tenant accepted")
	}
	rpc := httptest.NewServer(bootstrap.InternalHandler(f.pool, f.caps, slog.New(slog.NewJSONHandler(io.Discard, nil))))
	defer rpc.Close()
	client := integrationconnect.NewConnectionServiceClient(rpc.Client(), rpc.URL)
	check := func(token string) *connect.Request[integration.CheckRequest] {
		r := connect.NewRequest(&integration.CheckRequest{})
		r.Header().Set("Authorization", "Bearer "+token)
		return r
	}
	out, err := client.Check(ctx, check(secret))
	if err != nil || out.Msg.ServiceId != keyID || out.Msg.TenantId != f.tenant || len(out.Msg.Scopes) != 1 || out.Msg.Scopes[0] != "payments.read" {
		t.Fatal("connection contract failed")
	}
	if _, err = client.Check(ctx, connect.NewRequest(&integration.CheckRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatal("anonymous key check")
	}
	if _, err = client.Check(ctx, check(strings.Repeat("x", 43))); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatal("unknown key accepted")
	}
	for _, h := range []string{"Cookie", "Origin"} {
		req := check(secret)
		req.Header().Set(h, "untrusted")
		if _, err = client.Check(ctx, req); err == nil {
			t.Fatal("browser credential accepted")
		}
	}
	pexpect(t, operator, "POST", path+"/"+keyID+"/revoke", map[string]any{}, "", 403)
	pexpect(t, admin, "POST", path+"/"+keyID+"/revoke", map[string]any{}, "", 200)
	pexpect(t, admin, "POST", path+"/"+keyID+"/revoke", map[string]any{}, "", 200)
	if _, err = client.Check(ctx, check(secret)); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatal("revoked key accepted")
	}
	replay = pexpect(t, admin, "POST", path, in, request, 200)
	if replay["key"].(map[string]any)["status"] != "revoked" || replay["secret"] != "" {
		t.Fatal("replay revived key")
	}
	var issued, revoked int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FILTER(WHERE action='issued'),count(*) FILTER(WHERE action='revoked') FROM platform_identity.service_account_audit WHERE service_id=$1`, keyID).Scan(&issued, &revoked); err != nil || issued != 1 || revoked != 1 {
		t.Fatal("audit duplication", err)
	}
	// State persists in a fresh handler; expiry is enforced by the same RPC auth.
	fresh := httptest.NewServer(bootstrap.Handler(f.pool, f.caps, origin, slog.New(slog.NewJSONHandler(io.Discard, nil))))
	defer fresh.Close()
	admin.server = fresh
	exp := pexpect(t, admin, "POST", path, in, key(), 200)
	expID := exp["key"].(map[string]any)["id"].(string)
	if _, err = f.pool.Exec(ctx, `UPDATE platform_identity.service_accounts SET expires_at=now()-interval '1 second' WHERE id=$1`, expID); err != nil {
		t.Fatal(err)
	}
	if _, err = client.Check(ctx, check(exp["secret"].(string))); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatal("expired key accepted")
	}
}

func TestManagedKeyConcurrentGenerationReturnsSecretOnce(t *testing.T) {
	f := setup(t)
	admin := portalAccount(t, f, "admin", "administrator")
	s := identity.ManagedKeys{Repo: identityrepo.Repository{Pool: f.pool}}
	p := identity.PortalPrincipal{ID: admin.user, Surface: "admin", Role: "administrator"}
	in := identity.CreateManagedKey{Name: "Concurrent", TenantID: f.tenant, Scopes: []string{"shipping.read"}, ValidDays: 1}
	k := key()
	var wg sync.WaitGroup
	type result struct {
		id, secret string
		err        error
	}
	results := make(chan result, 4)
	for range 4 {
		wg.Go(func() {
			v, secret, err := s.Create(context.Background(), p, k, in)
			results <- result{v.ID, secret, err}
		})
	}
	wg.Wait()
	close(results)
	ids := map[string]bool{}
	secrets := 0
	for r := range results {
		if r.err != nil {
			t.Fatal(r.err)
		}
		ids[r.id] = true
		if r.secret != "" {
			secrets++
		}
	}
	if len(ids) != 1 || secrets != 1 {
		t.Fatal("duplicate credential creation")
	}
}
