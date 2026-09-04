package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

type IdentityService struct {
	repository ports.IdentityRepository
	id         ids.Generator
	now        func() time.Time
	sessionTTL time.Duration
	idleTTL    time.Duration
}

type CreatedIdentitySession struct {
	Session      domain.IdentitySession
	Membership   *domain.OrganizationMembership
	SessionToken string
	CSRFToken    string
}

func NewIdentityService(repository ports.IdentityRepository, id ids.Generator, now func() time.Time, sessionTTL, idleTTL time.Duration) *IdentityService {
	return &IdentityService{repository: repository, id: id, now: now, sessionTTL: sessionTTL, idleTTL: idleTTL}
}

func (s *IdentityService) CreateSession(ctx context.Context, userID, email, displayName string, platformOperator bool, preferredOrganizationID string) (CreatedIdentitySession, error) {
	return s.createSession(ctx, userID, email, displayName, platformOperator, preferredOrganizationID, nil, nil)
}

func (s *IdentityService) CreateMerchantSession(ctx context.Context, userID, email, displayName, merchantID string, environment domain.Environment) (CreatedIdentitySession, error) {
	return s.createSession(ctx, userID, email, displayName, false, "", &merchantID, &environment)
}

func (s *IdentityService) createSession(ctx context.Context, userID, email, displayName string, platformOperator bool, preferredOrganizationID string, merchantID *string, merchantEnvironment *domain.Environment) (CreatedIdentitySession, error) {
	memberships, err := s.repository.ListOrganizationMemberships(ctx, userID)
	if err != nil {
		return CreatedIdentitySession{}, err
	}
	var active *domain.OrganizationMembership
	for index := range memberships {
		membership := &memberships[index]
		if membership.Status != "active" {
			continue
		}
		if active == nil || strings.EqualFold(membership.OrganizationID, preferredOrganizationID) {
			copy := *membership
			active = &copy
		}
		if strings.EqualFold(membership.OrganizationID, preferredOrganizationID) {
			break
		}
	}
	sessionToken, err := randomIdentityToken()
	if err != nil {
		return CreatedIdentitySession{}, fmt.Errorf("generate session token: %w", err)
	}
	csrfToken, err := randomIdentityToken()
	if err != nil {
		return CreatedIdentitySession{}, fmt.Errorf("generate CSRF token: %w", err)
	}
	sessionID, err := s.id()
	if err != nil {
		return CreatedIdentitySession{}, fmt.Errorf("generate session id: %w", err)
	}
	now := s.now().UTC()
	session := domain.IdentitySession{
		ID: sessionID, TokenHash: IdentityTokenDigest(sessionToken), CSRFTokenHash: IdentityTokenDigest(csrfToken),
		UserID: userID, Email: strings.ToLower(strings.TrimSpace(email)), DisplayName: strings.TrimSpace(displayName),
		PlatformOperator: platformOperator, CreatedAt: now, LastSeenAt: now,
		ExpiresAt: now.Add(s.sessionTTL), IdleExpiresAt: now.Add(s.idleTTL),
		MerchantID: merchantID, MerchantEnvironment: merchantEnvironment,
	}
	if session.IdleExpiresAt.After(session.ExpiresAt) {
		session.IdleExpiresAt = session.ExpiresAt
	}
	if active != nil {
		session.ActiveOrgID = &active.OrganizationID
	}
	if err := s.repository.CreateIdentitySession(ctx, session); err != nil {
		return CreatedIdentitySession{}, err
	}
	return CreatedIdentitySession{Session: session, Membership: active, SessionToken: sessionToken, CSRFToken: csrfToken}, nil
}

func (s *IdentityService) Authenticate(ctx context.Context, sessionToken string) (domain.IdentitySession, *domain.OrganizationMembership, error) {
	if strings.TrimSpace(sessionToken) == "" {
		return domain.IdentitySession{}, nil, domain.ErrUnauthorized
	}
	return s.repository.GetIdentitySessionByTokenHash(ctx, IdentityTokenDigest(sessionToken), s.now().UTC(), s.idleTTL)
}

func (s *IdentityService) ValidateCSRF(session domain.IdentitySession, csrfToken string) error {
	digest := IdentityTokenDigest(strings.TrimSpace(csrfToken))
	if strings.TrimSpace(csrfToken) == "" || subtle.ConstantTimeCompare([]byte(digest), []byte(session.CSRFTokenHash)) != 1 {
		return domain.ErrForbidden
	}
	return nil
}

func (s *IdentityService) ListOrganizations(ctx context.Context, userID string) ([]domain.OrganizationMembership, error) {
	return s.repository.ListOrganizationMemberships(ctx, userID)
}

func (s *IdentityService) SwitchOrganization(ctx context.Context, sessionID, userID, organizationID string) (domain.IdentitySession, domain.OrganizationMembership, error) {
	if strings.TrimSpace(organizationID) == "" {
		return domain.IdentitySession{}, domain.OrganizationMembership{}, fmt.Errorf("%w: organizationId is required", domain.ErrValidation)
	}
	return s.repository.SwitchIdentitySessionOrganization(ctx, sessionID, userID, organizationID, s.now().UTC())
}

func (s *IdentityService) Revoke(ctx context.Context, sessionToken string) error {
	if strings.TrimSpace(sessionToken) == "" {
		return nil
	}
	return s.repository.RevokeIdentitySession(ctx, IdentityTokenDigest(sessionToken), s.now().UTC())
}

func IdentityTokenDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func randomIdentityToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
