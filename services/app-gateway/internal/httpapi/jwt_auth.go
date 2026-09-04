package httpapi

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
)

type JWTAuthenticator struct {
	issuer    string
	audience  string
	keyID     string
	publicKey *rsa.PublicKey
	clockSkew time.Duration
	now       func() time.Time
}

type jwtHeader struct {
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
	Type      string `json:"typ"`
}

type jwtClaims struct {
	Issuer           string          `json:"iss"`
	Subject          string          `json:"sub"`
	Audience         json.RawMessage `json:"aud"`
	ExpiresAt        int64           `json:"exp"`
	NotBefore        int64           `json:"nbf"`
	IssuedAt         int64           `json:"iat"`
	OrganizationID   string          `json:"organization_id"`
	Role             domain.Role     `json:"role"`
	Email            string          `json:"email"`
	DisplayName      string          `json:"name"`
	PlatformOperator bool            `json:"platform_operator"`
}

var actorUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

func NewJWTAuthenticator(issuer, audience, keyID string, publicKeyPEM []byte, clockSkew time.Duration) (*JWTAuthenticator, error) {
	block, _ := pem.Decode(publicKeyPEM)
	if block == nil {
		return nil, fmt.Errorf("decode JWT public key PEM")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		if key, pkcs1Err := x509.ParsePKCS1PublicKey(block.Bytes); pkcs1Err == nil {
			parsed = key
		} else {
			return nil, fmt.Errorf("parse JWT RSA public key: %w", err)
		}
	}
	publicKey, ok := parsed.(*rsa.PublicKey)
	if !ok || publicKey.N.BitLen() < 2048 {
		return nil, fmt.Errorf("JWT public key must be RSA with at least 2048 bits")
	}
	if strings.TrimSpace(issuer) == "" || strings.TrimSpace(audience) == "" || strings.TrimSpace(keyID) == "" {
		return nil, fmt.Errorf("JWT issuer, audience, and key id are required")
	}
	return &JWTAuthenticator{
		issuer: issuer, audience: audience, keyID: keyID, publicKey: publicKey,
		clockSkew: clockSkew, now: func() time.Time { return time.Now().UTC() },
	}, nil
}

func (a *JWTAuthenticator) Authenticate(request *http.Request) (Actor, error) {
	authorization := strings.TrimSpace(request.Header.Get("Authorization"))
	if !strings.HasPrefix(authorization, "Bearer ") {
		return Actor{}, fmt.Errorf("missing bearer token")
	}
	token := strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return Actor{}, fmt.Errorf("invalid JWT format")
	}
	var header jwtHeader
	if err := decodeJWTPart(parts[0], &header); err != nil {
		return Actor{}, fmt.Errorf("decode JWT header: %w", err)
	}
	if header.Algorithm != "RS256" || header.KeyID != a.keyID || (header.Type != "" && header.Type != "JWT") {
		return Actor{}, fmt.Errorf("unsupported JWT header")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Actor{}, fmt.Errorf("decode JWT signature")
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(a.publicKey, crypto.SHA256, digest[:], signature); err != nil {
		return Actor{}, fmt.Errorf("verify JWT signature")
	}
	var claims jwtClaims
	if err := decodeJWTPart(parts[1], &claims); err != nil {
		return Actor{}, fmt.Errorf("decode JWT claims: %w", err)
	}
	if claims.Issuer != a.issuer || !jwtAudienceContains(claims.Audience, a.audience) {
		return Actor{}, fmt.Errorf("invalid JWT issuer or audience")
	}
	now := a.now().UTC()
	if claims.ExpiresAt <= 0 || now.After(time.Unix(claims.ExpiresAt, 0).Add(a.clockSkew)) {
		return Actor{}, fmt.Errorf("JWT expired")
	}
	if claims.NotBefore > 0 && now.Add(a.clockSkew).Before(time.Unix(claims.NotBefore, 0)) {
		return Actor{}, fmt.Errorf("JWT not active")
	}
	if claims.IssuedAt > 0 && time.Unix(claims.IssuedAt, 0).After(now.Add(a.clockSkew)) {
		return Actor{}, fmt.Errorf("JWT issued in the future")
	}
	if !actorUUID.MatchString(claims.Subject) || !actorUUID.MatchString(claims.OrganizationID) || !validRole(claims.Role) {
		return Actor{}, fmt.Errorf("invalid JWT actor claims")
	}
	requestedOrganization := strings.TrimSpace(request.Header.Get("X-Organization-Id"))
	if !strings.EqualFold(requestedOrganization, claims.OrganizationID) {
		return Actor{}, fmt.Errorf("organization header does not match JWT")
	}
	return Actor{
		UserID: strings.ToLower(claims.Subject), OrganizationID: strings.ToLower(claims.OrganizationID),
		Role: claims.Role, Email: strings.ToLower(strings.TrimSpace(claims.Email)),
		DisplayName: strings.TrimSpace(claims.DisplayName), PlatformOperator: claims.PlatformOperator,
		AuthenticationMethod: "bearer",
	}, nil
}

func decodeJWTPart(value string, destination any) error {
	encoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, destination)
}

func jwtAudienceContains(raw json.RawMessage, expected string) bool {
	var single string
	if json.Unmarshal(raw, &single) == nil {
		return single == expected
	}
	var multiple []string
	if json.Unmarshal(raw, &multiple) != nil {
		return false
	}
	for _, audience := range multiple {
		if audience == expected {
			return true
		}
	}
	return false
}
