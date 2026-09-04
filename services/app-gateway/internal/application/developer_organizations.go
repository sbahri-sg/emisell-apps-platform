package application

import (
	"context"
	"fmt"
	"strings"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

type DeveloperOrganizationService struct {
	repository ports.DeveloperOrganizationRepository
}

func NewDeveloperOrganizationService(repository ports.DeveloperOrganizationRepository) *DeveloperOrganizationService {
	return &DeveloperOrganizationService{repository: repository}
}

func (s *DeveloperOrganizationService) List(ctx context.Context, filter ports.DeveloperOrganizationFilter) ([]domain.DeveloperOrganization, ports.PageMeta, error) {
	filter.Search = strings.TrimSpace(filter.Search)
	if len(filter.Search) > 100 {
		return nil, ports.PageMeta{}, fmt.Errorf("%w: search must not exceed 100 characters", domain.ErrValidation)
	}
	if filter.Status != "" && filter.Status != domain.OrganizationStatusActive && filter.Status != domain.OrganizationStatusSuspended {
		return nil, ports.PageMeta{}, fmt.Errorf("%w: invalid organization status", domain.ErrValidation)
	}
	return s.repository.ListDeveloperOrganizations(ctx, filter)
}

func (s *DeveloperOrganizationService) Get(ctx context.Context, organizationID string) (domain.DeveloperOrganizationDetail, error) {
	if !uuidValue.MatchString(strings.TrimSpace(organizationID)) {
		return domain.DeveloperOrganizationDetail{}, fmt.Errorf("%w: organizationId must be a UUID", domain.ErrValidation)
	}
	return s.repository.GetDeveloperOrganization(ctx, organizationID)
}
