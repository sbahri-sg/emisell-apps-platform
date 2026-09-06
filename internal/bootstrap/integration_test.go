package bootstrap_test

import (
	"bytes"
	"context"
	apprepo "emisell.app/platform/internal/app/repository"
	"emisell.app/platform/internal/bootstrap"
	"emisell.app/platform/internal/identity"
	identityrepo "emisell.app/platform/internal/identity/postgres"
	"emisell.app/platform/internal/platform/ids"
	"emisell.app/platform/migrations"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"
)

const origin = "http://localhost:4317"

var payScopes = []string{"orders.read", "payments.read", "payments.write"}
var shippingScopes = []string{"orders.read", "shipping.read", "shipping.write"}

type fixture struct {
	pool, caps                           *pgxpool.Pool
	server                               *httptest.Server
	user, email, password, tenant, other string
	cookie                               *http.Cookie
}

func setup(t *testing.T) *fixture {
	t.Helper()
	dsn := os.Getenv("EMISELL_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set EMISELL_TEST_DATABASE_URL to the isolated local test database")
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Path != "/emisell_local_test" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") {
		t.Fatal("integration tests require loopback emisell_local_test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	caps, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(caps.Close)
	if err = migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err = migrations.Apply(ctx, pool); err != nil {
		t.Fatal("migration not repeatable", err)
	}
	f := &fixture{pool: pool, caps: caps, user: ids.New("user"), tenant: ids.New("tenant"), other: ids.New("tenant"), password: ids.New("password")}
	f.email = f.user + "@local.invalid"
	if err = (identityrepo.Repository{Pool: pool}).SeedUser(ctx, f.user, f.email, f.password, []identity.Workspace{{ID: f.tenant, Name: "Test Store"}, {ID: f.other, Name: "Other Store"}}); err != nil {
		t.Fatal(err)
	}
	if err = (apprepo.Postgres{Pool: pool}).SeedLocal(ctx); err != nil {
		t.Fatal(err)
	}
	f.server = httptest.NewServer(bootstrap.Handler(pool, caps, origin, slog.New(slog.NewJSONHandler(io.Discard, nil))))
	t.Cleanup(f.server.Close)
	status, _, cookies, err := f.call("POST", "/api/v1/login", map[string]string{"email": f.email, "password": f.password}, "", origin)
	if err != nil || status != 200 || len(cookies) != 1 {
		t.Fatalf("login: status=%d err=%v", status, err)
	}
	f.cookie = cookies[0]
	return f
}
func (f *fixture) call(method, path string, body any, key, requestOrigin string) (int, map[string]any, []*http.Cookie, error) {
	raw, _ := json.Marshal(body)
	req, err := http.NewRequest(method, f.server.URL+path, bytes.NewReader(raw))
	if err != nil {
		return 0, nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", requestOrigin)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	if f.cookie != nil {
		req.AddCookie(f.cookie)
	}
	resp, err := f.server.Client().Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	var data map[string]any
	err = json.NewDecoder(resp.Body).Decode(&data)
	return resp.StatusCode, data, resp.Cookies(), err
}
func (f *fixture) expect(t *testing.T, method, path string, body any, key string, want int) map[string]any {
	t.Helper()
	status, data, _, err := f.call(method, path, body, key, origin)
	if err != nil || status != want {
		t.Fatalf("%s %s: got %d want %d data=%v err=%v", method, path, status, want, data, err)
	}
	return data
}
func (f *fixture) installPath(tenant string) string {
	return "/api/v1/workspaces/" + tenant + "/installations"
}
func (f *fixture) capPath(tenant, cap string) string {
	return "/api/v1/workspaces/" + tenant + "/capabilities/" + cap + "/v1/invoke"
}
func action(kind, app string, scopes []string) map[string]any {
	m := map[string]any{"type": kind, "appId": app}
	if kind == "install" {
		m["version"] = "1.0.0"
		m["grants"] = scopes
	}
	return m
}
func key() string { return ids.New("request") }

func TestPersistentLifecycleIsolationAndCapabilityContracts(t *testing.T) {
	f := setup(t)
	path := f.installPath(f.tenant)
	if !f.cookie.HttpOnly || f.cookie.SameSite != http.SameSiteStrictMode || f.cookie.Path != "/api/v1" {
		t.Fatal("unsafe local cookie")
	}
	f.expect(t, "POST", path, action("install", "emisell-pay", nil), key(), 400)
	f.expect(t, "POST", path, action("activate", "emisell-pay", nil), key(), 404)
	k := key()
	body := action("install", "emisell-pay", payScopes)
	installed := f.expect(t, "POST", path, body, k, 200)
	if !reflect.DeepEqual(installed, f.expect(t, "POST", path, body, k, 200)) {
		t.Fatal("idempotent response changed")
	}
	f.expect(t, "POST", path, action("activate", "emisell-pay", nil), k, 409)
	f.expect(t, "POST", path, action("activate", "emisell-pay", nil), key(), 200)
	f.expect(t, "POST", path, action("install", "emisell-pay-alt", payScopes), key(), 200)
	f.expect(t, "POST", path, action("activate", "emisell-pay-alt", nil), key(), 409)
	events := f.expect(t, "GET", "/api/v1/workspaces/"+f.tenant+"/events", nil, "", 200)["events"].([]any)
	if len(events) != 3 {
		t.Fatalf("duplicate or missing audit: %d", len(events))
	}
	firstEvent := events[0].(map[string]any)
	if firstEvent["tenantId"] != f.tenant || firstEvent["actorId"] != f.user || firstEvent["correlationId"] == "" {
		t.Fatal("incomplete audit envelope")
	}
	second := f.expect(t, "GET", f.installPath(f.other), nil, "", 200)["installations"].([]any)
	if len(second) != 0 {
		t.Fatal("cross-tenant installation leak")
	}
	foreign := ids.New("foreign")
	f.expect(t, "GET", f.installPath(foreign), nil, "", 404)
	f.expect(t, "POST", f.installPath(foreign), body, key(), 404)
	capPath := f.capPath(f.tenant, "payment")
	paymentKey := key()
	create := map[string]any{"operation": "create", "reference": "order-001", "amountMinor": 12000, "currency": "IDR"}
	payment := f.expect(t, "POST", capPath, create, paymentKey, 200)
	if payment["simulation"] != true {
		t.Fatal("missing simulation marker")
	}
	if !reflect.DeepEqual(payment, f.expect(t, "POST", capPath, create, paymentKey, 200)) {
		t.Fatal("payment replay duplicated")
	}
	paymentID := payment["resource"].(map[string]any)["id"].(string)
	for _, operation := range []string{"status", "capture", "refund", "status"} {
		f.expect(t, "POST", capPath, map[string]string{"operation": operation, "resourceId": paymentID}, key(), 200)
	}
	f.expect(t, "POST", capPath, map[string]string{"operation": "capture", "resourceId": paymentID}, key(), 409)
	for _, kind := range []string{"install", "activate"} {
		f.expect(t, "POST", f.installPath(f.other), action(kind, "emisell-pay", payScopes), key(), 200)
	}
	f.expect(t, "POST", f.capPath(f.other, "payment"), map[string]string{"operation": "status", "resourceId": paymentID}, key(), 404)
	for _, kind := range []string{"install", "activate"} {
		f.expect(t, "POST", path, action(kind, "parcel", shippingScopes), key(), 200)
	}
	shippingPath := f.capPath(f.tenant, "shipping")
	f.expect(t, "POST", shippingPath, map[string]any{"operation": "get_rates", "weightGrams": 1200, "destinationZone": "ID-JKT"}, key(), 200)
	shipping := f.expect(t, "POST", shippingPath, map[string]any{"operation": "create", "weightGrams": 1200, "destinationZone": "ID-JKT", "reference": "order-001"}, key(), 200)
	f.expect(t, "POST", shippingPath, map[string]any{"operation": "track", "resourceId": shipping["resource"].(map[string]any)["id"]}, key(), 200)
	// A fresh server instance reads the same PostgreSQL sessions and installations.
	fresh := httptest.NewServer(bootstrap.Handler(f.pool, f.caps, origin, slog.New(slog.NewJSONHandler(io.Discard, nil))))
	defer fresh.Close()
	old := f.server
	f.server = fresh
	f.expect(t, "GET", "/api/v1/dashboard", nil, "", 200)
	persisted := f.expect(t, "GET", path, nil, "", 200)["installations"].([]any)
	if len(persisted) != 3 {
		t.Fatal("state did not survive restart")
	}
	f.server = old
	f.expect(t, "POST", path, action("uninstall", "emisell-pay", nil), key(), 200)
	f.expect(t, "POST", path, action("uninstall", "emisell-pay", nil), key(), 200)
	f.expect(t, "POST", capPath, create, key(), 404)
	f.expect(t, "POST", path, action("activate", "emisell-pay-alt", nil), key(), 200)
	switched := f.expect(t, "POST", capPath, create, key(), 200)
	if switched["installationId"] == payment["installationId"] {
		t.Fatal("resolver did not switch installation")
	}
	f.expect(t, "POST", capPath, map[string]string{"operation": "status", "resourceId": paymentID}, key(), 404)
	// No active scopes/routing survive uninstall, including in stored rows.
	var scopes, caps int
	err := f.pool.QueryRow(context.Background(), "SELECT jsonb_array_length(scopes),jsonb_array_length(capabilities) FROM platform_installation.installations WHERE tenant_id=$1 AND app_id='emisell-pay'", f.tenant).Scan(&scopes, &caps)
	if err != nil || scopes != 0 || caps != 0 {
		t.Fatal("persisted grant revocation failed", err)
	}
	f.expect(t, "POST", "/api/v1/logout", map[string]any{}, "", 200)
	f.expect(t, "GET", "/api/v1/session", nil, "", 401)
}

func TestConcurrentRetriesAreAtomic(t *testing.T) {
	f := setup(t)
	path := f.installPath(f.tenant)
	k := key()
	body := action("install", "emisell-pay", payScopes)
	results := make(chan string, 20)
	failures := make(chan error, 20)
	var wait sync.WaitGroup
	for range 20 {
		wait.Go(func() {
			status, result, _, err := f.call("POST", path, body, k, origin)
			if err != nil || status != 200 {
				failures <- fmt.Errorf("status=%d err=%v", status, err)
				return
			}
			results <- result["installation"].(map[string]any)["id"].(string)
		})
	}
	wait.Wait()
	close(results)
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	var first string
	for id := range results {
		if first == "" {
			first = id
		}
		if first != id {
			t.Fatal("duplicate installation under concurrency")
		}
	}
	events := f.expect(t, "GET", "/api/v1/workspaces/"+f.tenant+"/events", nil, "", 200)["events"].([]any)
	if len(events) != 1 {
		t.Fatalf("expected one audit event, got %d", len(events))
	}
	var count int
	if err := f.pool.QueryRow(context.Background(), "SELECT count(*) FROM platform_installation.idempotency WHERE tenant_id=$1", f.tenant).Scan(&count); err != nil || count != 1 {
		t.Fatal("idempotency not atomic", err)
	}
	f.expect(t, "POST", path, action("activate", "emisell-pay", nil), key(), 200)
	capPath := f.capPath(f.tenant, "payment")
	capKey := key()
	capBody := map[string]any{"operation": "create", "reference": "concurrent", "amountMinor": 1000, "currency": "IDR"}
	capResults := make(chan string, 20)
	capErrors := make(chan error, 20)
	for range 20 {
		wait.Go(func() {
			status, result, _, err := f.call("POST", capPath, capBody, capKey, origin)
			if err != nil || status != 200 {
				capErrors <- fmt.Errorf("capability status=%d err=%v", status, err)
				return
			}
			capResults <- result["resource"].(map[string]any)["id"].(string)
		})
	}
	wait.Wait()
	close(capResults)
	close(capErrors)
	for err := range capErrors {
		t.Error(err)
	}
	first = ""
	for id := range capResults {
		if first == "" {
			first = id
		}
		if first != id {
			t.Fatal("duplicate simulated payment")
		}
	}
	if err := f.pool.QueryRow(context.Background(), "SELECT count(*) FROM platform_capability.events WHERE tenant_id=$1 AND envelope->>'type'='emisell.capability.invoked.v1'", f.tenant).Scan(&count); err != nil || count != 1 {
		t.Fatal("capability audit/idempotency not atomic", err)
	}
	if err := f.pool.QueryRow(context.Background(), "SELECT count(*) FROM platform_capability.events WHERE tenant_id=$1 AND envelope->>'type'='emisell.payment.status_changed.v1'", f.tenant).Scan(&count); err != nil || count != 1 {
		t.Fatal("payment event/idempotency not atomic", err)
	}
	if _, err := f.pool.Exec(context.Background(), "UPDATE platform_installation.installations SET scopes='[\"payments.write\"]'::jsonb WHERE tenant_id=$1", f.tenant); err != nil {
		t.Fatal(err)
	}
	f.expect(t, "POST", capPath, capBody, key(), 403)
}

func TestHTTPBoundariesAndSessionExpiry(t *testing.T) {
	f := setup(t)
	status, _, _, err := f.call("POST", f.installPath(f.tenant), action("install", "emisell-pay", payScopes), key(), "https://evil.example")
	if err != nil || status != 403 {
		t.Fatal("CSRF accepted", status, err)
	}
	invalid := action("install", "emisell-pay", payScopes)
	invalid["tenantId"] = f.other
	f.expect(t, "POST", f.installPath(f.tenant), invalid, key(), 400)
	f.expect(t, "POST", f.installPath(f.tenant), action("install", "emisell-pay", payScopes), "", 400)
	f.expect(t, "POST", "/api/v1/login", map[string]string{"email": f.email, "password": "wrong-password"}, "", 401)
	// A different principal cannot read, modify, or invoke the first tenant.
	foreign := setup(t)
	foreign.expect(t, "GET", f.installPath(f.tenant), nil, "", 404)
	foreign.expect(t, "POST", f.installPath(f.tenant), action("install", "emisell-pay", payScopes), key(), 404)
	foreign.expect(t, "POST", f.capPath(f.tenant, "payment"), map[string]string{"operation": "status", "resourceId": "unknown"}, key(), 404)
	if _, err = f.pool.Exec(context.Background(), "UPDATE platform_identity.sessions SET expires_at=$1 WHERE user_id=$2", time.Now().Add(-time.Hour), f.user); err != nil {
		t.Fatal(err)
	}
	f.expect(t, "GET", "/api/v1/session", nil, "", 401)
	if _, err = f.pool.Exec(context.Background(), "UPDATE platform_app.releases SET version='9.0.0' WHERE app_id='emisell-pay'"); err == nil {
		t.Fatal("published release mutated")
	}
}
