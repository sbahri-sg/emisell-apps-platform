package embedded_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	service "emisell.app/platform/internal/oauth/embedded"
	"emisell.app/platform/pkg/embedded"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLaunchEndpointReferenceLifecycle(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	launch := embedded.Launch{AppID: "app", ClientID: "client", ReleaseDigest: strings.Repeat("a", 64), URL: "https://app.example/embedded", ParentOrigin: "https://core.example"}
	sig, err := embedded.SignLaunch(launch, key, false)
	if err != nil {
		t.Fatal(err)
	}
	id := embedded.Identity{MerchantID: "merchant", ActorID: "staff", AppID: "app", InstallationID: "installed"}
	var denied, offline bool
	s := service.Service{PrivateKey: key, Keys: map[string]ed25519.PublicKey{"id": pub}, KeyID: "id", Issuer: "https://platform.example", Authorize: func(_ context.Context, got embedded.Identity, aud string) error {
		if offline {
			return service.ErrUnavailable
		}
		if denied || got != id || aud != "client" {
			return service.ErrDenied
		}
		return nil
	}}
	l := service.Launcher{Identity: s, LaunchKey: pub, Resolve: func(_ context.Context, got embedded.Identity) (service.Binding, error) {
		if got != id {
			return service.Binding{}, service.ErrDenied
		}
		return service.Binding{Launch: launch, Signature: sig}, nil
	}}
	h := service.LaunchHandler(l, launch.ParentOrigin, func(_ context.Context, r *http.Request, installation string) (embedded.Identity, error) {
		// Test-only session boundary. Production must use Core session/membership/CSRF.
		if r.Header.Get("X-Test-Session") != "staff-session" {
			return embedded.Identity{}, service.ErrUnauthenticated
		}
		if r.Header.Get("X-Test-CSRF") != "valid" || installation != id.InstallationID {
			return embedded.Identity{}, service.ErrDenied
		}
		return id, nil
	})
	call := func(origin, session, csrf, body string, want int) service.Session {
		t.Helper()
		r := httptest.NewRequest("POST", "/embedded/session", strings.NewReader(body))
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Test-Session", session)
		r.Header.Set("X-Test-CSRF", csrf)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("status %d want %d", w.Code, want)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("cacheable identity")
		}
		var out service.Session
		if want == 200 {
			if json.Unmarshal(w.Body.Bytes(), &out) != nil {
				t.Fatal("response")
			}
		} else if strings.Contains(w.Body.String(), "identityToken") {
			t.Fatal("token in failure")
		}
		return out
	}
	body := `{"installationId":"installed"}`
	call(launch.ParentOrigin, "", "valid", body, 401)
	call("https://evil.example", "staff-session", "valid", body, 403)
	call(launch.ParentOrigin, "staff-session", "bad", body, 403)
	call(launch.ParentOrigin, "staff-session", "valid", `{"installationId":"installed","merchantId":"other"}`, 400)
	session := call(launch.ParentOrigin, "staff-session", "valid", body, 200)
	if _, err = s.Authenticate(context.Background(), session.Token, "client", id); err != nil {
		t.Fatal(err)
	}
	denied = true
	call(launch.ParentOrigin, "staff-session", "valid", body, 403)
	if _, err = s.Authenticate(context.Background(), session.Token, "client", id); err == nil {
		t.Fatal("revoked identity accepted")
	}
	denied = false
	offline = true
	call(launch.ParentOrigin, "staff-session", "valid", body, 503)
	offline = false
	launch.URL = "https://evil.example/"
	call(launch.ParentOrigin, "staff-session", "valid", body, 403)
}
