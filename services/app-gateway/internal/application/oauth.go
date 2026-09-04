package application

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

type OAuthService struct {
	repository     ports.Repository
	cipher         SecretCipher
	id             ids.Generator
	now            func() time.Time
	codeTTL        time.Duration
	accessTokenTTL time.Duration
	pilot          DevelopmentResourcePilot
}

type AuthorizeOAuthCommand struct {
	OrganizationID       string
	ActorID              string
	ClientID             string
	RedirectURI          string
	State                string
	CodeChallenge        string
	MerchantID           string
	MerchantName         string
	MerchantDomain       *string
	Environment          domain.Environment
	GrantedScopes        []string
	TestInstallRequestID string
}

type OAuthAuthorizationResponse struct {
	Authorization domain.OAuthAuthorization `json:"authorization"`
	Code          string                    `json:"code"`
	RedirectTo    string                    `json:"redirectTo"`
}

type ExchangeOAuthCommand struct {
	GrantType    string
	Code         string
	ClientID     string
	ClientSecret string
	RedirectURI  string
	CodeVerifier string
}

type MerchantOAuthRequest struct {
	ClientID             string
	RedirectURI          string
	State                string
	CodeChallenge        string
	RequestedScopes      []string
	TestInstallRequestID string
}

type MerchantConsentApp struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	AppURL      *string `json:"appUrl"`
}

type MerchantConsentVersion struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type MerchantOAuthConsent struct {
	App                MerchantConsentApp      `json:"app"`
	Version            MerchantConsentVersion  `json:"version"`
	Merchant           domain.MerchantIdentity `json:"merchant"`
	RedirectURI        string                  `json:"redirectUri"`
	RequiredScopes     []domain.SnapshotScope  `json:"requiredScopes"`
	OptionalScopes     []domain.SnapshotScope  `json:"optionalScopes"`
	RequestedScopes    []string                `json:"requestedScopes"`
	DevelopmentInstall bool                    `json:"developmentInstall"`
}

type AuthorizeMerchantOAuthCommand struct {
	Request       MerchantOAuthRequest
	GrantedScopes []string
}

type OAuthTokenResponse struct {
	AccessToken  string                 `json:"access_token"`
	TokenType    string                 `json:"token_type"`
	ExpiresIn    int64                  `json:"expires_in"`
	Scope        string                 `json:"scope"`
	Installation domain.AppInstallation `json:"installation"`
}

var pkceValue = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)

func NewOAuthService(repository ports.Repository, cipher SecretCipher, id ids.Generator, now func() time.Time, codeTTL, accessTokenTTL time.Duration, pilots ...DevelopmentResourcePilot) *OAuthService {
	service := &OAuthService{repository: repository, cipher: cipher, id: id, now: now, codeTTL: codeTTL, accessTokenTTL: accessTokenTTL}
	if len(pilots) > 0 {
		service.pilot = pilots[0]
	}
	return service
}

