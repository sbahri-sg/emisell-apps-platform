package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"emisell-app-platform/services/app-gateway/internal/adapters/postgres"
	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/database"
	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/httpapi"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/security"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAdminLoginPostgres(t *testing.T) {
	raw := os.Getenv("ADMIN_LOGIN_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("use npm run test:admin-login for an isolated database")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() != "127.0.0.1" || !strings.HasPrefix(u.Path, "/emisell_admin_login_test_") {
		t.Fatal("refusing non-disposable database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, raw)
	if err != nil {
		t.Fatal("open disposable database")
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := database.MigrateAdminLogin(ctx, pool); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	clock := func() time.Time { return now }
	repo := postgres.NewRepository(pool, ids.NewUUIDv7, clock)
	service, err := application.NewAdminLoginService(repo, ids.NewUUIDv7, clock)
	if err != nil {
		t.Fatal(err)
	}
	password := "Only for disposable admin tests!"
	orgID, err := ids.NewUUIDv7()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO organizations (id, name, slug) VALUES ($1::uuid, 'Emisell Internal', 'emisell-internal')`, orgID); err != nil {
		t.Fatal(err)
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	create := func(email string) domain.AdminAccount {
		t.Helper()
		id, err := ids.NewUUIDv7()
		if err != nil {
			t.Fatal(err)
		}
		a := domain.AdminAccount{UserID: id, OrganizationID: orgID, Email: email, DisplayName: "Test Admin", PasswordHash: hash}
		if err := repo.CreateAdminAccount(ctx, a); err != nil {
			t.Fatal(err)
		}
		return a
	}
	admin := create("admin@example.test")
	if err := repo.CreateAdminAccount(ctx, admin); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("duplicate identity accepted")
	}
	server := httpapi.NewServer(httpapi.Dependencies{
		AdminLogin: service, DeveloperProgram: application.NewDeveloperProgramService(repo, ids.NewUUIDv7, clock, time.Hour),
		Authenticator: httpapi.DevelopmentAuthenticator{BearerToken: "old-dev-token", DefaultRole: domain.RoleOwner, PlatformOperator: true},
		IdentityHTTP:  httpapi.IdentityHTTPOptions{FrontendURL: "https://console.example.test", CookieSecure: true},
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	request := func(method, path, body, origin string, cookies []*http.Cookie, csrf string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", origin)
		r.Header.Set("X-CSRF-Token", csrf)
		r.Header.Set("Authorization", "Bearer old-dev-token")
		r.Header.Set("X-Organization-Id", "00000000-0000-4000-8000-000000000001")
		for _, cookie := range cookies {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		server.ServeHTTP(w, r)
		return w
	}
	loginBody := fmt.Sprintf(`{"email":"ADMIN@example.test","password":%q}`, password)
	for _, origin := range []string{"", "null", "https://attacker.example"} {
		if w := request("POST", "/auth/admin/login", loginBody, origin, nil, ""); w.Code != 403 {
			t.Fatalf("login origin status %d", w.Code)
		}
	}
	for _, body := range []string{`{"email":"admin@example.test","password":"incorrect"}`, `{"email":"unknown@example.test","password":"incorrect"}`} {
		w := request("POST", "/auth/admin/login", body, "https://console.example.test", nil, "")
		if w.Code != 401 || strings.Contains(w.Body.String(), "admin@example") {
			t.Fatal("credentials must fail generically")
		}
	}
	w := request("POST", "/auth/admin/login", loginBody, "https://console.example.test", nil, "")
	if w.Code != 200 {
		t.Fatalf("login status = %d", w.Code)
	}
	cookies := w.Result().Cookies()
	var token, csrf string
	for _, c := range cookies {
		if !c.Secure || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || c.MaxAge != 28800 {
			t.Fatal("unsafe cookie attributes")
		}
		if c.Name == httpapi.AdminSessionCookie {
			token = c.Value
			if !c.HttpOnly {
				t.Fatal("session must be HttpOnly")
			}
		}
		if c.Name == httpapi.AdminCSRFCookie {
			csrf = c.Value
			if c.HttpOnly {
				t.Fatal("CSRF must be readable")
			}
		}
	}
	if len(token) != 43 || len(csrf) != 43 || strings.Contains(w.Body.String(), token) || strings.Contains(w.Body.String(), password) {
		t.Fatal("secret in response or invalid token")
	}
	for _, path := range []string{"/auth/admin/session", "/v1/internal/developer-applications", "/v1/internal/organizations"} {
		if w := request("GET", path, "", "", nil, ""); w.Code != 401 {
			t.Fatalf("dev bearer bypassed admin on %s (%d)", path, w.Code)
		}
		if w := request("GET", path, "", "", []*http.Cookie{{Name: "emisell_session", Value: token}}, ""); w.Code != 401 {
			t.Fatal("developer cookie accepted")
		}
	}
	for _, path := range []string{"/auth/admin/session", "/v1/internal/developer-applications"} {
		if w := request("GET", path, "", "", cookies, ""); w.Code != 200 {
			t.Fatalf("admin access status %d on %s", w.Code, path)
		}
	}
	for _, value := range []string{"", "incorrect"} {
		if w := request("POST", "/auth/admin/logout", "", "", cookies, value); w.Code != 403 {
			t.Fatal("missing/invalid CSRF accepted")
		}
	}
	// Authentication survives a gateway-service restart, but logout is durable.
	restarted, err := application.NewAdminLoginService(repo, ids.NewUUIDv7, clock)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.Authenticate(ctx, token); err != nil {
		t.Fatal("session did not persist")
	}
	if w := request("POST", "/auth/admin/logout", "", "", cookies, csrf); w.Code != 204 {
		t.Fatalf("logout status %d", w.Code)
	}
	if _, err := restarted.Authenticate(ctx, token); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatal("revoked session accepted")
	}
	if w := request("GET", "/auth/admin/session", "", "", cookies, ""); w.Code != 401 {
		t.Fatal("expired cookie fell back to bearer")
	}

	t.Run("password reset and disable revoke sessions", func(t *testing.T) {
		a := create("reset@example.test")
		issued, err := service.Login(ctx, a.Email, password, "192.0.2.21")
		if err != nil {
			t.Fatal(err)
		}
		newHash, err := security.HashPassword("A different long test password!")
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.UpdateAdminPassword(ctx, a.Email, newHash, false, now); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Authenticate(ctx, issued.SessionToken); !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatal("reset did not revoke")
		}
		stale := issued.Session
		stale.ID, _ = ids.NewUUIDv7()
		stale.TokenHash = application.IdentityTokenDigest("new test token")
		if err := repo.CreateAdminSession(ctx, stale, hash); !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatal("concurrent stale login accepted")
		}
		issued, err = service.Login(ctx, a.Email, "A different long test password!", "192.0.2.21")
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.UpdateAdminPassword(ctx, a.Email, "", true, now); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Authenticate(ctx, issued.SessionToken); !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatal("disabled session accepted")
		}
		if _, err := service.Login(ctx, a.Email, "A different long test password!", "192.0.2.21"); !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatal("disabled login accepted")
		}
	})
	t.Run("idle and absolute expiry", func(t *testing.T) {
		a := create("expiry@example.test")
		issued, err := service.Login(ctx, a.Email, password, "192.0.2.22")
		if err != nil {
			t.Fatal(err)
		}
		now = now.Add(31 * time.Minute)
		if _, err := service.Authenticate(ctx, issued.SessionToken); !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatal("idle session accepted")
		}
		issued, err = service.Login(ctx, a.Email, password, "192.0.2.22")
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 23; i++ {
			now = now.Add(20 * time.Minute)
			if _, err := service.Authenticate(ctx, issued.SessionToken); err != nil {
				t.Fatal(err)
			}
		}
		now = now.Add(20 * time.Minute)
		if _, err := service.Authenticate(ctx, issued.SessionToken); !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatal("absolute expiry extended")
		}
	})
	t.Run("durable rate limits are atomic and expire", func(t *testing.T) {
		var allowed atomic.Int32
		var group sync.WaitGroup
		for i := 0; i < 20; i++ {
			group.Go(func() {
				ok, err := repo.ConsumeAdminLoginAttempt(ctx, "concurrent-bucket", 5, now)
				if err != nil {
					t.Error(err)
				}
				if ok {
					allowed.Add(1)
				}
			})
		}
		group.Wait()
		if allowed.Load() != 5 {
			t.Fatalf("allowed %d attempts, expected 5", allowed.Load())
		}
		if ok, err := repo.ConsumeAdminLoginAttempt(ctx, "concurrent-bucket", 5, now.Add(16*time.Minute)); err != nil || !ok {
			t.Fatal("limit failed to expire")
		}
		for i := 0; i < 5; i++ {
			_, err := service.Login(ctx, "limited@example.test", password, "192.0.2.44")
			if !errors.Is(err, domain.ErrUnauthorized) {
				t.Fatal(err)
			}
		}
		if _, err := restarted.Login(ctx, "LIMITED@example.test", password, "192.0.2.45"); !errors.Is(err, application.ErrAdminLoginLimited) {
			t.Fatal("email limit lost on restart/IP change")
		}
	})
}
