package resourceclient

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProductionPrivateNetwork(t *testing.T) {
	for _, value := range []string{"127.0.0.1", "::1", "10.0.0.2", "172.19.0.5", "192.168.0.50", "fd00::1"} {
		if !privateResourceIP(net.ParseIP(value)) {
			t.Fatal("private peer rejected", value)
		}
	}
	for _, value := range []string{"8.8.8.8", "169.254.169.254", "fe80::1", "0.0.0.0", "::", "224.0.0.1", "100.100.100.200", ""} {
		if privateResourceIP(net.ParseIP(value)) {
			t.Fatal("unsafe peer accepted", value)
		}
	}
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	var calls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		parts := strings.Split(token, ".")
		if len(parts) != 3 {
			t.Fatal("missing assertion")
		}
		raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
		var claims map[string]any
		_ = json.Unmarshal(raw, &claims)
		if claims["environment"] != "production" || claims["merchant_id"] != "merchant-a" {
			t.Error("incorrect assertion context")
		}
		w.Header().Set("X-Emisell-App-Access", "resource-v1")
		io.WriteString(w, `{"data":[],"meta":{"nextCursor":null}}`)
	}))
	defer upstream.Close()
	options := Options{Origin: upstream.URL, Environment: "production", KeyID: "test-production", PrivateNetworkHTTP: true,
		PrivateKeyPEM: pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})}
	client, err := NewProducts(options)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.read(context.Background(), delegation{MerchantID: "merchant-a", InstallationID: "ins-a", AppID: "app-a", Environment: "production"}, "", nil, "test-private-network-123"); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("internal read not performed")
	}
	options.PrivateNetworkHTTP = false
	if _, err := NewProducts(options); err == nil {
		t.Fatal("implicit production HTTP accepted")
	}
	options.PrivateNetworkHTTP = true
	for _, value := range []string{"http://8.8.8.8:8000", "http://169.254.169.254", "http://[fe80::1]", "https://example.test"} {
		options.Origin = value
		if _, err := NewProducts(options); err == nil {
			t.Fatal("invalid internal origin accepted", value)
		}
	}
	if _, err := privateResourceDialer("127.0.0.1")(context.Background(), "tcp", "8.8.8.8:80"); err == nil {
		t.Fatal("unexpected dial host accepted")
	}
	if _, err := privateResourceDialer("8.8.8.8")(context.Background(), "tcp", "8.8.8.8:80"); err == nil {
		t.Fatal("public numeric peer accepted")
	}
}
