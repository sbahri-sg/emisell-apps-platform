package application

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

type DevelopmentInstallService struct {
	repository ports.Repository
	merchants  ports.MerchantRepository
	id         ids.Generator
	now        func() time.Time
	ttl        time.Duration
	pilot      DevelopmentResourcePilot
}

type CreateDevelopmentInstallRequestCommand struct {
	OrganizationID string
	ActorID        string
	IdempotencyKey string
	AppID          string
	MerchantID     string
}

func NewDevelopmentInstallService(repository ports.Repository, merchants ports.MerchantRepository, id ids.Generator, now func() time.Time, ttl time.Duration, pilots ...DevelopmentResourcePilot) *DevelopmentInstallService {
	service := &DevelopmentInstallService{repository: repository, merchants: merchants, id: id, now: now, ttl: ttl}
	if len(pilots) > 0 {
		service.pilot = pilots[0]
	}
	return service
}

func (s *DevelopmentInstallService) List(ctx context.Context, organizationID, appID string) ([]domain.DevelopmentInstallRequest, error) {
	return s.repository.ListDevelopmentInstallRequests(ctx, organizationID, appID)
}

func (s *DevelopmentInstallService) Create(ctx context.Context, command CreateDevelopmentInstallRequestCommand) (domain.DevelopmentInstallRequest, error) {
	merchantID := strings.TrimSpace(command.MerchantID)
	if !externalIdentityValue.MatchString(merchantID) {
		return domain.DevelopmentInstallRequest{}, fmt.Errorf("%w: merchantId must be a valid Emisell identifier", domain.ErrValidation)
	}
	app, err := s.repository.GetApp(ctx, command.OrganizationID, command.AppID)
	if err != nil {
		return domain.DevelopmentInstallRequest{}, err
	}
	if app.Status != domain.AppStatusActive || app.ActiveVersionID == nil {
		return domain.DevelopmentInstallRequest{}, fmt.Errorf("%w: activate a development version before creating a test request", domain.ErrConflict)
	}
	if app.AppURL == nil || strings.TrimSpace(*app.AppURL) == "" {
		return domain.DevelopmentInstallRequest{}, fmt.Errorf("%w: configure the app launch URL before creating a test request", domain.ErrConflict)
	}
	releaseStatus, err := resolveAppReleaseStatus(ctx, s.repository, command.OrganizationID, app.ID)
	if err != nil {
		return domain.DevelopmentInstallRequest{}, err
	}
	if releaseStatus == domain.AppReleaseStatusReleased {
		return domain.DevelopmentInstallRequest{}, fmt.Errorf("%w: released apps must be installed from the Emisell App Store", domain.ErrConflict)
	}
	version, err := s.repository.GetVersion(ctx, command.OrganizationID, app.ID, *app.ActiveVersionID)
	if err != nil {
		return domain.DevelopmentInstallRequest{}, err
	}
	merchant, err := s.merchants.ResolveMerchantIdentity(ctx, merchantID)
	if err != nil {
		return domain.DevelopmentInstallRequest{}, fmt.Errorf("%w: Merchant ID is not registered in Emisell", domain.ErrNotFound)
	}
	if err := requireEnvironmentAccess(ctx, s.repository, command.OrganizationID, merchant.Environment); err != nil {
		return domain.DevelopmentInstallRequest{}, err
	}
	if unavailable := s.pilot.unavailable(version.Snapshot.Scopes, merchant); unavailable != "" {
		return domain.DevelopmentInstallRequest{}, fmt.Errorf("%w: scope %q is not available for development installations", domain.ErrConflict, unavailable)
	}
	requestID, err := s.id()
	if err != nil {
		return domain.DevelopmentInstallRequest{}, fmt.Errorf("generate development install request id: %w", err)
	}
	launchURL, err := url.Parse(*app.AppURL)
	if err != nil {
		return domain.DevelopmentInstallRequest{}, fmt.Errorf("parse app launch URL: %w", err)
	}
	query := launchURL.Query()
	query.Set("emisell_test_install_request", requestID)
	launchURL.RawQuery = query.Encode()
	now := s.now().UTC()
	request := domain.DevelopmentInstallRequest{
		ID: requestID, AppID: app.ID, MerchantID: merchant.MerchantID, MerchantName: merchant.Name,
		MerchantDomain: merchant.Domain, Environment: merchant.Environment, VersionID: version.ID,
		Status: domain.DevelopmentInstallRequestStatusPending, LaunchURL: launchURL.String(), RequestedBy: command.ActorID,
		CreatedAt: now, UpdatedAt: now, ExpiresAt: now.Add(s.ttl),
	}
	return s.repository.CreateDevelopmentInstallRequest(ctx, command.OrganizationID, request, ports.MutationMeta{
		ActorID: command.ActorID, Action: "development_install_request.created", IdempotencyKey: command.IdempotencyKey,
	})
}
