package httpapi

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
)

type EmisellBackendAuthenticator interface {
	AuthenticateEmisellBackend(request *http.Request) (application.EmisellBackendPrincipal, error)
}

type DevelopmentEmisellBackendAuthenticator struct {
	BearerToken string
}

func (a DevelopmentEmisellBackendAuthenticator) AuthenticateEmisellBackend(request *http.Request) (application.EmisellBackendPrincipal, error) {
	token, ok := bearerToken(request)
	if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(a.BearerToken)) != 1 {
		return application.EmisellBackendPrincipal{}, domain.ErrUnauthorized
	}
	var merchantDomain *string
	if value := strings.TrimSpace(request.Header.Get("X-Emisell-Store-Domain")); value != "" {
		merchantDomain = &value
	}
	return application.EmisellBackendPrincipal{
		Subject:      strings.TrimSpace(request.Header.Get("X-Emisell-Subject")),
		JTI:          strings.TrimSpace(request.Header.Get("X-Emisell-Token-Id")),
		Email:        strings.TrimSpace(request.Header.Get("X-Emisell-Email")),
		DisplayName:  strings.TrimSpace(request.Header.Get("X-Emisell-Display-Name")),
		MerchantID:   strings.TrimSpace(request.Header.Get("X-Emisell-Store-Id")),
		MerchantName: strings.TrimSpace(request.Header.Get("X-Emisell-Store-Name")),
		Domain:       merchantDomain,
		Environment:  domain.Environment(strings.TrimSpace(request.Header.Get("X-Emisell-Environment"))),
		Permissions:  splitHeader(request.Header.Get("X-Emisell-Permissions")),
	}, nil
}

type JWTEmisellBackendAuthenticator struct {
	issuer    string
	audience  string
	keyID     string
	publicKey *rsa.PublicKey
	clockSkew time.Duration
	now       func() time.Time
}

type emisellBackendClaims struct {
	Issuer         string             `json:"iss"`
	Subject        string             `json:"sub"`
	Audience       json.RawMessage    `json:"aud"`
	ExpiresAt      int64              `json:"exp"`
	NotBefore      int64              `json:"nbf"`
	IssuedAt       int64              `json:"iat"`
	JTI            string             `json:"jti"`
	Email          string             `json:"email"`
	DisplayName    string             `json:"name"`
	MerchantID     string             `json:"store_id"`
	MerchantName   string             `json:"store_name"`
	MerchantDomain *string            `json:"store_domain"`
	Environment    domain.Environment `json:"environment"`
	Permissions    []string           `json:"permissions"`
}

func NewJWTEmisellBackendAuthenticator(issuer, audience, keyID string, publicKeyPEM []byte, clockSkew time.Duration) (*JWTEmisellBackendAuthenticator, error) {
	block, _ := pem.Decode(publicKeyPEM)
	if block == nil {
		return nil, fmt.Errorf("decode Emisell Backend JWT public key PEM")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		if key, pkcs1Err := x509.ParsePKCS1PublicKey(block.Bytes); pkcs1Err == nil {
			parsed = key
		} else {
			return nil, fmt.Errorf("parse Emisell Backend JWT RSA public key: %w", err)
		}
	}
	publicKey, ok := parsed.(*rsa.PublicKey)
	if !ok || publicKey.N.BitLen() < 2048 {
		return nil, fmt.Errorf("Emisell Backend JWT public key must be RSA with at least 2048 bits")
	}
	if strings.TrimSpace(issuer) == "" || strings.TrimSpace(audience) == "" || strings.TrimSpace(keyID) == "" {
		return nil, fmt.Errorf("Emisell Backend JWT issuer, audience, and key id are required")
	}
	return &JWTEmisellBackendAuthenticator{
		issuer: issuer, audience: audience, keyID: keyID, publicKey: publicKey,
		clockSkew: clockSkew, now: func() time.Time { return time.Now().UTC() },
	}, nil
}

func (a *JWTEmisellBackendAuthenticator) AuthenticateEmisellBackend(request *http.Request) (application.EmisellBackendPrincipal, error) {
	token, ok := bearerToken(request)
	if !ok {
		return application.EmisellBackendPrincipal{}, domain.ErrUnauthorized
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return application.EmisellBackendPrincipal{}, domain.ErrUnauthorized
	}
	var header jwtHeader
	if err := decodeJWTPart(parts[0], &header); err != nil || header.Algorithm != "RS256" || header.KeyID != a.keyID || (header.Type != "" && header.Type != "JWT") {
		return application.EmisellBackendPrincipal{}, domain.ErrUnauthorized
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return application.EmisellBackendPrincipal{}, domain.ErrUnauthorized
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(a.publicKey, crypto.SHA256, digest[:], signature) != nil {
		return application.EmisellBackendPrincipal{}, domain.ErrUnauthorized
	}
	var claims emisellBackendClaims
	if err := decodeJWTPart(parts[1], &claims); err != nil || claims.Issuer != a.issuer || !jwtAudienceContains(claims.Audience, a.audience) {
		return application.EmisellBackendPrincipal{}, domain.ErrUnauthorized
	}
	now := a.now().UTC()
	if claims.ExpiresAt <= 0 || claims.IssuedAt <= 0 || now.After(time.Unix(claims.ExpiresAt, 0).Add(a.clockSkew)) || time.Unix(claims.IssuedAt, 0).After(now.Add(a.clockSkew)) {
		return application.EmisellBackendPrincipal{}, domain.ErrUnauthorized
	}
	if claims.NotBefore > 0 && now.Add(a.clockSkew).Before(time.Unix(claims.NotBefore, 0)) {
		return application.EmisellBackendPrincipal{}, domain.ErrUnauthorized
	}
	if time.Unix(claims.ExpiresAt, 0).Sub(time.Unix(claims.IssuedAt, 0)) > 5*time.Minute {
		return application.EmisellBackendPrincipal{}, domain.ErrUnauthorized
	}
	return application.EmisellBackendPrincipal{
		Subject: claims.Subject, JTI: claims.JTI, Email: claims.Email, DisplayName: claims.DisplayName,
		MerchantID: claims.MerchantID, MerchantName: claims.MerchantName, Domain: claims.MerchantDomain,
		Environment: claims.Environment, Permissions: claims.Permissions,
	}, nil
}

func bearerToken(request *http.Request) (string, bool) {
	authorization := strings.TrimSpace(request.Header.Get("Authorization"))
	if !strings.HasPrefix(authorization, "Bearer ") {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
	return token, token != ""
}

func splitHeader(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}
