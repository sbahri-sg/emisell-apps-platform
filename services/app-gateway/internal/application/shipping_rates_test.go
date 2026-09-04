package application

import (
	"context"
	"errors"
	"testing"

	"emisell-app-platform/services/app-gateway/internal/domain"
)

type shippingRepositoryStub struct {
	items            []domain.MerchantInstalledApp
	installation     domain.AppInstallation
	app              domain.App
	version          domain.AppVersion
	entitlement      domain.OrganizationEntitlement
	entitlementError error
}

func (r shippingRepositoryStub) ListMerchantInstalledApps(context.Context, string, domain.Environment) ([]domain.MerchantInstalledApp, error) {
	return r.items, nil
}
func (r shippingRepositoryStub) GetInstallation(context.Context, string, string, string) (domain.AppInstallation, error) {
	return r.installation, nil
}
func (r shippingRepositoryStub) GetApp(context.Context, string, string) (domain.App, error) {
	return r.app, nil
}
func (r shippingRepositoryStub) GetVersion(context.Context, string, string, string) (domain.AppVersion, error) {
	return r.version, nil
}
func (r shippingRepositoryStub) GetOrganizationEntitlement(context.Context, string) (domain.OrganizationEntitlement, error) {
	return r.entitlement, r.entitlementError
}

type shippingClientStub struct {
	calls       int
	merchantID  string
	environment domain.Environment
	input       ShippingRateRequest
	response    ShippingRateResponse
	err         error
}

func (c *shippingClientStub) Calculate(_ context.Context, merchantID string, environment domain.Environment, _ string, input ShippingRateRequest) (ShippingRateResponse, error) {
	c.calls++
	c.merchantID, c.environment, c.input = merchantID, environment, input
	return c.response, c.err
}

func shippingFixture() shippingRepositoryStub {
	const org = "01995f72-0000-7000-8000-000000000001"
	const app = "01995f72-0000-7000-8000-000000000002"
	const installation = "01995f72-0000-7000-8000-000000000003"
	const version = "01995f72-0000-7000-8000-000000000004"
	return shippingRepositoryStub{
		items:        []domain.MerchantInstalledApp{{OrganizationID: org, AppID: app, InstallationID: installation, Status: domain.InstallationStatusActive}},
		installation: domain.AppInstallation{ID: installation, AppID: app, MerchantID: "merchant-a", Environment: domain.EnvironmentSandbox, Status: domain.InstallationStatusActive, InstalledVersionID: version},
		app:          domain.App{ID: app, OrganizationID: org, Status: domain.AppStatusActive},
		version: domain.AppVersion{ID: version, AppID: app, Status: domain.VersionStatusActive, Snapshot: domain.VersionSnapshot{Extensions: []domain.SnapshotExtension{{
			ExtensionID: "01995f72-0000-7000-8000-000000000005", Type: string(domain.ExtensionTypeShipping),
			Configuration: map[string]any{"capabilities": []any{ShippingRatesCalculate}},
		}}}},
		entitlement: domain.OrganizationEntitlement{OrganizationID: org, SandboxAccess: true},
	}
}

func shippingPrincipal() EmisellBackendPrincipal {
	return EmisellBackendPrincipal{MerchantID: "merchant-a", Environment: domain.EnvironmentSandbox, Permissions: []string{ShippingRatesCalculate}}
}

func TestShippingRateServiceResolvesOptionalCapabilityBeforeCallingAPIKurir(t *testing.T) {
	repository := shippingFixture()
	client := &shippingClientStub{response: ShippingRateResponse{Meta: ShippingRateMeta{Code: 200, Status: "success"}, Data: []ShippingRateOption{}}}
	service := NewShippingRateService(repository, client)
	input := ShippingRateRequest{Origin: "442", Destination: "1354", Weight: 1200}
	result, err := service.Calculate(t.Context(), shippingPrincipal(), input, "request-shipping-000001")
	if err != nil || result.Meta.Code != 200 || client.calls != 1 || client.merchantID != "merchant-a" || client.environment != domain.EnvironmentSandbox || client.input != input {
		t.Fatalf("result=%#v client=%#v err=%v", result, client, err)
	}
}

