package endpointproof

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"emisell.app/platform/internal/oauth/appclient"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestDevelopmentPinnedProof(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	template := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{"app.emisell.test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	expected := appclient.Proof{Schema: appclient.ProofSchema, ClientID: "test", Challenge: "nonce", ReleaseSHA256: strings.Repeat("a", 64)}
	status := 200
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", "https://app.emisell.test/private")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(expected)
	}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}
	srv.StartTLS()
	defer srv.Close()
	v, err := NewDevelopment("development", "https://app.emisell.test", certPEM)
	if err != nil {
		t.Fatal(err)
	}
	v.lookup = func(context.Context, string, string) ([]netip.Addr, error) {
		t.Fatal("local mode must not resolve DNS")
		return nil, nil
	}
	calls := 0
	v.dial = func(ctx context.Context, network, target string) (net.Conn, error) {
		calls++
		if network != "tcp4" || target != "127.0.0.1:443" {
			t.Fatal("unbounded destination", target)
		}
		return (&net.Dialer{}).DialContext(ctx, "tcp", srv.Listener.Addr().String())
	}
	path := "/.well-known/emisell-app-verification/proof_ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	if err := v.Verify(context.Background(), "https://app.emisell.test"+path, expected); err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"https://other.test", "https://127.0.0.1", "http://app.emisell.test", "https://app.emisell.test:8443"} {
		before := calls
		if v.Verify(context.Background(), origin+path, expected) == nil || calls != before {
			t.Fatal("foreign origin dialed", origin)
		}
	}
	status = 302
	if v.Verify(context.Background(), "https://app.emisell.test"+path, expected) == nil {
		t.Fatal("redirect accepted")
	}
	cert, _ := x509.ParseCertificate(der)
	if v.localTLS(tls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}}) != nil {
		t.Fatal("pin rejected")
	}
	if v.localTLS(tls.ConnectionState{PeerCertificates: []*x509.Certificate{{Raw: []byte("different")}}}) == nil {
		t.Fatal("wrong pin accepted")
	}
	for _, origin := range []string{"https://app.emisell.com", "http://app.emisell.test", "https://app.emisell.test/", "https://app.emisell.test:443", "https://app.emisell.test#", "https://other.test"} {
		if _, err := NewDevelopment("development", origin, certPEM); err == nil {
			t.Fatal("invalid config", origin)
		}
	}
	for _, env := range []string{"", "production", "test"} {
		if _, err := NewDevelopment(env, "https://app.emisell.test", certPEM); err == nil {
			t.Fatal("invalid environment")
		}
	}
	if _, err := NewDevelopment("development", "https://app.emisell.test", []byte("bad")); err == nil {
		t.Fatal("bad certificate")
	}
}
