package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

type InstallationAccessService struct {
	repository ports.Repository
	now        func() time.Time
}

const ScopeReadMerchant = "read_merchant"

func NewInstallationAccessService(repository ports.Repository, now func() time.Time) *InstallationAccessService {
	return &InstallationAccessService{repository: repository, now: now}
}

func (s *InstallationAccessService) Authenticate(ctx context.Context, accessToken string, requiredScopes ...string) (domain.InstallationAccessContext, error) {
	token := strings.TrimSpace(accessToken)
	if len(token) < 16 || len(token) > 512 || !strings.HasPrefix(token, "es_at_") {
		return domain.InstallationAccessContext{}, domain.ErrUnauthorized
	}
	access, err := s.repository.GetInstallationAccessContextByTokenHash(ctx, tokenDigest(token), s.now().UTC())
	if errors.Is(err, domain.ErrNotFound) {
		return domain.InstallationAccessContext{}, domain.ErrUnauthorized
	}
	if err != nil {
		return domain.InstallationAccessContext{}, err
	}
	if access.InstallationStatus != domain.InstallationStatusActive {
		return domain.InstallationAccessContext{}, domain.ErrUnauthorized
	}
	if err := requireEnvironmentAccess(ctx, s.repository, access.OrganizationID, access.Environment); err != nil {
		if errors.Is(err, domain.ErrForbidden) || errors.Is(err, domain.ErrNotFound) {
			return domain.InstallationAccessContext{}, domain.ErrUnauthorized
		}
		return domain.InstallationAccessContext{}, err
	}
	if err := RequireInstallationScopes(access, requiredScopes...); err != nil {
		return domain.InstallationAccessContext{}, err
	}
	return access, nil
}

func RequireInstallationScopes(access domain.InstallationAccessContext, requiredScopes ...string) error {
	granted := make(map[string]struct{}, len(access.Scopes))
	for _, scope := range access.Scopes {
		granted[scope] = struct{}{}
	}
	for _, raw := range requiredScopes {
		scope := strings.TrimSpace(raw)
		if scope == "" {
			continue
		}
		if _, ok := granted[scope]; !ok {
			return fmt.Errorf("%w: installation token is missing required scope %q", domain.ErrForbidden, scope)
		}
	}
	return nil
}

func (s *InstallationAccessService) GetMerchantProfile(ctx context.Context, accessToken string) (domain.MerchantProfile, error) {
	access, err := s.Authenticate(ctx, accessToken, ScopeReadMerchant)
	if err != nil {
		return domain.MerchantProfile{}, err
	}
	return domain.MerchantProfile{
		ID:             access.MerchantID,
		Name:           access.MerchantName,
		Domain:         access.MerchantDomain,
		Environment:    access.Environment,
		InstallationID: access.InstallationID,
	}, nil
}