func TestShippingRateServiceFailsClosedForLifecycleCapabilityAndAmbiguity(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*shippingRepositoryStub)
		want   string
	}{
		{"capability omitted", func(r *shippingRepositoryStub) { r.version.Snapshot.Extensions[0].Configuration = map[string]any{} }, "shipping_extension_unavailable"},
		{"installation suspended", func(r *shippingRepositoryStub) {
			r.items[0].Status = domain.InstallationStatusSuspended
			r.installation.Status = domain.InstallationStatusSuspended
		}, "shipping_extension_unavailable"},
		{"app archived", func(r *shippingRepositoryStub) { r.app.Status = domain.AppStatusArchived }, "shipping_extension_unavailable"},
		{"draft installed version", func(r *shippingRepositoryStub) { r.version.Status = domain.VersionStatusDraft }, "shipping_extension_unavailable"},
		{"environment disabled", func(r *shippingRepositoryStub) { r.entitlement.SandboxAccess = false }, "shipping_extension_unavailable"},
		{"ambiguous extensions", func(r *shippingRepositoryStub) {
			r.version.Snapshot.Extensions = append(r.version.Snapshot.Extensions, domain.SnapshotExtension{ExtensionID: "01995f72-0000-7000-8000-000000000099", Type: string(domain.ExtensionTypeShipping), Configuration: map[string]any{"capabilities": []string{ShippingRatesCalculate}}})
		}, "shipping_extension_ambiguous"},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := shippingFixture()
			test.mutate(&repository)
			client := &shippingClientStub{}
			_, err := NewShippingRateService(repository, client).Calculate(t.Context(), shippingPrincipal(), ShippingRateRequest{Origin: "442", Destination: "1354", Weight: 1000}, "request-shipping-000001")
			var rateError *ShippingRateError
			if !errors.As(err, &rateError) || rateError.Code != test.want || client.calls != 0 {
				t.Fatalf("err=%v client calls=%d", err, client.calls)
			}
		})
	}
}

func TestShippingRateServiceRejectsCallerOverridesAndDisabledBridge(t *testing.T) {
	validInput := ShippingRateRequest{Origin: "442", Destination: "1354", Weight: 1000}
	for _, test := range []struct {
		name      string
		principal EmisellBackendPrincipal
		input     ShippingRateRequest
		client    ShippingRateClient
		want      string
	}{
		{"permission", EmisellBackendPrincipal{MerchantID: "merchant-a", Environment: domain.EnvironmentSandbox}, validInput, &shippingClientStub{}, "shipping_rate_forbidden"},
		{"merchant", EmisellBackendPrincipal{MerchantID: "bad merchant", Environment: domain.EnvironmentSandbox, Permissions: []string{ShippingRatesCalculate}}, validInput, &shippingClientStub{}, "invalid_merchant_context"},
		{"same location", shippingPrincipal(), ShippingRateRequest{Origin: "442", Destination: "442", Weight: 1000}, &shippingClientStub{}, "invalid_rate_request"},
		{"weight", shippingPrincipal(), ShippingRateRequest{Origin: "442", Destination: "1354", Weight: 0}, &shippingClientStub{}, "invalid_rate_request"},
		{"disabled", shippingPrincipal(), validInput, nil, "shipping_runtime_disabled"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewShippingRateService(shippingFixture(), test.client).Calculate(t.Context(), test.principal, test.input, "request-shipping-000001")
			var rateError *ShippingRateError
			if !errors.As(err, &rateError) || rateError.Code != test.want {
				t.Fatalf("unexpected err=%v", err)
			}
		})
	}
}

func TestShippingRateProductionRequiresEntitlement(t *testing.T) {
	repository := shippingFixture()
	repository.items[0].Status = domain.InstallationStatusActive
	repository.installation.Environment = domain.EnvironmentProduction
	repository.entitlement.ProductionAccess = false
	principal := shippingPrincipal()
	principal.Environment = domain.EnvironmentProduction
	client := &shippingClientStub{}
	_, err := NewShippingRateService(repository, client).Calculate(t.Context(), principal, ShippingRateRequest{Origin: "442", Destination: "1354", Weight: 1000}, "request-shipping-000001")
	var rateError *ShippingRateError
	if !errors.As(err, &rateError) || rateError.Code != "shipping_extension_unavailable" || client.calls != 0 {
		t.Fatalf("production entitlement not enforced: err=%v", err)
	}
}
