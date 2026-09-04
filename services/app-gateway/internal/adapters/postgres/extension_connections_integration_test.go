package postgres_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	postgresadapter "emisell-app-platform/services/app-gateway/internal/adapters/postgres"
	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/database"
	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/httpapi"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/ports"
	"emisell-app-platform/services/app-gateway/internal/security"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestExtensionConnectionsPostgres(t *testing.T) {
	// This test changes lifecycle state and installs a failing audit trigger. Never
	// accept the normal DATABASE_URL / TEST_DATABASE_URL or a shared database.
	raw := os.Getenv("EXTENSION_CONNECTION_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("run npm run test:extensions for disposable PostgreSQL verification")
	}
	u, err := url.Parse(raw)
	if err != nil || u.User == nil || u.Hostname() != "127.0.0.1" || u.User.Username() != "extension_test" || !regexp.MustCompile(`^/emisell_extensions_test_[a-f0-9]{24}$`).MatchString(u.Path) {
		t.Fatal("refusing non-disposable database")
	}
	ctx := t.Context()
	pool, err := database.Open(ctx, database.Options{URL: raw, MaxConnections: 8, ConnectTimeout: 5 * time.Second})
	if err != nil {
		t.Fatal("cannot open disposable database")
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	org, user := mustID(t), mustID(t)
	if err := database.BootstrapDevelopmentIdentity(ctx, pool, org, user, "owner", user+"@example.test", "Extension Test"); err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO organization_entitlements(organization_id,sandbox_access,production_access,max_apps,max_webhooks,created_at,updated_at) VALUES($1,true,false,20,20,now(),now()) ON CONFLICT(organization_id) DO UPDATE SET sandbox_access=true`, org)
	now := func() time.Time { return time.Now().UTC() }
	repo := postgresadapter.NewRepository(pool, ids.NewUUIDv7, now)
	meta := func(action string) ports.MutationMeta {
		return ports.MutationMeta{ActorID: user, Action: action, IdempotencyKey: mustID(t)}
	}
	app, err := repo.CreateApp(ctx, domain.App{ID: mustID(t), OrganizationID: org, Name: "Managed test", Slug: "managed-test", Status: domain.AppStatusDraft, Distribution: domain.DistributionCustom, CreatedBy: user, CreatedAt: now(), UpdatedAt: now(), Revision: 1}, meta("app.created"))
	if err != nil {
		t.Fatal(err)
	}
	ext, err := repo.CreateExtension(ctx, org, domain.AppExtension{ID: mustID(t), AppID: app.ID, Name: "Managed test extension", Type: domain.ExtensionTypeShipping, Status: domain.ExtensionStatusDraft, Configuration: map[string]any{}, CreatedAt: now(), UpdatedAt: now(), Revision: 1}, meta("extension.created"))
	if err != nil {
		t.Fatal(err)
	}
	version, err := repo.CreateVersion(ctx, org, domain.AppVersion{ID: mustID(t), AppID: app.ID, Version: "1.0.0", Status: domain.VersionStatusDraft, CreatedBy: user, CreatedAt: now(), Snapshot: domain.VersionSnapshot{Extensions: []domain.SnapshotExtension{{ExtensionID: ext.ID, Type: string(ext.Type)}}, Scopes: []domain.SnapshotScope{{Scope: "read_merchant", Access: string(domain.ScopeAccessRequired)}}, WebhookSubscriptions: []domain.SnapshotWebhook{}, RedirectURLs: []string{}, ConfigurationHash: "extension-test-snapshot"}}, meta("version.created"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ActivateVersion(ctx, org, app.ID, version.ID, domain.VersionStatusDraft, nil, meta("version.released")); err != nil {
		t.Fatal(err)
	}
	install := func(merchant string) domain.AppInstallation {
		i, err := repo.CreateInstallation(ctx, org, domain.AppInstallation{ID: mustID(t), AppID: app.ID, MerchantID: merchant, MerchantName: "Test merchant", Environment: domain.EnvironmentSandbox, Status: domain.InstallationStatusActive, InstalledVersionID: version.ID, GrantedScopes: []string{"read_merchant"}, InstalledBy: user, InstalledAt: now(), CreatedAt: now(), UpdatedAt: now(), Revision: 1}, version.ID, meta("installation.created"))
		if err != nil {
			t.Fatal(err)
		}
		return i
	}
	a, b := install("merchant-test-a"), install("merchant-test-b")
	cipher, err := security.NewSecretBox(bytes.Repeat([]byte{17}, 32))
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewExtensionConnectionService(repo, cipher, 1, ids.NewUUIDv7, now, application.DevelopmentResourcePilot{})
	server := func(operator bool, enabled bool) http.Handler {
		var connections *application.ExtensionConnectionService
		if enabled {
			connections = service
		}
		return httpapi.NewServer(httpapi.Dependencies{ExtensionConnections: connections, Authenticator: httpapi.DevelopmentAuthenticator{BearerToken: "operator-test-credential", DefaultUserID: user, DefaultRole: domain.RoleOwner, PlatformOperator: operator}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	}
	handler := server(true, true)
	call := func(h http.Handler, method, path, token string, body any, expected int, headers map[string]string) []byte {
		t.Helper()
		data, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(data))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Organization-Id", org)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		for key, value := range headers {
			r.Header.Set(key, value)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != expected {
			t.Fatalf("%s %s: got %d, want %d", method, path, w.Code, expected)
		}
		if expected == 200 && !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
			t.Fatal("missing no-store")
		}
		if expected >= 400 && (strings.Contains(w.Body.String(), "synthetic-key-") || strings.Contains(w.Body.String(), "runtimeToken")) {
			t.Fatal("error exposed protected material")
		}
		return w.Body.Bytes()
	}
	path := func(i domain.AppInstallation) string {
		return "/v1/internal/organizations/" + org + "/apps/" + app.ID + "/installations/" + i.ID + "/extensions/" + ext.ID + "/connection"
	}
	const runtimePath = "/v1/runtime/extension-credentials/resolve"
	decodeConnection := func(data []byte) application.ExtensionConnectionResult {
		var result struct {
			Data application.ExtensionConnectionResult
		}
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatal(err)
		}
		return result.Data
	}
	provision := func(i domain.AppInstallation, key string, revision int64) application.ExtensionConnectionResult {
		return decodeConnection(call(handler, "PUT", path(i), "operator-test-credential", map[string]any{"revision": revision, "runtimeName": "test-runtime", "scopes": []string{"read_merchant"}, "secret": map[string]string{"apiKey": key}, "runtimeExpiresAt": now().Add(time.Hour)}, 200, nil))
	}
	if result := decodeConnection(call(handler, "GET", path(a), "operator-test-credential", nil, 200, nil)); result.Mode != "external" || result.Connection != nil {
		t.Fatal("external default mismatch")
	}
	call(server(false, true), "GET", path(a), "operator-test-credential", nil, 403, nil)
	call(server(true, false), "GET", path(a), "operator-test-credential", nil, 503, nil)
	call(handler, "PUT", path(a), "", nil, 401, nil)
	ca, cb := provision(a, "synthetic-key-a", 0), provision(b, "synthetic-key-b", 0)
	foreignApp, err := repo.CreateApp(ctx, domain.App{ID: mustID(t), OrganizationID: org, Name: "Other managed test", Slug: "other-managed-test", Status: domain.AppStatusDraft, Distribution: domain.DistributionCustom, CreatedBy: user, CreatedAt: now(), UpdatedAt: now(), Revision: 1}, meta("app.created"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `UPDATE extension_connections SET app_id=$1 WHERE id=$2`, foreignApp.ID, ca.Connection.ID)
	if pg, ok := err.(*pgconn.PgError); !ok || pg.Code != "23503" {
		t.Fatal("cross-app connection must fail foreign key validation")
	}
	_, err = pool.Exec(ctx, `UPDATE extension_connections SET secret_ciphertext=NULL WHERE id=$1`, ca.Connection.ID)
	if pg, ok := err.(*pgconn.PgError); !ok || pg.Code != "23514" {
		t.Fatal("active connection must require ciphertext")
	}
	call(handler, "GET", strings.Replace(path(a), org, mustID(t), 1), "operator-test-credential", nil, 404, nil)
	call(server(false, true), "GET", path(a), "operator-test-credential", nil, 403, map[string]string{"X-Platform-Operator": "true"})
	for _, item := range []struct{ token, merchant, key string }{{ca.RuntimeToken, a.MerchantID, "synthetic-key-a"}, {cb.RuntimeToken, b.MerchantID, "synthetic-key-b"}} {
		data := call(handler, "POST", runtimePath, item.token, map[string]string{"scope": "read_merchant"}, 200, map[string]string{"X-Merchant-Id": "attacker-merchant", "X-Installation-Id": mustID(t)})
		var result struct {
			Data application.ResolvedExtensionCredential
		}
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatal(err)
		}
		if result.Data.MerchantID != item.merchant || result.Data.Secret["apiKey"] != item.key {
			t.Fatal("merchant binding broken")
		}
	}
	for _, token := range []string{"operator-test-credential", "es_at_not-a-runtime-token", "er_" + strings.Repeat("x", 43)} {
		call(handler, "POST", runtimePath, token, map[string]string{"scope": "read_merchant"}, 401, nil)
	}
	call(handler, "GET", path(a), ca.RuntimeToken, nil, 401, nil)
	call(handler, "POST", runtimePath, ca.RuntimeToken, map[string]string{"scope": "read_merchant", "merchantId": b.MerchantID}, 422, nil)
	call(handler, "POST", runtimePath+"?merchantId="+b.MerchantID, ca.RuntimeToken, map[string]string{"scope": "read_merchant"}, 422, nil)
	call(handler, "POST", runtimePath, ca.RuntimeToken, map[string]string{"scope": "read_orders"}, 403, nil)
	for _, headers := range []map[string]string{{"Cookie": "emisell_session=synthetic"}, {"Origin": "http://localhost:3003"}} {
		call(handler, "POST", runtimePath, ca.RuntimeToken, map[string]string{"scope": "read_merchant"}, 401, headers)
	}
	metadata := call(handler, "GET", path(a), "operator-test-credential", nil, 200, nil)
	if strings.Contains(string(metadata), ca.RuntimeToken) || strings.Contains(string(metadata), "synthetic-key") || strings.Contains(string(metadata), "ciphertext") {
		t.Fatal("metadata leaked secrets")
	}
	var ciphertext []byte
	var digest string
	if err := pool.QueryRow(ctx, `SELECT secret_ciphertext,token_hash FROM extension_connections WHERE id=$1`, ca.Connection.ID).Scan(&ciphertext, &digest); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext, []byte("synthetic-key-a")) || digest == ca.RuntimeToken || len(digest) != 64 {
		t.Fatal("unsafe storage")
	}
	// Recreating the service/repository proves resolution is persisted, not process-local.
	service = application.NewExtensionConnectionService(postgresadapter.NewRepository(pool, ids.NewUUIDv7, now), cipher, 1, ids.NewUUIDv7, now, application.DevelopmentResourcePilot{})
	handler = server(true, true)
	call(handler, "POST", runtimePath, ca.RuntimeToken, map[string]string{"scope": "read_merchant"}, 200, nil)
	var originalB []byte
	if err := pool.QueryRow(ctx, `SELECT secret_ciphertext FROM extension_connections WHERE id=$1`, cb.Connection.ID).Scan(&originalB); err != nil {
		t.Fatal(err)
	}
	exec(`UPDATE extension_connections SET secret_ciphertext=$1 WHERE id=$2`, ciphertext, cb.Connection.ID)
	call(handler, "POST", runtimePath, cb.RuntimeToken, map[string]string{"scope": "read_merchant"}, 500, nil)
	exec(`UPDATE extension_connections SET secret_ciphertext=$1 WHERE id=$2`, originalB, cb.Connection.ID)
	// Every resolve re-reads lifecycle state. Restorations affect test-only rows.
	for _, test := range []struct {
		name, disable, restore string
		id                     string
	}{
		{"suspend installation", `UPDATE app_installations SET status='suspended' WHERE id=$1`, `UPDATE app_installations SET status='active' WHERE id=$1`, a.ID},
		{"remove merchant grant", `UPDATE app_installations SET granted_scopes='[]' WHERE id=$1`, `UPDATE app_installations SET granted_scopes='["read_merchant"]' WHERE id=$1`, a.ID},
		{"disable extension", `UPDATE app_extensions SET status='disabled' WHERE id=$1`, `UPDATE app_extensions SET status='active' WHERE id=$1`, ext.ID},
		{"suspend organization", `UPDATE organizations SET status='suspended' WHERE id=$1`, `UPDATE organizations SET status='active' WHERE id=$1`, org},
		{"remove entitlement", `UPDATE organization_entitlements SET sandbox_access=false WHERE organization_id=$1`, `UPDATE organization_entitlements SET sandbox_access=true WHERE organization_id=$1`, org},
	} {
		t.Run(test.name, func(t *testing.T) {
			exec(test.disable, test.id)
			call(handler, "POST", runtimePath, ca.RuntimeToken, map[string]string{"scope": "read_merchant"}, 403, nil)
			exec(test.restore, test.id)
		})
	}
	// Fail the audit insertion: plaintext must never leave an uncommitted access transaction.
	exec(`CREATE FUNCTION extension_test_fail_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic audit unavailable'; END $$`)
	exec(`CREATE TRIGGER extension_test_fail_audit BEFORE INSERT ON extension_credential_access_events FOR EACH ROW EXECUTE FUNCTION extension_test_fail_audit()`)
	call(handler, "POST", runtimePath, ca.RuntimeToken, map[string]string{"scope": "read_merchant"}, 500, nil)
	exec(`DROP TRIGGER extension_test_fail_audit ON extension_credential_access_events`)
	exec(`DROP FUNCTION extension_test_fail_audit()`)
	// Concurrent rotation has exactly one winner; the loser never receives a token.
	sel := ports.ExtensionConnectionSelector{OrganizationID: org, AppID: app.ID, InstallationID: a.ID, ExtensionID: ext.ID}
	var wg sync.WaitGroup
	type rotationOutcome struct {
		value application.ExtensionConnectionResult
		err   error
	}
	results := make(chan rotationOutcome, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, err := service.Rotate(ctx, sel, user, 1, now().Add(time.Hour))
			results <- rotationOutcome{value, err}
		}()
	}
	wg.Wait()
	close(results)
	var rotated application.ExtensionConnectionResult
	successes := 0
	for outcome := range results {
		if outcome.err != nil {
			if !errors.Is(outcome.err, domain.ErrConflict) || outcome.value.RuntimeToken != "" {
				t.Fatal("losing rotation must fail with a revision conflict and no token")
			}
			continue
		}
		if outcome.value.RuntimeToken != "" {
			successes++
			rotated = outcome.value
		}
	}
	if successes != 1 {
		t.Fatal("rotation concurrency broken")
	}
	call(handler, "POST", runtimePath, ca.RuntimeToken, map[string]string{"scope": "read_merchant"}, 401, nil)
	call(handler, "POST", runtimePath, rotated.RuntimeToken, map[string]string{"scope": "read_merchant"}, 200, nil)
	call(handler, "DELETE", path(a), "operator-test-credential", map[string]int{"revision": 2}, 204, nil)
	call(handler, "POST", runtimePath, rotated.RuntimeToken, map[string]string{"scope": "read_merchant"}, 401, nil)
	var wiped bool
	if err := pool.QueryRow(ctx, `SELECT secret_ciphertext IS NULL AND token_hash IS NULL FROM extension_connections WHERE id=$1`, ca.Connection.ID).Scan(&wiped); err != nil || !wiped {
		t.Fatal("revoke did not wipe credential")
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM extension_credential_access_events`).Scan(&count); err != nil || count != 4 {
		t.Fatalf("persisted successful access audit count=%d error=%v", count, err)
	}
	var audit, snapshot string
	if err := pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(a))::text,'[]') FROM audit_events a WHERE organization_id=$1`, org).Scan(&audit); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT snapshot::text FROM app_versions WHERE id=$1`, version.ID).Scan(&snapshot); err != nil {
		t.Fatal(err)
	}
	for _, content := range []string{audit, snapshot} {
		if strings.Contains(content, "synthetic-key-") || strings.Contains(content, ca.RuntimeToken) {
			t.Fatal("audit/snapshot exposed credential")
		}
	}
}
