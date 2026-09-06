package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	app "emisell.app/platform/internal/oauth/embedded"
)

type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSellerBackendBoundary(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != "http://127.0.0.1:8000/v1/app-platform/core/embedded-demo/identity" || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "Bearer test-only" {
			t.Fatal("unsafe backchannel")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"status":"connected","demo":true}`)), Header: http.Header{}}, nil
	})}
	h := sellerIdentity(client)
	if w := request(h, "GET", appOrigin+"/seller/identity", "", map[string]string{"Authorization": "Bearer test-only"}); w.Code != 200 {
		t.Fatal(w.Code)
	}
	for _, headers := range []map[string]string{{}, {"Authorization": "Bearer test-only", "Cookie": "session=never-forward"}, {"Authorization": "Bearer test-only", "Origin": "https://evil.example"}} {
		if w := request(h, "GET", appOrigin+"/seller/identity", "", headers); w.Code != 403 {
			t.Fatal("unsafe request allowed")
		}
	}
	if calls != 1 {
		t.Fatal("invalid input reached Core")
	}
	d, _ := newDemo()
	_, frame := d.handlers()
	w := request(frame, "GET", appOrigin+"/seller", "", nil)
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors http://localhost:3000;") {
		t.Fatal("seller frame policy missing")
	}
}

func TestSellerBackendErrorClassification(t *testing.T) {
	for _, code := range []int{401, 403, 429, 500, 503} {
		client := &http.Client{Transport: testTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(`{"error":"private upstream detail"}`)), Header: http.Header{}}, nil
		})}
		w := request(sellerIdentity(client), "GET", appOrigin+"/seller/identity", "", map[string]string{"Authorization": "Bearer test"})
		want := 503
		if code == 401 || code == 403 {
			want = 403
		}
		if w.Code != want || strings.Contains(w.Body.String(), "private") {
			t.Fatalf("status %d mapped to %d", code, w.Code)
		}
	}
}

func request(h http.Handler, method, url, body string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, url, strings.NewReader(body))
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestDemoProtocolAndRevocation(t *testing.T) {
	d, err := newDemo()
	if err != nil {
		t.Fatal(err)
	}
	host, frame := d.handlers()
	headers := map[string]string{"Origin": parentOrigin, "Content-Type": "application/json", "X-Demo-CSRF": d.csrf}
	body := `{"installationId":"demo-installation"}`
	issue := func() app.Session {
		w := request(host, "POST", parentOrigin+"/demo/session", body, headers)
		if w.Code != 200 {
			t.Fatalf("issue: %d", w.Code)
		}
		var s app.Session
		if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
			t.Fatal(err)
		}
		if s.ExpiresIn != 60 || s.Launch.URL != appOrigin+"/" || strings.Contains(s.Launch.URL, s.Token) {
			t.Fatal("invalid session")
		}
		return s
	}
	s := issue()
	next := issue()
	if s.Token == next.Token {
		t.Fatal("renewal reused token")
	}
	verify := func(token string) int {
		return request(frame, "GET", appOrigin+"/demo/identity", "", map[string]string{"Authorization": "Bearer " + token}).Code
	}
	if verify(s.Token) != 200 || verify(next.Token) != 200 || verify(s.Token+"x") != 403 {
		t.Fatal("token validation")
	}
	d.verifier.Now = func() time.Time { return time.Now().Add(61 * time.Second) }
	if verify(s.Token) != 403 {
		t.Fatal("expired token accepted")
	}
	d.verifier.Now = nil
	for _, bad := range []string{`{"installationId":"other-store"}`, `{"installationId":"demo-installation","merchantId":"other"}`} {
		if w := request(host, "POST", parentOrigin+"/demo/session", bad, headers); w.Code < 400 {
			t.Fatal("identity override accepted")
		}
	}
	if w := request(host, "POST", parentOrigin+"/demo/revoke", "", map[string]string{"Origin": "https://evil.example", "X-Demo-CSRF": d.csrf}); w.Code != 403 {
		t.Fatal("foreign origin accepted")
	}
	if w := request(host, "POST", parentOrigin+"/demo/revoke", "", map[string]string{"Origin": parentOrigin}); w.Code != 403 {
		t.Fatal("missing CSRF accepted")
	}
	if verify(s.Token) != 200 {
		t.Fatal("denied mutation changed state")
	}
	for i := 0; i < 2; i++ {
		if w := request(host, "POST", parentOrigin+"/demo/revoke", "", headers); w.Code != 200 {
			t.Fatal("idempotent revoke failed")
		}
	}
	if verify(s.Token) != 403 || verify(next.Token) != 403 {
		t.Fatal("old token survives revoke")
	}
	if w := request(host, "POST", parentOrigin+"/demo/session", body, headers); w.Code != 403 {
		t.Fatal("new token issued after revoke")
	}
	if w := request(frame, "GET", appOrigin+"/demo/identity?token="+s.Token, "", nil); w.Code != 403 {
		t.Fatal("token URL accepted")
	}
}

func TestDemoIsolationAndHeaders(t *testing.T) {
	d, _ := newDemo()
	host, frame := d.handlers()
	for _, tc := range []struct {
		h    http.Handler
		url  string
		want string
	}{{host, parentOrigin + "/", "frame-ancestors 'none'"}, {frame, appOrigin + "/", "frame-ancestors " + parentOrigin}} {
		w := request(tc.h, "GET", tc.url, "", nil)
		if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Header().Get("Content-Security-Policy"), tc.want) {
			t.Fatal("missing isolation headers")
		}
		if w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal("unexpected CORS")
		}
	}
	if w := request(host, "GET", "http://attacker.example:4320/demo/config", "", nil); w.Code != 403 {
		t.Fatal("DNS rebinding host accepted")
	}
	if w := request(host, "POST", parentOrigin+"/demo/session", `{"installationId":"demo-installation"}`, map[string]string{"Origin": parentOrigin, "Content-Type": "application/json"}); w.Code != 403 {
		t.Fatal("no CSRF session issued")
	}
}
