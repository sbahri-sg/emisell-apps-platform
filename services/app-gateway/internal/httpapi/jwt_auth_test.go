package httpapi_test

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http/httptest"
	"testing"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/httpapi"
)

func TestJWTAuthenticatorValidatesTrustedClaims(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	encodedPublicKey, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: encodedPublicKey})
	authenticator, err := httpapi.NewJWTAuthenticator("https://identity.emisell.test", "emisell-app-platform", "test-key-1", publicKeyPEM, 30*time.Second)
	if err != nil {
		t.Fatalf("create authenticator: %v", err)
	}
	claims := map[string]any{
		"iss": "https://identity.emisell.test", "aud": []string{"another-service", "emisell-app-platform"},
		"sub": testUserID, "organization_id": testOrganizationID, "role": string(domain.RoleAdmin),
		"iat": time.Now().Add(-time.Minute).Unix(), "exp": time.Now().Add(5 * time.Minute).Unix(), "email": "owner@example.test",
		"name": "Emisell Operator", "platform_operator": true,
	}
	token := signJWT(t, privateKey, "test-key-1", claims)
	request := httptest.NewRequest("GET", "/v1/apps", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Organization-Id", testOrganizationID)
	request.Header.Set("X-Emisell-Role", string(domain.RoleOwner))
	actor, err := authenticator.Authenticate(request)
	if err != nil {
		t.Fatalf("authenticate valid token: %v", err)
	}
	if actor.UserID != testUserID || actor.OrganizationID != testOrganizationID || actor.Role != domain.RoleAdmin || actor.Email != "owner@example.test" || !actor.PlatformOperator {
		t.Fatalf("actor used untrusted request headers: %#v", actor)
	}
	mismatchedOrganization := httptest.NewRequest("GET", "/v1/apps", nil)
	mismatchedOrganization.Header.Set("Authorization", "Bearer "+token)
	mismatchedOrganization.Header.Set("X-Organization-Id", "01995f72-0000-7000-8000-000000000099")
	if _, err := authenticator.Authenticate(mismatchedOrganization); err == nil {
		t.Fatal("mismatched organization header was accepted")
	}

	expiredClaims := mapsCopy(claims)
	expiredClaims["exp"] = time.Now().Add(-time.Hour).Unix()
	expiredRequest := httptest.NewRequest("GET", "/v1/apps", nil)
	expiredRequest.Header.Set("Authorization", "Bearer "+signJWT(t, privateKey, "test-key-1", expiredClaims))
	expiredRequest.Header.Set("X-Organization-Id", testOrganizationID)
	if _, err := authenticator.Authenticate(expiredRequest); err == nil {
		t.Fatal("expired token was accepted")
	}

	tamperIndex := len(token) / 2
	tamperedByte := byte('A')
	if token[tamperIndex] == tamperedByte {
		tamperedByte = 'B'
	}
	tampered := token[:tamperIndex] + string(tamperedByte) + token[tamperIndex+1:]
	tamperedRequest := httptest.NewRequest("GET", "/v1/apps", nil)
	tamperedRequest.Header.Set("Authorization", "Bearer "+tampered)
	tamperedRequest.Header.Set("X-Organization-Id", testOrganizationID)
	if _, err := authenticator.Authenticate(tamperedRequest); err == nil {
		t.Fatal("tampered token was accepted")
	}
}

func signJWT(t *testing.T, key *rsa.PrivateKey, keyID string, claims map[string]any) string {
	t.Helper()
	header := map[string]string{"alg": "RS256", "kid": keyID, "typ": "JWT"}
	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)
	unsigned := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign JWT: %v", err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func mapsCopy(source map[string]any) map[string]any {
	copy := make(map[string]any, len(source))
	for key, value := range source {
		copy[key] = value
	}
	return copy
}
