package resourceclient

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExistingProductEndpointRejectsLegacySuccessAndNeverFallsBack(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"", "product-a"} {
		for _, mode := range []string{"app", "legacy", "wrong-ack", "duplicate", "redirect", "not-found"} {
			t.Run(mode+"/"+id, func(t *testing.T) {
				path := "/v1/products"
				if id != "" {
					path += "/" + id
				}
				calls := 0
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					if r.Method != http.MethodGet || r.URL.Path != path || r.Header.Get("X-Emisell-App-Access") != "resource-v1" || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
						t.Errorf("Wrong app endpoint/auth contract: %s", r.URL.Path)
					}
					if r.Header.Get("Cookie") != "" || r.Header.Get("Origin") != "" {
						t.Error("Browser credentials forwarded")
					}
					switch mode {
					case "app":
						w.Header().Set("X-Emisell-App-Access", "resource-v1")
					case "wrong-ack":
						w.Header().Set("X-Emisell-App-Access", "resource-v2")
					case "duplicate":
						w.Header().Add("X-Emisell-App-Access", "resource-v1")
						w.Header().Add("X-Emisell-App-Access", "resource-v1")
					case "redirect":
						w.Header().Set("Location", "/internal/app-platform/v1/products")
						w.WriteHeader(302)
						return
					case "not-found":
						w.WriteHeader(404)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					if id == "" {
						_ = json.NewEncoder(w).Encode(Page{Data: []Product{}, Meta: PageMeta{}})
					} else {
						_ = json.NewEncoder(w).Encode(Item{Data: Product{ID: id, Name: "Fixture", Slug: "fixture", Price: "1000", Status: "ACTIVE", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}})
					}
				}))
				defer upstream.Close()
				client, err := NewProducts(Options{Origin: upstream.URL, Environment: "sandbox", KeyID: "test", AllowHTTP: true, PrivateKeyPEM: pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})})
				if err != nil {
					t.Fatal(err)
				}
				_, err = client.read(context.Background(), delegation{MerchantID: "merchant-a", InstallationID: "installation-a", AppID: "app-a", Environment: "sandbox"}, id, nil, "request-products-123456")
				if (mode == "app") != (err == nil) {
					t.Fatalf("mode %s error %v", mode, err)
				}
				if calls != 1 {
					t.Fatalf("Unexpected retry/fallback: %d", calls)
				}
			})
		}
	}
}