func (s *OAuthService) Authorize(ctx context.Context, command AuthorizeOAuthCommand) (OAuthAuthorizationResponse, error) {
	client, err := s.repository.GetOAuthClient(ctx, strings.TrimSpace(command.ClientID))
	if err != nil || client.OrganizationID != command.OrganizationID {
		return OAuthAuthorizationResponse{}, fmt.Errorf("%w: OAuth client is unavailable", domain.ErrNotFound)
	}
	now := s.now().UTC()
	if client.Credential.Status != domain.CredentialStatusActive || (client.Credential.ExpiresAt != nil && !client.Credential.ExpiresAt.After(now)) {
		return OAuthAuthorizationResponse{}, fmt.Errorf("%w: OAuth client is not active", domain.ErrConflict)
	}
	if client.Credential.Environment != command.Environment {
		return OAuthAuthorizationResponse{}, fmt.Errorf("%w: credential and installation environments must match", domain.ErrValidation)
	}
	if err := requireEnvironmentAccess(ctx, s.repository, command.OrganizationID, command.Environment); err != nil {
		return OAuthAuthorizationResponse{}, err
	}
	if len(command.State) < 16 || len(command.State) > 512 {
		return OAuthAuthorizationResponse{}, fmt.Errorf("%w: state must contain 16 to 512 characters", domain.ErrValidation)
	}
	if !pkceValue.MatchString(command.CodeChallenge) {
		return OAuthAuthorizationResponse{}, fmt.Errorf("%w: codeChallenge must be a 43 to 128 character PKCE S256 value", domain.ErrValidation)
	}
	app, err := s.repository.GetApp(ctx, command.OrganizationID, client.Credential.AppID)
	if err != nil {
		return OAuthAuthorizationResponse{}, err
	}
	if app.Status != domain.AppStatusActive || app.ActiveVersionID == nil {
		return OAuthAuthorizationResponse{}, fmt.Errorf("%w: app must have an active version before authorization", domain.ErrConflict)
	}
	version, err := s.repository.GetVersion(ctx, command.OrganizationID, app.ID, *app.ActiveVersionID)
	if err != nil {
		return OAuthAuthorizationResponse{}, err
	}
	redirectURI := strings.TrimSpace(command.RedirectURI)
	if !containsExact(version.Snapshot.RedirectURLs, redirectURI) {
		return OAuthAuthorizationResponse{}, fmt.Errorf("%w: redirectUri is not registered in the active app version", domain.ErrValidation)
	}
	merchantName := strings.TrimSpace(command.MerchantName)
	merchantID := strings.TrimSpace(command.MerchantID)
	if !externalIdentityValue.MatchString(merchantID) || len([]rune(merchantName)) < 2 || len([]rune(merchantName)) > 120 {
		return OAuthAuthorizationResponse{}, fmt.Errorf("%w: valid merchantId and merchantName are required", domain.ErrValidation)
	}
	var merchantHost *string
	if command.MerchantDomain != nil && strings.TrimSpace(*command.MerchantDomain) != "" {
		value := strings.ToLower(strings.TrimSpace(*command.MerchantDomain))
		if !merchantDomain.MatchString(value) {
			return OAuthAuthorizationResponse{}, fmt.Errorf("%w: merchantDomain must be a hostname without protocol", domain.ErrValidation)
		}
		merchantHost = &value
	}
	grantedScopes, err := validateInstallationScopes(version.Snapshot.Scopes, command.GrantedScopes)
	if err != nil {
		return OAuthAuthorizationResponse{}, err
	}
	authorizationID, err := s.id()
	if err != nil {
		return OAuthAuthorizationResponse{}, fmt.Errorf("generate authorization id: %w", err)
	}
	code, err := randomToken("es_code_", 32)
	if err != nil {
		return OAuthAuthorizationResponse{}, err
	}
	authorization := domain.OAuthAuthorization{
		ID: authorizationID, OrganizationID: command.OrganizationID, AppID: app.ID,
		CredentialID: client.Credential.ID, ClientID: client.Credential.ClientID, CodeHash: tokenDigest(code),
		RedirectURI: redirectURI, CodeChallenge: command.CodeChallenge, MerchantID: merchantID,
		MerchantName: merchantName, MerchantDomain: merchantHost, Environment: command.Environment,
		InstalledVersionID: version.ID, GrantedScopes: grantedScopes, ApprovedBy: command.ActorID,
		CreatedAt: now, ExpiresAt: now.Add(s.codeTTL),
	}
	if requestID := strings.TrimSpace(command.TestInstallRequestID); requestID != "" {
		authorization.TestInstallRequestID = &requestID
	}
	created, err := s.repository.CreateOAuthAuthorization(ctx, authorization, ports.MutationMeta{ActorID: command.ActorID, Action: "oauth.authorization.approved"})
	if err != nil {
		return OAuthAuthorizationResponse{}, err
	}
	redirect, err := url.Parse(redirectURI)
	if err != nil {
		return OAuthAuthorizationResponse{}, fmt.Errorf("build redirect URI: %w", err)
	}
	query := redirect.Query()
	query.Set("code", code)
	query.Set("state", command.State)
	redirect.RawQuery = query.Encode()
	return OAuthAuthorizationResponse{Authorization: created, Code: code, RedirectTo: redirect.String()}, nil
}

func (s *OAuthService) PreviewMerchantConsent(ctx context.Context, merchant domain.MerchantIdentity, request MerchantOAuthRequest) (MerchantOAuthConsent, error) {
	consent, _, err := s.prepareMerchantConsent(ctx, merchant, request)
	return consent, err
}

