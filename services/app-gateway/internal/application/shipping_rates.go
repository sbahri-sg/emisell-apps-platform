package application

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"emisell-app-platform/services/app-gateway/internal/domain"
)

const ShippingRatesCalculate = "shipping.rates.calculate"

var shippingLocationID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

type ShippingRateRequest struct {
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
	Weight      int64  `json:"weight"`
}

type ShippingRateMeta struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
	Status  string `json:"status"`
}

type ShippingRateOption struct {
	Name             string `json:"name"`
	Code             string `json:"code"`
	Logo             string `json:"logo"`
	Service          string `json:"service"`
	CanonicalService string `json:"canonicalService"`
	ServiceGroup     string `json:"serviceGroup"`
	ServiceType      string `json:"serviceType"`
	Description      string `json:"description"`
	Cost             int64  `json:"cost"`
	ETD              string `json:"etd"`
}

type ShippingRateResponse struct {
	Meta ShippingRateMeta     `json:"meta"`
	Data []ShippingRateOption `json:"data"`
}

type ShippingRateError struct {
	Status     int
	Code       string
	RetryAfter int
}

func (e *ShippingRateError) Error() string { return e.Code }

type ShippingRateClient interface {
	Calculate(context.Context, string, domain.Environment, string, ShippingRateRequest) (ShippingRateResponse, error)
}

// ShippingRateRepository is intentionally narrow so the runtime resolver can
// be tested without depending on every control-plane repository operation.
type ShippingRateRepository interface {
	ListMerchantInstalledApps(context.Context, string, domain.Environment) ([]domain.MerchantInstalledApp, error)
	GetInstallation(context.Context, string, string, string) (domain.AppInstallation, error)
	GetApp(context.Context, string, string) (domain.App, error)
	GetVersion(context.Context, string, string, string) (domain.AppVersion, error)
	GetOrganizationEntitlement(context.Context, string) (domain.OrganizationEntitlement, error)
}

type ShippingRateService struct {
	repository ShippingRateRepository
	client     ShippingRateClient
}

func NewShippingRateService(repository ShippingRateRepository, client ShippingRateClient) *ShippingRateService {
	return &ShippingRateService{repository: repository, client: client}
}

func (s *ShippingRateService) Calculate(ctx context.Context, principal EmisellBackendPrincipal, input ShippingRateRequest, requestID string) (ShippingRateResponse, error) {
	principal.MerchantID = strings.TrimSpace(principal.MerchantID)
	input.Origin = strings.TrimSpace(input.Origin)
	input.Destination = strings.TrimSpace(input.Destination)
	if !externalIdentityValue.MatchString(principal.MerchantID) ||
		(principal.Environment != domain.EnvironmentSandbox && principal.Environment != domain.EnvironmentProduction) {
		return ShippingRateResponse{}, &ShippingRateError{Status: 400, Code: "invalid_merchant_context"}
	}
	if !slices.Contains(principal.Permissions, ShippingRatesCalculate) {
		return ShippingRateResponse{}, &ShippingRateError{Status: 403, Code: "shipping_rate_forbidden"}
	}
	if !shippingLocationID.MatchString(input.Origin) || !shippingLocationID.MatchString(input.Destination) ||
		input.Origin == input.Destination || input.Weight < 1 || input.Weight > 100_000_000 {
		return ShippingRateResponse{}, &ShippingRateError{Status: 400, Code: "invalid_rate_request"}
	}
	if s == nil || s.repository == nil || s.client == nil {
		return ShippingRateResponse{}, &ShippingRateError{Status: 503, Code: "shipping_runtime_disabled"}
	}

	selection, err := s.resolve(ctx, principal.MerchantID, principal.Environment)
	if err != nil {
		return ShippingRateResponse{}, err
	}
	// The selection is deliberately not sent upstream. API Kurir receives only
	// the authenticated merchant context and owns provider/credential choice.
	_ = selection
	return s.client.Calculate(ctx, principal.MerchantID, principal.Environment, requestID, input)
}

type shippingRuntimeSelection struct {
	OrganizationID string
	AppID          string
	InstallationID string
	VersionID      string
	ExtensionID    string
}

