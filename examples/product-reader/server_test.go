package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestEncryptedStorePersistenceAndSingleWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.enc")
	key := bytes.Repeat([]byte{7}, 32)
	s, err := openStore(path, key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := openStore(path, key); err == nil {
		t.Fatal("second writer accepted")
	}
	if err := s.change("cookie-secret", func(item *session) error {
		*item = session{Token: "es_at_private", ExpiresAt: time.Now().Add(time.Hour)}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	data, _ := os.ReadFile(path)
	if bytes.Contains(data, []byte("es_at_private")) || bytes.Contains(data, []byte("cookie-secret")) {
		t.Fatal("plaintext secrets persisted")
	}
	s, err = openStore(path, key)
	if err != nil {
		t.Fatal(err)
	}
	item, ok := s.get("cookie-secret")
	if !ok || item.Token != "es_at_private" {
		t.Fatal("session did not survive restart")
	}
	s.Close()
	if wrong, err := openStore(path, bytes.Repeat([]byte{8}, 32)); err == nil {
		wrong.Close()
		t.Fatal("wrong key accepted")
	}
	data[len(data)-1] ^= 1
	if os.WriteFile(path, data, 0600) != nil {
		t.Fatal("test write")
	}
	if broken, err := openStore(path, key); err == nil {
		broken.Close()
		t.Fatal("tampered store accepted")
	}
}

func TestProviderOAuthAndReadBoundary(t *testing.T) {
	var exchanges atomic.Int32
	var challenge string
	var revoked atomic.Bool
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/oauth/token" {
			exchanges.Add(1)
			user, password, ok := r.BasicAuth()
			if !ok || user != "client" || password != "client-secret" {
				t.Error("invalid client authentication")
			}
			_ = r.ParseForm()
			computed := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			if base64.RawURLEncoding.EncodeToString(computed[:]) != challenge {
				t.Error("invalid PKCE")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "es_at_private-token", "token_type": "Bearer", "expires_in": 3600, "scope": "read_products",
				"installation": map[string]any{"id": "installation-a", "appId": "app-a", "merchantId": "merchant-a", "merchantName": "<script>alert(1)</script>", "status": "active"}})
			return
		}
		if r.Header.Get("Authorization") != "Bearer es_at_private-token" || revoked.Load() {
			w.WriteHeader(401)
			return
		}
		if r.Header.Get("Cookie") != "" || r.Header.Get("X-Merchant-ID") != "" {
			t.Error("browser headers forwarded")
		}
		_, _ = io.WriteString(w, `{"data":[{"id":"product-a","name":"<script>bad</script>","price":"60000.5","sku":"sku","stock":45}],"meta":{"nextCursor":null}}`)
	}))
	defer gateway.Close()
	s, err := openStore(filepath.Join(t.TempDir(), "reader.enc"), bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := newReader(configuration{AppID: "app-a", ClientID: "client", ClientSecret: "client-secret", GatewayURL: gateway.URL,
		ConsentURL: "https://platform.example.invalid/install", ConnectedAppsURL: "https://platform.example.invalid/merchant/apps"}, s)
	server := httptest.NewServer(a.handler())
	defer server.Close()
	a.config.PublicURL = server.URL
	probe, err := http.Head(server.URL + "/?code=probe&state=probe")
	if err != nil {
		t.Fatal(err)
	}
	probe.Body.Close()
	if probe.StatusCode != http.StatusMethodNotAllowed || exchanges.Load() != 0 {
		t.Fatal("HEAD must not process an OAuth callback")
	}
	if policy := probe.Header.Get("Content-Security-Policy"); !strings.Contains(policy, "form-action 'self' https://platform.example.invalid;") || strings.Contains(policy, "*") {
		t.Fatal("CSP must allow only the configured consent origin")
	}
	if probe.Header.Get("Referrer-Policy") != "same-origin" {
		t.Fatal("native form POST needs a non-null same-origin Origin")
	}
	jar, _ := cookiejar.New(nil)
	browser := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	get := func(raw string) (int, string, string) {
		t.Helper()
		res, err := browser.Get(raw)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(data), res.Header.Get("Location")
	}
	if status, _, _ := get(server.URL + "/?emisell_test_install_request=01995f72-0000-7000-8000-000000000001"); status != 303 {
		t.Fatal("launch failed")
	}
	_, home, _ := get(server.URL + "/")
	csrf := regexp.MustCompile(`name="csrf" value="([^"]+)"`).FindStringSubmatch(home)[1]
	connect := func(origin, value string) (int, string) {
		t.Helper()
		req, _ := http.NewRequest("POST", server.URL+"/connect", strings.NewReader(url.Values{"csrf": {value}}.Encode()))
		req.Header.Set("Origin", origin)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		res, err := browser.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		return res.StatusCode, res.Header.Get("Location")
	}
	if status, _ := connect("https://evil.invalid", csrf); status != 403 {
		t.Fatal("foreign origin accepted")
	}
	if status, _ := connect("null", csrf); status != 403 {
		t.Fatal("opaque/null origin accepted")
	}
	if status, _ := connect(server.URL, "wrong"); status != 403 {
		t.Fatal("wrong CSRF accepted")
	}
	status, location := connect(server.URL, csrf)
	if status != 303 {
		t.Fatal("connect failed")
	}
	consent, _ := url.Parse(location)
	challenge = consent.Query().Get("code_challenge")
	callback := server.URL + "/?code=test-code&state=" + consent.Query().Get("state")
	if status, _, _ := get(server.URL + "/?code=test-code&state=" + strings.Repeat("z", 43)); status != 400 || exchanges.Load() != 0 {
		t.Fatal("invalid state reached token exchange")
	}
	foreign, err := http.Get(callback)
	if err != nil {
		t.Fatal(err)
	}
	foreign.Body.Close()
	if foreign.StatusCode != 400 {
		t.Fatal("foreign browser accepted")
	}
	if status, _, location := get(callback); status != 303 || location != "/products" {
		t.Fatal("callback failed")
	}
	if status, _, _ := get(callback); status != 400 || exchanges.Load() != 1 {
		t.Fatal("callback replay accepted")
	}
	if status, body, _ := get(server.URL + "/products"); status != 200 || strings.Contains(body, "es_at_") || strings.Contains(body, "client-secret") || strings.Contains(body, "<script>") {
		t.Fatal("unsafe product HTML")
	}
	if status, _, _ := get(server.URL + "/api/products?merchantId=other"); status != 400 {
		t.Fatal("tenant selector accepted")
	}
	revoked.Store(true)
	if status, body, _ := get(server.URL + "/products"); status != 401 || strings.Contains(body, "Instalasi terhubung.") {
		t.Fatal("revoked HTML response must not claim the app is still connected")
	}
	if status, _, _ := get(server.URL + "/api/products"); status != 401 {
		t.Fatal("revocation ignored")
	}
	u, _ := url.Parse(server.URL)
	item, _ := s.get(jar.Cookies(u)[0].Value)
	if item.Token != "" {
		t.Fatal("revoked token not removed")
	}
}

func TestOrigins(t *testing.T) {
	for _, raw := range []string{"http://evil.invalid", "http://localhost.evil.invalid", "http://127.0.0.2", "https://user:password@host.invalid", "https://host.invalid/path", "https://host.invalid?secret=value"} {
		if validateOrigin(raw, true) {
			t.Fatalf("unsafe origin: %s", raw)
		}
	}
	if validateOrigin("http://localhost:3014", false) || !validateOrigin("http://localhost:3014", true) {
		t.Fatal("local HTTP policy")
	}
}