func (s *OAuthService) AuthorizeMerchant(ctx context.Context, merchant domain.MerchantIdentity, command AuthorizeMerchantOAuthCommand) (OAuthAuthorizationResponse, error) {
	consent, client, err := s.prepareMerchantConsent(ctx, merchant, command.Request)
	if err != nil {
		return OAuthAuthorizationResponse{}, err
	}
	allowed := make(map[string]domain.ScopeAccess, len(consent.RequiredScopes)+len(consent.OptionalScopes))
	for _, scope := range consent.RequiredScopes {
		allowed[scope.Scope] = domain.ScopeAccessRequired
	}
	for _, scope := range consent.OptionalScopes {
		allowed[scope.Scope] = domain.ScopeAccessOptional
	}
	granted := make([]string, 0, len(command.GrantedScopes))
	seen := make(map[string]struct{}, len(command.GrantedScopes))
	for _, raw := range command.GrantedScopes {
		scope := strings.TrimSpace(raw)
		if _, exists := allowed[scope]; !exists {
			return OAuthAuthorizationResponse{}, fmt.Errorf("%w: scope %q was not requested by the app", domain.ErrValidation, scope)
		}
		if _, duplicate := seen[scope]; duplicate {
			return OAuthAuthorizationResponse{}, fmt.Errorf("%w: duplicate granted scope %q", domain.ErrValidation, scope)
		}
		seen[scope] = struct{}{}
		granted = append(granted, scope)
	}
	for scope, access := range allowed {
		if access == domain.ScopeAccessRequired {
			if _, exists := seen[scope]; !exists {
				return OAuthAuthorizationResponse{}, fmt.Errorf("%w: required scope %q must be granted", domain.ErrValidation, scope)
			}
		}
	}
	sort.Strings(granted)
	return s.Authorize(ctx, AuthorizeOAuthCommand{
		OrganizationID: client.OrganizationID, ActorID: merchant.UserID,
		ClientID: command.Request.ClientID, RedirectURI: command.Request.RedirectURI,
		State: command.Request.State, CodeChallenge: command.Request.CodeChallenge,
		MerchantID: merchant.MerchantID, MerchantName: merchant.Name, MerchantDomain: merchant.Domain,
		Environment: merchant.Environment, GrantedScopes: granted,
		TestInstallRequestID: command.Request.TestInstallRequestID,
	})
}

