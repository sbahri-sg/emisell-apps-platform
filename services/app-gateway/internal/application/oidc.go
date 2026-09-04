package application

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/ports"
	"emisell-app-platform/services/app-gateway/internal/security"
)

type OIDCOptions struct {
	Issuer                string
	AuthorizationEndpoint string
	TokenEndpoint         string
	ClientID              string
	ClientSecret          string
	RedirectURL           string
	KeyID                 string
	PublicKeyPEM          []byte
	Scopes                []string
	ClockSkew             time.Duration
	LoginTTL              time.Duration
}

type OIDCService struct {
	repository ports.IdentityRepository
	identity   *IdentityService
	secretBox  *security.SecretBox
	id         ids.Generator
	now        func() time.Time
	options    OIDCOptions
	publicKey  *rsa.PublicKey
	httpClient *http.Client
}

type oidcTokenResponse struct {
	IDToken string `json:"id_token"`
}

type oidcIDTokenClaims struct {
	Issuer           string          `json:"iss"`
	Subject          string          `json:"sub"`
	Audience         json.RawMessage `json:"aud"`
	ExpiresAt        int64           `json:"exp"`
	NotBefore        int64           `json:"nbf"`
	IssuedAt         int64           `json:"iat"`
	Nonce            string          `json:"nonce"`
	Email            string          `json:"email"`
	EmailVerified    bool            `json:"email_verified"`
	DisplayName      string          `json:"name"`
	PlatformOperator bool            `json:"platform_operator"`
}

func NewOIDCService(repository ports.IdentityRepository, identity *IdentityService, secretBox *security.SecretBox, id ids.Generator, now func() time.Time, options OIDCOptions) (*OIDCService, error) {
	publicKey, err := parseOIDCRSAPublicKey(options.PublicKeyPEM)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(options.Issuer) == "" || strings.TrimSpace(options.ClientID) == "" || strings.TrimSpace(options.KeyID) == "" {
		return nil, fmt.Errorf("OIDC issuer, client id, and key id are required")
	}
	for name, endpoint := range map[string]string{"authorization endpoint": options.AuthorizationEndpoint, "token endpoint": options.TokenEndpoint, "redirect URL": options.RedirectURL} {
		parsed, err := url.Parse(endpoint)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return nil, fmt.Errorf("invalid OIDC %s", name)
		}
	}
	if len(options.Scopes) == 0 {
		options.Scopes = []string{"openid", "profile", "email"}
	}
	return &OIDCService{
		repository: repository, identity: identity, secretBox: secretBox, id: id, now: now,
		options: options, publicKey: publicKey,
		httpClient: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }},
	}, nil
}

func (s *OIDCService) StartLogin(ctx context.Context, returnTo string) (string, error) {
	returnTo = validOIDCReturnTo(returnTo)
	stateToken, err := randomIdentityToken()
	if err != nil {
		return "", fmt.Errorf("generate OIDC state: %w", err)
	}
	nonce, err := randomIdentityToken()
	if err != nil {
		return "", fmt.Errorf("generate OIDC nonce: %w", err)
	}
	verifier, err := randomIdentityToken()
	if err != nil {
		return "", fmt.Errorf("generate PKCE verifier: %w", err)
	}
	ciphertext, err := s.secretBox.Encrypt([]byte(verifier))
	if err != nil {
		return "", err
	}
	stateID, err := s.id()
	if err != nil {
		return "", fmt.Errorf("generate OIDC state id: %w", err)
	}
	now := s.now().UTC()
	if err := s.repository.CreateOIDCLoginState(ctx, domain.OIDCLoginState{
		ID: stateID, StateHash: IdentityTokenDigest(stateToken), NonceHash: IdentityTokenDigest(nonce),
		CodeVerifierCiphertext: ciphertext, ReturnTo: returnTo, CreatedAt: now, ExpiresAt: now.Add(s.options.LoginTTL),
	}); err != nil {
		return "", err
	}
	authorizeURL, _ := url.Parse(s.options.AuthorizationEndpoint)
	query := authorizeURL.Query()
	query.Set("response_type", "code")
	query.Set("client_id", s.options.ClientID)
	query.Set("redirect_uri", s.options.RedirectURL)
	query.Set("scope", strings.Join(s.options.Scopes, " "))
	query.Set("state", stateToken)
	query.Set("nonce", nonce)
	challenge := sha256.Sum256([]byte(verifier))
	query.Set("code_challenge", base64.RawURLEncoding.EncodeToString(challenge[:]))
	query.Set("code_challenge_method", "S256")
	authorizeURL.RawQuery = query.Encode()
	return authorizeURL.String(), nil
}

