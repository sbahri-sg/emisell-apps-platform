package application

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
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"emisell-app-platform/services/app-gateway/internal/adapters/memory"
	"emisell-app-platform/services/app-gateway/internal/security"
)

func TestOIDCAuthorizationCodePKCEFlow(t *testing.T) {
	now := time.Date(2026, 8, 31, 9, 0, 0, 0, time.UTC)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})
	var nonce, challenge string
	tokenServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := request.ParseForm(); err != nil {
			t.Fatal(err)
		}
		verifier := request.Form.Get("code_verifier")
		verifierDigest := sha256.Sum256([]byte(verifier))
		if base64.RawURLEncoding.EncodeToString(verifierDigest[:]) != challenge || request.Form.Get("code") != "valid-code" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		claims := map[string]any{
			"iss": "https://identity.example.test", "sub": "provider-user-42", "aud": "emisell-client",
			"exp": now.Add(time.Hour).Unix(), "iat": now.Unix(), "nonce": nonce,
			"email": "developer@example.test", "email_verified": true, "name": "OIDC Developer",
		}
		encodedHeader := jwtTestPart(t, map[string]any{"alg": "RS256", "kid": "test-key", "typ": "JWT"})
		encodedClaims := jwtTestPart(t, claims)
		digest := sha256.Sum256([]byte(encodedHeader + "." + encodedClaims))
		signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
		if err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(writer).Encode(map[string]string{"id_token": encodedHeader + "." + encodedClaims + "." + base64.RawURLEncoding.EncodeToString(signature)})
	}))
	defer tokenServer.Close()

	sequence := 0
	id := func() (string, error) {
		sequence++
		return fmt.Sprintf("01995f72-0000-7000-8000-%012d", sequence), nil
	}
	repository := memory.NewRepository(id, func() time.Time { return now })
	identity := NewIdentityService(repository, id, func() time.Time { return now }, 12*time.Hour, 2*time.Hour)
	secretBox, err := security.NewSecretBox(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	oidc, err := NewOIDCService(repository, identity, secretBox, id, func() time.Time { return now }, OIDCOptions{
		Issuer: "https://identity.example.test", AuthorizationEndpoint: "https://identity.example.test/authorize",
		TokenEndpoint: tokenServer.URL, ClientID: "emisell-client", ClientSecret: "server-secret",
		RedirectURL: "https://gateway.example.test/auth/callback", KeyID: "test-key", PublicKeyPEM: publicPEM,
		Scopes: []string{"openid", "profile", "email"}, ClockSkew: 30 * time.Second, LoginTTL: 5 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	authorizeLocation, err := oidc.StartLogin(context.Background(), "//evil.example.test")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(authorizeLocation)
	if err != nil {
		t.Fatal(err)
	}
	state := parsed.Query().Get("state")
	nonce = parsed.Query().Get("nonce")
	challenge = parsed.Query().Get("code_challenge")
	if state == "" || nonce == "" || challenge == "" || parsed.Query().Get("code_challenge_method") != "S256" {
		t.Fatalf("missing OIDC protections in %s", authorizeLocation)
	}
	created, returnTo, err := oidc.CompleteLogin(context.Background(), state, "valid-code")
	if err != nil {
		t.Fatal(err)
	}
	if returnTo != "/overview" || created.Session.Email != "developer@example.test" || created.SessionToken == "" {
		t.Fatalf("unexpected completed session: returnTo=%q session=%+v", returnTo, created.Session)
	}
	if _, _, err := oidc.CompleteLogin(context.Background(), state, "valid-code"); err == nil {
		t.Fatal("OIDC state must be single-use")
	}
}

func jwtTestPart(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(encoded)
}