func (s *OAuthService) prepareMerchantConsent(ctx context.Context, merchant domain.MerchantIdentity, request MerchantOAuthRequest) (MerchantOAuthConsent, domain.OAuthClient, error) {
	if (merchant.Environment != domain.EnvironmentSandbox && merchant.Environment != domain.EnvironmentProduction) || !externalIdentityValue.MatchString(merchant.MerchantID) || !uuidValue.MatchString(merchant.UserID) {
		return MerchantOAuthConsent{}, domain.OAuthClient{}, domain.ErrUnauthorized
	}
	client, err := s.repository.GetOAuthClient(ctx, strings.TrimSpace(request.ClientID))
	if err != nil {
		return MerchantOAuthConsent{}, domain.OAuthClient{}, fmt.Errorf("%w: OAuth client is unavailable", domain.ErrNotFound)
	}
	now := s.now().UTC()
	if client.Credential.Environment != merchant.Environment || client.Credential.Status != domain.CredentialStatusActive || (client.Credential.ExpiresAt != nil && !client.Credential.ExpiresAt.After(now)) {
		return MerchantOAuthConsent{}, domain.OAuthClient{}, fmt.Errorf("%w: OAuth client is not active for the merchant environment", domain.ErrConflict)
	}
	if err := requireEnvironmentAccess(ctx, s.repository, client.OrganizationID, merchant.Environment); err != nil {
		return MerchantOAuthConsent{}, domain.OAuthClient{}, err
	}
	if len(request.State) < 16 || len(request.State) > 512 {
		return MerchantOAuthConsent{}, domain.OAuthClient{}, fmt.Errorf("%w: state must contain 16 to 512 characters", domain.ErrValidation)
	}
	if !pkceValue.MatchString(request.CodeChallenge) {
		return MerchantOAuthConsent{}, domain.OAuthClient{}, fmt.Errorf("%w: codeChallenge must be a 43 to 128 character PKCE S256 value", domain.ErrValidation)
	}
	app, err := s.repository.GetApp(ctx, client.OrganizationID, client.Credential.AppID)
	if err != nil {
		return MerchantOAuthConsent{}, domain.OAuthClient{}, err
	}
	if app.Status != domain.AppStatusActive || app.ActiveVersionID == nil {
		return MerchantOAuthConsent{}, domain.OAuthClient{}, fmt.Errorf("%w: app must have an active version before authorization", domain.ErrConflict)
	}
	version, err := s.repository.GetVersion(ctx, client.OrganizationID, app.ID, *app.ActiveVersionID)
	if err != nil {
		return MerchantOAuthConsent{}, domain.OAuthClient{}, err
	}
	redirectURI := strings.TrimSpace(request.RedirectURI)
	if !containsExact(version.Snapshot.RedirectURLs, redirectURI) {
		return MerchantOAuthConsent{}, domain.OAuthClient{}, fmt.Errorf("%w: redirectUri is not registered in the active app version", domain.ErrValidation)
	}
	developmentInstall := false
	if requestID := strings.TrimSpace(request.TestInstallRequestID); requestID != "" {
		if !uuidValue.MatchString(requestID) {
			return MerchantOAuthConsent{}, domain.OAuthClient{}, fmt.Errorf("%w: invalid development install request", domain.ErrNotFound)
		}
		testRequest, err := s.repository.GetDevelopmentInstallRequest(ctx, client.OrganizationID, app.ID, requestID)
		if err != nil || testRequest.Status != domain.DevelopmentInstallRequestStatusPending ||
			!testRequest.ExpiresAt.After(now) || testRequest.MerchantID != merchant.MerchantID ||
			testRequest.Environment != merchant.Environment || testRequest.VersionID != version.ID {
			return MerchantOAuthConsent{}, domain.OAuthClient{}, fmt.Errorf("%w: development install request is unavailable for this merchant", domain.ErrNotFound)
		}
		developmentInstall = true
		if unavailable := s.pilot.unavailable(version.Snapshot.Scopes, merchant); unavailable != "" {
			return MerchantOAuthConsent{}, domain.OAuthClient{}, fmt.Errorf("%w: development resource pilot is unavailable", domain.ErrConflict)
		}
	}
	defined := make(map[string]domain.SnapshotScope, len(version.Snapshot.Scopes))
	required := make([]domain.SnapshotScope, 0)
	for _, scope := range version.Snapshot.Scopes {
		defined[scope.Scope] = scope
		if domain.ScopeAccess(scope.Access) == domain.ScopeAccessRequired {
			required = append(required, scope)
		}
	}
	requested := make([]string, 0, len(request.RequestedScopes)+len(required))
	seen := make(map[string]struct{}, len(request.RequestedScopes)+len(required))
	for _, scope := range required {
		requested = append(requested, scope.Scope)
		seen[scope.Scope] = struct{}{}
	}
	optional := make([]domain.SnapshotScope, 0)
	for _, raw := range request.RequestedScopes {
		scopeName := strings.TrimSpace(raw)
		scope, exists := defined[scopeName]
		if !exists {
			return MerchantOAuthConsent{}, domain.OAuthClient{}, fmt.Errorf("%w: scope %q is not part of the active version", domain.ErrValidation, scopeName)
		}
		if _, duplicate := seen[scopeName]; duplicate {
			if domain.ScopeAccess(scope.Access) == domain.ScopeAccessRequired {
				continue
			}
			return MerchantOAuthConsent{}, domain.OAuthClient{}, fmt.Errorf("%w: duplicate requested scope %q", domain.ErrValidation, scopeName)
		}
		seen[scopeName] = struct{}{}
		requested = append(requested, scopeName)
		if domain.ScopeAccess(scope.Access) == domain.ScopeAccessOptional {
			optional = append(optional, scope)
		}
	}
	sort.Slice(required, func(i, j int) bool { return required[i].Scope < required[j].Scope })
	sort.Slice(optional, func(i, j int) bool { return optional[i].Scope < optional[j].Scope })
	sort.Strings(requested)
	return MerchantOAuthConsent{
		App:     MerchantConsentApp{ID: app.ID, Name: app.Name, Description: app.Description, AppURL: app.AppURL},
		Version: MerchantConsentVersion{ID: version.ID, Version: version.Version}, Merchant: merchant,
		RedirectURI: redirectURI, RequiredScopes: required, OptionalScopes: optional, RequestedScopes: requested,
		DevelopmentInstall: developmentInstall,
	}, client, nil
}

