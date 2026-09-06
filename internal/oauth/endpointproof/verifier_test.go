package endpointproof

import (
	"context"
	"crypto/x509"
	"emisell.app/platform/internal/oauth/appclient"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestPublicDestinationPolicy(t *testing.T) {
	for _, s := range []string{"127.0.0.1", "10.1.2.3", "100.100.100.200", "169.254.169.254", "172.31.2.1", "192.168.1.1", "192.0.0.8", "192.0.2.1", "192.88.99.1", "198.18.0.1", "198.51.100.1", "203.0.113.1", "224.0.0.1", "255.255.255.255", "0.0.0.0", "168.63.129.16", "::1", "::ffff:127.0.0.1", "2001:db8::1"} {
		if public(netip.MustParseAddr(s)) {
			t.Fatal("forbidden IP", s)
		}
	}
	if !public(netip.MustParseAddr("93.184.216.34")) {
		t.Fatal("public IPv4 denied")
	}
}
func TestHTTPSPinnedProof(t *testing.T) {
	expected := appclient.Proof{Schema: appclient.ProofSchema, ClientID: "client-test", ReleaseSHA256: strings.Repeat("a", 64), Challenge: "nonce-not-sent-to-server"}
	valid, _ := json.Marshal(expected)
	body := string(valid)
	status := 200
	contentType := "application/json"
	calls := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" || r.URL.RawQuery != "" || strings.Contains(r.RequestURI, expected.Challenge) {
			t.Error("outbound credential/nonce leak")
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Location", "https://127.0.0.1/private")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	v := New()
	v.roots = x509.NewCertPool()
	v.roots.AddCert(srv.Certificate())
	v.lookup = func(_ context.Context, network, host string) ([]netip.Addr, error) {
		if network != "ip4" || host != "example.com" {
			t.Error("unexpected resolver", network, host)
		}
		return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
	}
	dials := 0
	v.dial = func(ctx context.Context, network, target string) (net.Conn, error) {
		dials++
		if network != "tcp4" || target != "93.184.216.34:443" {
			t.Error("not pinned", target)
		}
		return (&net.Dialer{}).DialContext(ctx, "tcp", strings.TrimPrefix(srv.URL, "https://"))
	}
	u := "https://example.com/.well-known/emisell-app-verification/proof_ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	if err := v.Verify(context.Background(), u, expected); err != nil {
		t.Fatal("valid TLS proof", err)
	}
	for _, tc := range []struct {
		name, body string
		status     int
		ct         string
	}{
		{"wrong_nonce", strings.ReplaceAll(string(valid), expected.Challenge, "wrong"), 200, "application/json"},
		{"redirect", string(valid), 302, "application/json"}, {"html", string(valid), 200, "text/html"},
		{"large", strings.Repeat(" ", 4097), 200, "application/json"},
		{"duplicate", strings.Replace(string(valid), "{", `{"schema":"bad",`, 1), 200, "application/json"},
		{"trailing", string(valid) + "{}", 200, "application/json"}, {"unknown", strings.Replace(string(valid), "{", `{"extra":"x",`, 1), 200, "application/json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, status, contentType = tc.body, tc.status, tc.ct
			before := calls
			if v.Verify(context.Background(), u, expected) == nil {
				t.Fatal("bad proof accepted")
			}
			if calls != before+1 {
				t.Fatal("redirect/retry attempted")
			}
		})
	}
	body, status, contentType = string(valid), 200, "application/json"
	v.roots = nil
	if v.Verify(context.Background(), u, expected) == nil {
		t.Fatal("untrusted TLS accepted")
	}
	before := dials
	v.lookup = func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("93.184.216.34"), netip.MustParseAddr("127.0.0.1")}, nil
	}
	if v.Verify(context.Background(), u, expected) == nil || dials != before {
		t.Fatal("mixed DNS private answer dialed")
	}
	for _, bad := range []string{"http://example.com/", "https://127.0.0.1/", "https://example.com:8443/", "https://example.com/oauth/callback", u + "?secret=x", u + "#fragment", "https://user:pass@example.com/"} {
		if v.Verify(context.Background(), bad, expected) == nil {
			t.Fatal("bad URL", bad)
		}
	}
}
func TestVerifierBoundedConcurrencyAndDeadline(t *testing.T) {
	v := New()
	for i := 0; i < 4; i++ {
		v.slots <- struct{}{}
	}
	if v.Verify(context.Background(), "", appclient.Proof{}) == nil {
		t.Fatal("concurrency gate")
	}
	for i := 0; i < 4; i++ {
		<-v.slots
	}
	v.lookup = func(ctx context.Context, _, _ string) ([]netip.Addr, error) { <-ctx.Done(); return nil, ctx.Err() }
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if v.Verify(ctx, "https://example.com/.well-known/emisell-app-verification/proof_ABCDEFGHIJKLMNOPQRSTUVWXYZ", appclient.Proof{}) == nil {
		t.Fatal("timeout ignored")
	}
}
