package connectapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"emisell.app/platform/internal/identity"
	installservice "emisell.app/platform/internal/installation/service"
	"emisell.app/platform/internal/platform/fault"
)

const coreTestOrigin = "https://apps.example.test"
const coreTestPath = CoreHTTPPrefix + "/emisell.integration.v1.ConnectionService/Check"

func coreRequest() *http.Request {
	r := httptest.NewRequest("POST", coreTestOrigin+coreTestPath, strings.NewReader("{}"))
	r.Header.Set("Authorization", "Bearer epk_"+strings.Repeat("a", 43))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Connect-Protocol-Version", "1")
	return r
}

func TestCoreHTTPBoundary(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*http.Request)
		status int
	}{
		{"TLS", func(r *http.Request) {}, 200},
		{"Node fetch", func(r *http.Request) { r.Header.Set("Sec-Fetch-Mode", "cors") }, 200},
		{"trusted TLS proxy", func(r *http.Request) { r.TLS = nil; r.Header.Set("X-Forwarded-Proto", "https") }, 200},
		{"plaintext", func(r *http.Request) { r.TLS = nil }, 403},
		{"ambiguous proxy", func(r *http.Request) { r.TLS = nil; r.Header["X-Forwarded-Proto"] = []string{"https", "http"} }, 403},
		{"wrong host", func(r *http.Request) { r.Host = "evil.test" }, 403},
		{"missing key", func(r *http.Request) { r.Header.Del("Authorization") }, 401},
		{"legacy key", func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 43)) }, 401},
		{"duplicate key", func(r *http.Request) { r.Header.Add("Authorization", r.Header.Get("Authorization")) }, 401},
		{"empty browser origin", func(r *http.Request) { r.Header["Origin"] = []string{""} }, 403},
		{"browser cookie", func(r *http.Request) { r.Header.Set("Cookie", "session=secret") }, 403},
		{"browser fetch", func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "same-origin") }, 403},
		{"compressed", func(r *http.Request) { r.Header.Set("Content-Encoding", "gzip") }, 403},
		{"preflight", func(r *http.Request) { r.Method = "OPTIONS" }, 405},
		{"get", func(r *http.Request) { r.Method = "GET" }, 405},
		{"query", func(r *http.Request) { r.URL.RawQuery = "merchant=other" }, 404},
		{"empty query", func(r *http.Request) { r.URL.ForceQuery = true }, 404},
		{"encoded path", func(r *http.Request) { r.URL.RawPath = strings.Replace(r.URL.Path, "Check", "%43heck", 1) }, 404},
		{"payment", func(r *http.Request) { r.URL.Path = CoreHTTPPrefix + "/emisell.payment.v1.PaymentService/Create" }, 404},
		{"metrics", func(r *http.Request) { r.URL.Path = CoreHTTPPrefix + "/metrics" }, 404},
		{"unknown method", func(r *http.Request) { r.URL.Path = coreTestPath + "/extra" }, 404},
		{"traversal", func(r *http.Request) { r.URL.Path = CoreHTTPPrefix + "/../metrics" }, 404},
		{"wrong content type", func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }, 415},
		{"missing protocol", func(r *http.Request) { r.Header.Del("Connect-Protocol-Version") }, 415},
		{"duplicate protocol", func(r *http.Request) { r.Header.Add("Connect-Protocol-Version", "1") }, 415},
		{"invalid json", func(r *http.Request) { r.Body = io.NopCloser(strings.NewReader("{broken")) }, 400},
		{"large body", func(r *http.Request) { r.Body = io.NopCloser(strings.NewReader(strings.Repeat(" ", 32769))) }, 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			internal := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Host != "127.0.0.1" || r.URL.Path != strings.TrimPrefix(coreTestPath, CoreHTTPPrefix) || r.Context().Value(coreHTTPKey{}) != true {
					t.Fatal("invalid internal dispatch")
				}
				if _, ok := r.Context().Deadline(); !ok {
					t.Fatal("deadline missing")
				}
				w.WriteHeader(200)
			})
			h, err := ExposeCore(http.NotFoundHandler(), internal, coreTestOrigin, "true")
			if err != nil {
				t.Fatal(err)
			}
			r := coreRequest()
			tc.change(r)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("got %d, want %d", w.Code, tc.status)
			}
			if (calls == 1) != (tc.status == 200) {
				t.Fatal("unexpected dispatch")
			}
			if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("unsafe caching or CORS")
			}
			if strings.Contains(w.Body.String(), "epk_") {
				t.Fatal("credential leak")
			}
		})
	}
}

