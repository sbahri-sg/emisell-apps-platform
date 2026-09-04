package httpapi_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http/httptest"
	"testing"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/httpapi"
)

func TestJWTEmisellBackendAuthenticatorUsesOnlySignedStoreClaims(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	encodedPublicKey, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: encodedPublicKey})
	authenticator, err := httpapi.NewJWTEmisellBackendAuthenticator(
		"https://api.emisell.test", "emisell-store-bridge", "bridge-key-1", publicKeyPEM, 30*time.Second,
	)
	if err != nil {
		t.Fatalf("create authenticator: %v", err)
	}
	now := time.Now().UTC()
	claims := map[string]any{
		"iss": "https://api.emisell.test", "aud": "emisell-store-bridge",
		"sub": "cmuserowner0000000000001", "jti": "bridge-token-00000001",
		"iat": now.Add(-time.Minute).Unix(), "exp": now.Add(2 * time.Minute).Unix(),
		"email": "owner@example.com", "name": "Store Owner",
		"store_id": "cmmerchantsigned000000001", "store_name": "Signed Store",
		"store_domain": "signed.example.com", "environment": "production",
		"permissions": []string{"apps.install"},
	}
	request := httptest.NewRequest("POST", "/v1/integrations/emisell/merchant-session-grants", nil)
	request.Header.Set("Authorization", "Bearer "+signJWT(t, privateKey, "bridge-key-1", claims))
	request.Header.Set("X-Emisell-Store-Id", "33333333-3333-7333-8333-333333333333")
	principal, err := authenticator.AuthenticateEmisellBackend(request)
	if err != nil {
		t.Fatalf("authenticate signed service token: %v", err)
	}
	if principal.MerchantID != "cmmerchantsigned000000001" || principal.MerchantName != "Signed Store" || principal.Environment != domain.EnvironmentProduction {
		t.Fatalf("principal used untrusted request headers: %#v", principal)
	}

	longLived := mapsCopy(claims)
	longLived["iat"] = now.Unix()
	longLived["exp"] = now.Add(10 * time.Minute).Unix()
	longLivedRequest := httptest.NewRequest("POST", "/v1/integrations/emisell/merchant-session-grants", nil)
	longLivedRequest.Header.Set("Authorization", "Bearer "+signJWT(t, privateKey, "bridge-key-1", longLived))
	if _, err := authenticator.AuthenticateEmisellBackend(longLivedRequest); err == nil {
		t.Fatal("service token with lifetime over five minutes was accepted")
	}

	wrongAudience := mapsCopy(claims)
	wrongAudience["aud"] = "another-service"
	wrongAudienceRequest := httptest.NewRequest("POST", "/v1/integrations/emisell/merchant-session-grants", nil)
	wrongAudienceRequest.Header.Set("Authorization", "Bearer "+signJWT(t, privateKey, "bridge-key-1", wrongAudience))
	if _, err := authenticator.AuthenticateEmisellBackend(wrongAudienceRequest); err == nil {
		t.Fatal("service token with wrong audience was accepted")
	}
}
