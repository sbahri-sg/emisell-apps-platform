package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

type MerchantService struct {
	repository         ports.MerchantRepository
	appRepository      ports.Repository
	identityRepository ports.IdentityRepository
	identity           *IdentityService
	id                 ids.Generator
	now                func() time.Time
}

type CreatedMerchantSession struct {
	Identity domain.MerchantIdentity `json:"merchant"`
	Session  CreatedIdentitySession  `json:"-"`
}

type MerchantSessionContext struct {
	Identity domain.MerchantIdentity
	Session  domain.IdentitySession
}

type CreateSandboxMerchantSessionCommand struct {
	MerchantID     string
	MerchantName   string
	MerchantDomain *string
}

func NewMerchantService(repository ports.MerchantRepository, appRepository ports.Repository, identityRepository ports.IdentityRepository, identity *IdentityService, id ids.Generator, now func() time.Time) *MerchantService {
	return &MerchantService{
		repository: repository, appRepository: appRepository, identityRepository: identityRepository,
		identity: identity, id: id, now: now,
	}
}

func (s *MerchantService) CreateSandboxSession(ctx context.Context, command CreateSandboxMerchantSessionCommand) (CreatedMerchantSession, error) {
	merchantID := strings.TrimSpace(command.MerchantID)
	if !externalIdentityValue.MatchString(merchantID) {
		return CreatedMerchantSession{}, fmt.Errorf("%w: merchantId must be a valid Emisell identifier", domain.ErrValidation)
	}
	name := strings.TrimSpace(command.MerchantName)
	if len([]rune(name)) < 2 || len([]rune(name)) > 120 {
		return CreatedMerchantSession{}, fmt.Errorf("%w: merchantName must contain 2 to 120 characters", domain.ErrValidation)
	}
	var merchantHost *string
	if command.MerchantDomain != nil && strings.TrimSpace(*command.MerchantDomain) != "" {
		value := strings.ToLower(strings.TrimSpace(*command.MerchantDomain))
		if !merchantDomain.MatchString(value) {
			return CreatedMerchantSession{}, fmt.Errorf("%w: merchantDomain must be a hostname without protocol", domain.ErrValidation)
		}
		merchantHost = &value
	}
	userID, err := s.id()
	if err != nil {
		return CreatedMerchantSession{}, fmt.Errorf("generate merchant user id: %w", err)
	}
	emailID := strings.NewReplacer("-", "", ":", "", ".", "", "_", "").Replace(merchantID)
	email := "sandbox+" + emailID + "@merchant.emisell.test"
	now := s.now().UTC()
	resolvedUserID, err := s.identityRepository.ProvisionOIDCUser(ctx, "sandbox-merchant", merchantID, email, name, userID, now)
	if err != nil {
		return CreatedMerchantSession{}, err
	}
	identity, err := s.repository.UpsertMerchantIdentity(ctx, domain.MerchantIdentity{
		MerchantID: merchantID, UserID: resolvedUserID, Name: name, Domain: merchantHost,
		Environment: domain.EnvironmentSandbox, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return CreatedMerchantSession{}, err
	}
	session, err := s.identity.CreateMerchantSession(ctx, resolvedUserID, email, name, identity.MerchantID, identity.Environment)
	if err != nil {
		return CreatedMerchantSession{}, err
	}
	return CreatedMerchantSession{Identity: identity, Session: session}, nil
}

func (s *MerchantService) Authenticate(ctx context.Context, sessionToken string) (MerchantSessionContext, error) {
	session, _, err := s.identity.Authenticate(ctx, sessionToken)
	if err != nil {
		return MerchantSessionContext{}, err
	}
	if session.MerchantID == nil || session.MerchantEnvironment == nil {
		return MerchantSessionContext{}, domain.ErrUnauthorized
	}
	identity, err := s.repository.GetMerchantIdentity(ctx, session.UserID, *session.MerchantID, *session.MerchantEnvironment)
	if errors.Is(err, domain.ErrNotFound) {
		return MerchantSessionContext{}, domain.ErrUnauthorized
	}
	if err != nil {
		return MerchantSessionContext{}, err
	}
	if identity.Environment != domain.EnvironmentSandbox && identity.Environment != domain.EnvironmentProduction {
		return MerchantSessionContext{}, domain.ErrUnauthorized
	}
	return MerchantSessionContext{Identity: identity, Session: session}, nil
}

func (s *MerchantService) ValidateCSRF(session domain.IdentitySession, csrfToken string) error {
	return s.identity.ValidateCSRF(session, csrfToken)
}

func (s *MerchantService) Revoke(ctx context.Context, sessionToken string) error {
	return s.identity.Revoke(ctx, sessionToken)
}

func (s *MerchantService) ListInstalledApps(ctx context.Context, merchant MerchantSessionContext) ([]domain.MerchantInstalledApp, error) {
	return s.repository.ListMerchantInstalledApps(ctx, merchant.Identity.MerchantID, merchant.Identity.Environment)
}

func (s *MerchantService) Uninstall(ctx context.Context, merchant MerchantSessionContext, installationID, idempotencyKey string) error {
	if !uuidValue.MatchString(installationID) {
		return fmt.Errorf("%w: installationId must be a UUID", domain.ErrValidation)
	}
	installed, err := s.repository.GetMerchantInstalledApp(ctx, merchant.Identity.MerchantID, installationID, merchant.Identity.Environment)
	if err != nil {
		return err
	}
	return s.appRepository.UninstallInstallation(ctx, installed.OrganizationID, installed.AppID, installed.InstallationID, ports.MutationMeta{
		ActorID: merchant.Session.UserID, Action: "installation.uninstalled_by_merchant", IdempotencyKey: idempotencyKey,
	})
}