func (s *ShippingRateService) resolve(ctx context.Context, merchantID string, environment domain.Environment) (shippingRuntimeSelection, error) {
	installed, err := s.repository.ListMerchantInstalledApps(ctx, merchantID, environment)
	if err != nil {
		return shippingRuntimeSelection{}, &ShippingRateError{Status: 503, Code: "shipping_runtime_unavailable"}
	}
	candidates := make([]shippingRuntimeSelection, 0, 1)
	for _, item := range installed {
		if item.Status != domain.InstallationStatusActive {
			continue
		}
		installation, err := s.repository.GetInstallation(ctx, item.OrganizationID, item.AppID, item.InstallationID)
		if err != nil {
			return shippingRuntimeSelection{}, &ShippingRateError{Status: 503, Code: "shipping_runtime_unavailable"}
		}
		if installation.Status != domain.InstallationStatusActive || installation.MerchantID != merchantID || installation.Environment != environment {
			continue
		}
		app, err := s.repository.GetApp(ctx, item.OrganizationID, item.AppID)
		if err != nil {
			return shippingRuntimeSelection{}, &ShippingRateError{Status: 503, Code: "shipping_runtime_unavailable"}
		}
		if app.Status != domain.AppStatusActive {
			continue
		}
		if err := shippingEnvironmentAccess(ctx, s.repository, item.OrganizationID, environment); err != nil {
			if errors.Is(err, domain.ErrForbidden) || errors.Is(err, domain.ErrNotFound) {
				continue
			}
			return shippingRuntimeSelection{}, &ShippingRateError{Status: 503, Code: "shipping_runtime_unavailable"}
		}
		version, err := s.repository.GetVersion(ctx, item.OrganizationID, item.AppID, installation.InstalledVersionID)
		if err != nil || version.AppID != item.AppID {
			return shippingRuntimeSelection{}, &ShippingRateError{Status: 503, Code: "shipping_runtime_unavailable"}
		}
		if version.Status != domain.VersionStatusActive && version.Status != domain.VersionStatusReleased {
			continue
		}
		for _, extension := range version.Snapshot.Extensions {
			if extension.Type != string(domain.ExtensionTypeShipping) || !snapshotSupportsCapability(extension.Configuration, ShippingRatesCalculate) {
				continue
			}
			candidates = append(candidates, shippingRuntimeSelection{
				OrganizationID: item.OrganizationID, AppID: item.AppID, InstallationID: installation.ID,
				VersionID: version.ID, ExtensionID: extension.ExtensionID,
			})
		}
	}
	if len(candidates) == 0 {
		return shippingRuntimeSelection{}, &ShippingRateError{Status: 409, Code: "shipping_extension_unavailable"}
	}
	if len(candidates) != 1 {
		return shippingRuntimeSelection{}, &ShippingRateError{Status: 409, Code: "shipping_extension_ambiguous"}
	}
	return candidates[0], nil
}

func shippingEnvironmentAccess(ctx context.Context, repository ShippingRateRepository, organizationID string, environment domain.Environment) error {
	entitlement, err := repository.GetOrganizationEntitlement(ctx, organizationID)
	if errors.Is(err, domain.ErrNotFound) && environment == domain.EnvironmentSandbox {
		return nil
	}
	if err != nil {
		return err
	}
	if environment == domain.EnvironmentSandbox && !entitlement.SandboxAccess {
		return fmt.Errorf("%w: sandbox access is disabled", domain.ErrForbidden)
	}
	if environment == domain.EnvironmentProduction && !entitlement.ProductionAccess {
		return fmt.Errorf("%w: production access is disabled", domain.ErrForbidden)
	}
	return nil
}

func snapshotSupportsCapability(configuration map[string]any, capability string) bool {
	if configuration == nil {
		return false
	}
	switch values := configuration["capabilities"].(type) {
	case []string:
		return slices.Contains(values, capability)
	case []any:
		for _, raw := range values {
			if value, ok := raw.(string); ok && strings.TrimSpace(value) == capability {
				return true
			}
		}
	}
	return false
}