func (s *OAuthService) Exchange(ctx context.Context, command ExchangeOAuthCommand) (OAuthTokenResponse, error) {
	if command.GrantType != "authorization_code" {
		return OAuthTokenResponse{}, fmt.Errorf("%w: grant_type must be authorization_code", domain.ErrInvalidGrant)
	}
	client, err := s.repository.GetOAuthClient(ctx, strings.TrimSpace(command.ClientID))
	if err != nil {
		return OAuthTokenResponse{}, fmt.Errorf("%w: invalid OAuth client", domain.ErrUnauthorized)
	}
	plaintext, err := s.cipher.Decrypt(client.Credential.SecretCiphertext)
	if err != nil || subtle.ConstantTimeCompare(plaintext, []byte(command.ClientSecret)) != 1 {
		return OAuthTokenResponse{}, fmt.Errorf("%w: invalid OAuth client", domain.ErrUnauthorized)
	}
	now := s.now().UTC()
	if client.Credential.Status != domain.CredentialStatusActive || (client.Credential.ExpiresAt != nil && !client.Credential.ExpiresAt.After(now)) {
		return OAuthTokenResponse{}, fmt.Errorf("%w: invalid OAuth client", domain.ErrUnauthorized)
	}
	authorization, err := s.repository.GetOAuthAuthorizationByCodeHash(ctx, tokenDigest(command.Code))
	if err != nil || authorization.ClientID != client.Credential.ClientID || authorization.CredentialID != client.Credential.ID || authorization.RedirectURI != command.RedirectURI || authorization.ConsumedAt != nil || !authorization.ExpiresAt.After(now) {
		return OAuthTokenResponse{}, fmt.Errorf("%w: authorization code is invalid or expired", domain.ErrInvalidGrant)
	}
	if !pkceValue.MatchString(command.CodeVerifier) || !validPKCE(command.CodeVerifier, authorization.CodeChallenge) {
		return OAuthTokenResponse{}, fmt.Errorf("%w: PKCE verification failed", domain.ErrInvalidGrant)
	}
	if err := requireEnvironmentAccess(ctx, s.repository, authorization.OrganizationID, authorization.Environment); err != nil {
		return OAuthTokenResponse{}, err
	}
	installationID, err := s.id()
	if err != nil {
		return OAuthTokenResponse{}, fmt.Errorf("generate installation id: %w", err)
	}
	tokenID, err := s.id()
	if err != nil {
		return OAuthTokenResponse{}, fmt.Errorf("generate access token id: %w", err)
	}
	accessToken, err := randomToken("es_at_", 32)
	if err != nil {
		return OAuthTokenResponse{}, err
	}
	installation := domain.AppInstallation{
		ID: installationID, AppID: authorization.AppID, MerchantID: authorization.MerchantID,
		MerchantName: authorization.MerchantName, MerchantDomain: authorization.MerchantDomain,
		Environment: authorization.Environment, Status: domain.InstallationStatusActive,
		InstalledVersionID: authorization.InstalledVersionID, GrantedScopes: append([]string{}, authorization.GrantedScopes...),
		InstalledBy: authorization.ApprovedBy, InstalledAt: now, CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
	token := domain.OAuthAccessToken{
		ID: tokenID, AppID: authorization.AppID, InstallationID: installationID,
		CredentialID: authorization.CredentialID, TokenHash: tokenDigest(accessToken),
		Scopes: append([]string{}, authorization.GrantedScopes...), CreatedAt: now, ExpiresAt: now.Add(s.accessTokenTTL),
	}
	installed, err := s.repository.ConsumeOAuthAuthorization(ctx, authorization.ID, token, installation, now)
	if err != nil {
		return OAuthTokenResponse{}, err
	}
	return OAuthTokenResponse{
		AccessToken: accessToken, TokenType: "Bearer", ExpiresIn: int64(s.accessTokenTTL.Seconds()),
		Scope: strings.Join(authorization.GrantedScopes, " "), Installation: installed,
	}, nil
}

func tokenDigest(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

func validPKCE(verifier, challenge string) bool {
	hash := sha256.Sum256([]byte(verifier))
	computed := base64.RawURLEncoding.EncodeToString(hash[:])
	return subtle.ConstantTimeCompare([]byte(computed), []byte(challenge)) == 1
}

func containsExact(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
