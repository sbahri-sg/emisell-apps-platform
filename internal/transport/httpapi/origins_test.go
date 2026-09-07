package httpapi

import (
	"emisell.app/platform/internal/platform/config"
	"log/slog"
	"net/http/httptest"
	"testing"
)

func TestPortalCookiePublicDomain(t *testing.T) {
	for _, secure := range []bool{false, true} {
		s := Server{publicOrigins: config.PublicOrigins{Secure: secure}}
		for _, age := range []int{3600, -1} {
			w := httptest.NewRecorder()
			s.setPortalCookie(w, "admin", "test-token", age)
			cookies := w.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatal("missing cookie")
			}
			c := cookies[0]
			if c.Secure != secure || !c.HttpOnly || c.Domain != "" || c.Path != "/api/v1/admin" || c.MaxAge != age {
				t.Fatal("unsafe cookie attributes")
			}
		}
	}
}

func TestProductionDoesNotExposeLegacySimulator(t *testing.T) {
	t.Setenv("EMISELL_ENV", "production")
	t.Setenv("EMISELL_ADMIN_ORIGIN", "https://admin.example.com")
	t.Setenv("EMISELL_DEVELOPER_ORIGIN", "https://developer.example.com")
	t.Setenv("EMISELL_STORE_ORIGIN", "https://apps.example.com")
	h := (Server{Origin: "https://admin.example.com", Logger: slog.Default()}).Handler()
	for _, path := range []string{"/api/v1/login", "/api/v1/installations", "/api/v1/workspaces"} {
		req := httptest.NewRequest("GET", "https://admin.example.com"+path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 404 {
			t.Fatalf("legacy route exposed: %s %d", path, w.Code)
		}
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "https://evil.example.com/healthz", nil)
	req.Header.Set("X-Forwarded-Host", "admin.example.com")
	h.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatal("trusted forged forwarded host")
	}
}

func TestDashboardOnlyDisablesStore(t *testing.T) {
	t.Setenv("EMISELL_ENV", "production")
	t.Setenv("EMISELL_ADMIN_ORIGIN", "")
	t.Setenv("EMISELL_DEVELOPER_ORIGIN", "")
	t.Setenv("EMISELL_DASHBOARD_ORIGIN", "https://dashboard.example.com")
	t.Setenv("EMISELL_STORE_ORIGIN", "")
	t.Setenv("EMISELL_STORE_DISABLED", "true")
	h := (Server{Origin: "https://dashboard.example.com", Logger: slog.Default()}).Handler()
	for _, path := range []string{"/api/v1/store/apps", "/api/v1/store/session"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "https://dashboard.example.com"+path, nil))
		if w.Code != 404 {
			t.Fatalf("disabled store exposed: %s %d", path, w.Code)
		}
	}
}
