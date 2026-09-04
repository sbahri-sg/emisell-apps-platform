package emisell

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
)

func productTestKey(t *testing.T) (*rsa.PrivateKey, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
}
func productTestAccess() domain.InstallationAccessContext {
	return domain.InstallationAccessContext{AppID: "app-a", InstallationID: "install-a", MerchantID: "merchant-a", Environment: domain.EnvironmentSandbox, InstallationStatus: domain.InstallationStatusActive, Scopes: []string{ReadProducts, "read_orders"}, TokenExpiresAt: time.Now().Add(time.Hour)}
}

func TestProductAssertionAndProjection(t *testing.T) {
	key, pemKey := productTestKey(t)
	var lastJTI string
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/app-platform/v1/products/product-a" || r.Header.Get("Cookie") != "" || r.Header.Get("X-Emisell-Merchant-ID") != "merchant-a" || r.Header.Get("X-Emisell-Installation-ID") != "install-a" {
			t.Error("unsafe routing")
		}
		parts := strings.Split(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), ".")
		if len(parts) != 3 {
			t.Fatal("missing assertion")
		}
		digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
		sig, _ := base64.RawURLEncoding.DecodeString(parts[2])
		if rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, digest[:], sig) != nil {
			t.Error("bad signature")
		}
		payload, _ := base64.RawURLEncoding.DecodeString(parts[1])
		var claims map[string]any
		_ = json.Unmarshal(payload, &claims)
		if claims["iss"] != "emisell-app-platform" || claims["aud"] != "emisell-api-service" || claims["merchant_id"] != "merchant-a" || claims["exp"].(float64)-claims["iat"].(float64) > 45 {
			t.Error("bad claims")
		}
		if len(claims["scope"].([]any)) != 1 || claims["scope"].([]any)[0] != ReadProducts {
			t.Error("assertion is over-scoped")
		}
		if claims["jti"] == lastJTI {
			t.Error("assertion reused")
		}
		lastJTI = claims["jti"].(string)
		w.Write([]byte(`{"data":{"id":"product-a","name":"Example","slug":"example","sku":null,"description":null,"price":"19.99","compareAtPrice":null,"stock":2,"isPhysical":true,"isPublished":true,"status":"ACTIVE","type":null,"trackInventory":true,"continueSellingWhenOutOfStock":false,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z","cost":"secret","merchantId":"merchant-a","internal":"hidden"}}`))
	}))
	defer peer.Close()
	client, err := NewProducts(Options{Origin: peer.URL, KeyID: "test", PrivateKeyPEM: pemKey, AllowHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		result, err := client.Read(t.Context(), productTestAccess(), "product-a", nil, "request-test-0000001")
		if err != nil {
			t.Fatal(err)
		}
		body, _ := json.Marshal(result)
		if strings.Contains(string(body), "secret") || strings.Contains(string(body), "merchantId") || strings.Contains(string(body), "internal") {
			t.Fatal("private fields leaked")
		}
	}
}

func TestResourceTransportFailsClosed(t *testing.T) {
	_, pemKey := productTestKey(t)
	for _, tc := range []struct {
		name     string
		status   int
		body     string
		expected int
	}{
		{"backend credentials", 401, "private detail", 503}, {"backend policy", 403, "private detail", 503},
		{"not found", 404, "private detail", 404}, {"bad cursor", 400, "private detail", 400},
		{"rate", 429, "private detail", 429}, {"failure", 500, "SQL secret", 503}, {"redirect", 302, "", 503},
		{"invalid JSON", 200, "no-json", 503}, {"missing fields", 200, `{"data":{"id":"product-a"}}`, 503},
		{"too large", 200, strings.Repeat("x", (4<<20)+1), 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", "http://untrusted.invalid/")
				w.Header().Set("Retry-After", "99999")
				w.WriteHeader(tc.status)
				w.Write([]byte(tc.body))
			}))
			defer peer.Close()
			client, err := NewProducts(Options{Origin: peer.URL, KeyID: "test", PrivateKeyPEM: pemKey, AllowHTTP: true})
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Read(t.Context(), productTestAccess(), "product-a", nil, "request-test-0000001")
			var resourceError *Error
			if !errors.As(err, &resourceError) || resourceError.Status != tc.expected || strings.Contains(err.Error(), "private") {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.status == 429 && resourceError.RetryAfter != 60 {
				t.Fatal("unsafe retry delay")
			}
		})
	}
	for _, origin := range []string{"http://example.com", "https://user:pass@example.com", "https://example.com/admin", "https://example.com?override=x", "file:///tmp/test"} {
		if _, err := NewProducts(Options{Origin: origin, KeyID: "test", PrivateKeyPEM: pemKey}); err == nil {
			t.Errorf("accepted invalid origin: %s", origin)
		}
	}
}

func TestProductsValidationAndCancellation(t *testing.T) {
	_, pemKey := productTestKey(t)
	calls := 0
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; <-r.Context().Done() }))
	defer peer.Close()
	client, err := NewProducts(Options{Origin: peer.URL, KeyID: "test", PrivateKeyPEM: pemKey, AllowHTTP: true, Timeout: 30 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"merchantId=other", "limit=0", "limit=101", "limit=01", "limit=2&limit=3", "published=no", "updatedAfter=invalid", "updatedAfter=2026-02-30T00:00:00Z", "cursor="} {
		query, _ := url.ParseQuery(raw)
		if ValidateQuery(query, false) == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	access := productTestAccess()
	access.Scopes = []string{"read_merchant"}
	if _, err := client.Read(t.Context(), access, "", nil, "request-test-0000001"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal("missing scope accepted")
	}
	access = productTestAccess()
	access.TokenExpiresAt = time.Now().Add(-time.Minute)
	if _, err := client.Read(t.Context(), access, "", nil, "request-test-0000001"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatal("expired token accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := client.Read(ctx, productTestAccess(), "", nil, "request-test-0000001"); err == nil {
		t.Fatal("cancel ignored")
	}
	if calls != 0 {
		t.Fatal("unauthorized request reached backend")
	}
	if _, err := client.Read(t.Context(), productTestAccess(), "", nil, "request-test-0000001"); err == nil {
		t.Fatal("timeout ignored")
	}
}
