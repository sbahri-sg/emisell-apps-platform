package application

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

type EmisellBackendPrincipal struct {
	Subject      string
	JTI          string
	Email        string
	DisplayName  string
	MerchantID   string
	MerchantName string
	Domain       *string
	Environment  domain.Environment
	Permissions  []string
}

type EmisellIntegrationService struct {
	grants     ports.MerchantSessionGrantRepository
	merchants  ports.MerchantRepository
	identities ports.IdentityRepository
	identity   *IdentityService
	id         ids.Generator
	now        func() time.Time
	grantTTL   time.Duration
	gatewayURL string
}

type CreatedMerchantSessionGrant struct {
	ExchangeURL string    `json:"exchangeUrl"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

type ExchangedMerchantSession struct {
	Created  CreatedMerchantSession
	ReturnTo string
}

func NewEmisellIntegrationService(
	grants ports.MerchantSessionGrantRepository,
	merchants ports.MerchantRepository,
	identities ports.IdentityRepository,
	identity *IdentityService,
	id ids.Generator,
	now func() time.Time,
	grantTTL time.Duration,
	gatewayURL string,
) *EmisellIntegrationService {
	return &EmisellIntegrationService{
		grants: grants, merchants: merchants, identities: identities, identity: identity,
		id: id, now: now, grantTTL: grantTTL, gatewayURL: strings.TrimRight(gatewayURL, "/"),
	}
}

func (s *EmisellIntegrationService) CreateMerchantSessionGrant(ctx context.Context, principal EmisellBackendPrincipal, returnTo string) (CreatedMerchantSessionGrant, error) {
	principal.Subject = strings.TrimSpace(principal.Subject)
	principal.JTI = strings.TrimSpace(principal.JTI)
	principal.MerchantID = strings.TrimSpace(principal.MerchantID)
	principal.Email = strings.ToLower(strings.TrimSpace(principal.Email))
	principal.DisplayName = strings.TrimSpace(principal.DisplayName)
	principal.MerchantName = strings.TrimSpace(principal.MerchantName)
	if !externalIdentityValue.MatchString(principal.Subject) || !externalIdentityValue.MatchString(principal.MerchantID) {
		return CreatedMerchantSessionGrant{}, fmt.Errorf("%w: subject and merchantId must be valid Emisell identifiers", domain.ErrValidation)
	}
	if len(principal.JTI) < 16 || len(principal.JTI) > 128 {
		return CreatedMerchantSessionGrant{}, fmt.Errorf("%w: jti must contain 16 to 128 characters", domain.ErrValidation)
	}
	if principal.Email == "" || !strings.Contains(principal.Email, "@") || len(principal.Email) > 254 {
		return CreatedMerchantSessionGrant{}, fmt.Errorf("%w: email is invalid", domain.ErrValidation)
	}
	if len([]rune(principal.DisplayName)) < 2 || len([]rune(principal.DisplayName)) > 120 || len([]rune(principal.MerchantName)) < 2 || len([]rune(principal.MerchantName)) > 120 {
		return CreatedMerchantSessionGrant{}, fmt.Errorf("%w: display name and merchant name must contain 2 to 120 characters", domain.ErrValidation)
	}
	if principal.Environment != domain.EnvironmentSandbox && principal.Environment != domain.EnvironmentProduction {
		return CreatedMerchantSessionGrant{}, fmt.Errorf("%w: environment must be sandbox or production", domain.ErrValidation)
	}
	if !slices.Contains(principal.Permissions, "apps.install") {
		return CreatedMerchantSessionGrant{}, domain.ErrForbidden
	}
	var merchantHost *string
	if principal.Domain != nil && strings.TrimSpace(*principal.Domain) != "" {
		value := strings.ToLower(strings.TrimSpace(*principal.Domain))
		if !merchantDomain.MatchString(value) {
			return CreatedMerchantSessionGrant{}, fmt.Errorf("%w: merchant domain must be a hostname without protocol", domain.ErrValidation)
		}
		merchantHost = &value
	}
	returnTo = strings.TrimSpace(returnTo)
	if !safeMerchantReturnTo(returnTo) {
		return CreatedMerchantSessionGrant{}, fmt.Errorf("%w: returnTo must be a relative /merchant or /install path", domain.ErrValidation)
	}

	grantID, err := s.id()
	if err != nil {
		return CreatedMerchantSessionGrant{}, fmt.Errorf("generate merchant session grant id: %w", err)
	}
	code, err := randomIdentityToken()
	if err != nil {
		return CreatedMerchantSessionGrant{}, fmt.Errorf("generate merchant session grant code: %w", err)
	}
	permissions := uniqueStrings(principal.Permissions)
	sort.Strings(permissions)
	now := s.now().UTC()
	grant := domain.MerchantSessionGrant{
		ID: grantID, CodeHash: IdentityTokenDigest(code), SourceJTI: principal.JTI, Subject: principal.Subject,
		Email: principal.Email, DisplayName: principal.DisplayName, MerchantID: principal.MerchantID,
		MerchantName: principal.MerchantName, Domain: merchantHost, Environment: principal.Environment,
		Permissions: permissions, ReturnTo: returnTo, CreatedAt: now, ExpiresAt: now.Add(s.grantTTL),
	}
	if err := s.grants.CreateMerchantSessionGrant(ctx, grant); err != nil {
		return CreatedMerchantSessionGrant{}, err
	}
	exchange := s.gatewayURL + "/auth/emisell-merchant/exchange?code=" + url.QueryEscape(code)
	return CreatedMerchantSessionGrant{ExchangeURL: exchange, ExpiresAt: grant.ExpiresAt}, nil
}

func (s *EmisellIntegrationService) ExchangeMerchantSessionGrant(ctx context.Context, code string) (ExchangedMerchantSession, error) {
	code = strings.TrimSpace(code)
	if code == "" || len(code) > 256 {
		return ExchangedMerchantSession{}, domain.ErrInvalidGrant
	}
	grant, err := s.grants.ConsumeMerchantSessionGrant(ctx, IdentityTokenDigest(code), s.now().UTC())
	if err != nil {
		return ExchangedMerchantSession{}, err
	}
	userID, err := s.id()
	if err != nil {
		return ExchangedMerchantSession{}, fmt.Errorf("generate merchant user id: %w", err)
	}
	now := s.now().UTC()
	resolvedUserID, err := s.identities.ProvisionOIDCUser(ctx, "emisell-backend", grant.Subject, grant.Email, grant.DisplayName, userID, now)
	if err != nil {
		return ExchangedMerchantSession{}, err
	}
	identity, err := s.merchants.UpsertMerchantIdentity(ctx, domain.MerchantIdentity{
		MerchantID: grant.MerchantID, UserID: resolvedUserID, Name: grant.MerchantName,
		Domain: grant.Domain, Environment: grant.Environment, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return ExchangedMerchantSession{}, err
	}
	session, err := s.identity.CreateMerchantSession(ctx, resolvedUserID, grant.Email, grant.DisplayName, identity.MerchantID, identity.Environment)
	if err != nil {
		return ExchangedMerchantSession{}, err
	}
	return ExchangedMerchantSession{
		Created: CreatedMerchantSession{Identity: identity, Session: session}, ReturnTo: grant.ReturnTo,
	}, nil
}

func safeMerchantReturnTo(value string) bool {
	if len(value) == 0 || len(value) > 2048 || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") || strings.ContainsAny(value, "\r\n") {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return false
	}
	return parsed.Path == "/install" || strings.HasPrefix(parsed.Path, "/install/") || parsed.Path == "/merchant" || strings.HasPrefix(parsed.Path, "/merchant/")
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