func TestCoreHTTPConfigurationAndPortal(t *testing.T) {
	portal := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(202) })
	for _, flag := range []string{"", "false"} {
		h, err := ExposeCore(portal, http.NotFoundHandler(), "", flag)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, coreRequest())
		if w.Code != 202 {
			t.Fatal("disabled wrapper changed portal")
		}
	}
	for _, origin := range []string{"http://remote.test", "https://user:pass@apps.test", coreTestOrigin + "/path", coreTestOrigin + "?", coreTestOrigin + "#x"} {
		if _, err := ExposeCore(portal, portal, origin, "true"); err == nil {
			t.Fatalf("accepted %s", origin)
		}
	}
	for _, origin := range []string{coreTestOrigin, "http://localhost:8087", "http://127.0.0.1:8087"} {
		h, err := ExposeCore(portal, http.NotFoundHandler(), origin, "true")
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", origin+"/api/v1/portal/session", nil))
		if w.Code != 202 {
			t.Fatal("portal changed")
		}
	}
}

type coreAccountRepo struct {
	identity.ServiceAccountRepository
	principal identity.ServicePrincipal
	revoked   bool
	calls     int
}

func (r *coreAccountRepo) FindPlatformService(ctx context.Context, hash string) (identity.ServicePrincipal, error) {
	r.calls++
	want := sha256.Sum256([]byte("epk_" + strings.Repeat("a", 43)))
	if r.revoked || hash != hex.EncodeToString(want[:]) {
		return identity.ServicePrincipal{}, fault.Unauthenticated
	}
	return r.principal, nil
}
func (r *coreAccountRepo) PlatformServiceAllowed(context.Context, string, string) (bool, error) {
	return false, nil
}

func TestCoreHTTPUsesExistingAuthenticationAndRevocation(t *testing.T) {
	repo := &coreAccountRepo{principal: identity.ServicePrincipal{ID: "platformkey_test", PlatformFull: true}}
	s := Server{Accounts: identity.ServiceAccounts{Repo: repo}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	s.Lifecycle = installservice.Lifecycle{Intents: installservice.Intents{Auth: s.Accounts}}
	h, err := ExposeCore(http.NotFoundHandler(), s.Handler(), coreTestOrigin, "true")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name          string
		full, revoked bool
		key           string
		status        int
	}{
		{"active", true, false, "a", 200},
		{"invalid", true, false, "b", 401},
		{"revoked", true, true, "a", 401},
		{"not first-party", false, false, "a", 403},
		{"active again", true, false, "a", 200},
	} {
		repo.principal.PlatformFull, repo.revoked = tc.full, tc.revoked
		r := coreRequest()
		r.Header.Set("Authorization", "Bearer epk_"+strings.Repeat(tc.key, 43))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("%s: %d %s", tc.name, w.Code, w.Body.String())
		}
	}
	if repo.calls != 5 {
		t.Fatal("auth result cached")
	}
	// A valid platform key does not auto-enroll an unknown merchant.
	r := coreRequest()
	r.URL.Path = CoreHTTPPrefix + "/emisell.installation.v1.InstallationService/ListInstallations"
	r.Body = io.NopCloser(strings.NewReader(`{"merchantId":"merchant-unknown","coreActorId":"owner-test"}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 404 {
		t.Fatalf("merchant check bypassed: %d %s", w.Code, w.Body.String())
	}
}