func (s *OIDCService) CompleteLogin(ctx context.Context, stateToken, code string) (CreatedIdentitySession, string, error) {
	if strings.TrimSpace(stateToken) == "" || strings.TrimSpace(code) == "" {
		return CreatedIdentitySession{}, "", domain.ErrInvalidGrant
	}
	state, err := s.repository.ConsumeOIDCLoginState(ctx, IdentityTokenDigest(stateToken), s.now().UTC())
	if err != nil {
		return CreatedIdentitySession{}, "", err
	}
	verifier, err := s.secretBox.Decrypt(state.CodeVerifierCiphertext)
	if err != nil {
		return CreatedIdentitySession{}, "", domain.ErrInvalidGrant
	}
	idToken, err := s.exchangeCode(ctx, code, string(verifier))
	if err != nil {
		return CreatedIdentitySession{}, "", err
	}
	claims, err := s.verifyIDToken(idToken, state.NonceHash)
	if err != nil {
		return CreatedIdentitySession{}, "", err
	}
	newUserID, err := s.id()
	if err != nil {
		return CreatedIdentitySession{}, "", fmt.Errorf("generate user id: %w", err)
	}
	userID, err := s.repository.ProvisionOIDCUser(ctx, s.options.Issuer, claims.Subject, claims.Email, claims.DisplayName, newUserID, s.now().UTC())
	if err != nil {
		return CreatedIdentitySession{}, "", err
	}
	created, err := s.identity.CreateSession(ctx, userID, claims.Email, claims.DisplayName, claims.PlatformOperator, "")
	return created, state.ReturnTo, err
}

func (s *OIDCService) exchangeCode(ctx context.Context, code, verifier string) (string, error) {
	values := url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {s.options.RedirectURL},
		"client_id": {s.options.ClientID}, "code_verifier": {verifier},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.options.TokenEndpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	if s.options.ClientSecret != "" {
		request.SetBasicAuth(s.options.ClientID, s.options.ClientSecret)
	}
	response, err := s.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("exchange OIDC code: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		return "", domain.ErrInvalidGrant
	}
	var tokenResponse oidcTokenResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(&tokenResponse); err != nil || tokenResponse.IDToken == "" {
		return "", domain.ErrInvalidGrant
	}
	return tokenResponse.IDToken, nil
}

func (s *OIDCService) verifyIDToken(token, nonceHash string) (oidcIDTokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return oidcIDTokenClaims{}, domain.ErrInvalidGrant
	}
	var header struct {
		Algorithm string `json:"alg"`
		KeyID     string `json:"kid"`
		Type      string `json:"typ"`
	}
	if err := decodeOIDCJWTPart(parts[0], &header); err != nil || header.Algorithm != "RS256" || header.KeyID != s.options.KeyID || (header.Type != "" && header.Type != "JWT") {
		return oidcIDTokenClaims{}, domain.ErrInvalidGrant
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return oidcIDTokenClaims{}, domain.ErrInvalidGrant
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(s.publicKey, crypto.SHA256, digest[:], signature) != nil {
		return oidcIDTokenClaims{}, domain.ErrInvalidGrant
	}
	var claims oidcIDTokenClaims
	if err := decodeOIDCJWTPart(parts[1], &claims); err != nil {
		return oidcIDTokenClaims{}, domain.ErrInvalidGrant
	}
	now := s.now().UTC()
	if claims.Issuer != s.options.Issuer || !oidcAudienceContains(claims.Audience, s.options.ClientID) || claims.Subject == "" || !claims.EmailVerified || strings.TrimSpace(claims.Email) == "" {
		return oidcIDTokenClaims{}, domain.ErrInvalidGrant
	}
	if claims.ExpiresAt <= 0 || now.After(time.Unix(claims.ExpiresAt, 0).Add(s.options.ClockSkew)) || (claims.NotBefore > 0 && now.Add(s.options.ClockSkew).Before(time.Unix(claims.NotBefore, 0))) || (claims.IssuedAt > 0 && time.Unix(claims.IssuedAt, 0).After(now.Add(s.options.ClockSkew))) {
		return oidcIDTokenClaims{}, domain.ErrInvalidGrant
	}
	if subtle.ConstantTimeCompare([]byte(IdentityTokenDigest(claims.Nonce)), []byte(nonceHash)) != 1 {
		return oidcIDTokenClaims{}, domain.ErrInvalidGrant
	}
	claims.Email = strings.ToLower(strings.TrimSpace(claims.Email))
	claims.DisplayName = strings.TrimSpace(claims.DisplayName)
	if claims.DisplayName == "" {
		claims.DisplayName = claims.Email
	}
	return claims, nil
}

func validOIDCReturnTo(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") || strings.ContainsAny(value, "\r\n") || len(value) > 500 {
		return "/overview"
	}
	return value
}

func decodeOIDCJWTPart(value string, destination any) error {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(decoded, destination)
}

func oidcAudienceContains(raw json.RawMessage, expected string) bool {
	var single string
	if json.Unmarshal(raw, &single) == nil {
		return single == expected
	}
	var multiple []string
	if json.Unmarshal(raw, &multiple) != nil {
		return false
	}
	for _, value := range multiple {
		if value == expected {
			return true
		}
	}
	return false
}

func parseOIDCRSAPublicKey(publicKeyPEM []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(publicKeyPEM)
	if block == nil {
		return nil, fmt.Errorf("decode OIDC public key PEM")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		if key, pkcs1Err := x509.ParsePKCS1PublicKey(block.Bytes); pkcs1Err == nil {
			parsed = key
		} else {
			return nil, fmt.Errorf("parse OIDC RSA public key: %w", err)
		}
	}
	publicKey, ok := parsed.(*rsa.PublicKey)
	if !ok || publicKey.N.BitLen() < 2048 {
		return nil, fmt.Errorf("OIDC public key must be RSA with at least 2048 bits")
	}
	return publicKey, nil
}
