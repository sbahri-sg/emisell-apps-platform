package bootstrap

import (
	"context"
	apprepo "emisell.app/platform/internal/app/repository"
	appservice "emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/installation/domain"
	installservice "emisell.app/platform/internal/installation/service"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/appmanifest"
	"emisell.app/platform/pkg/managedshipping"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"net/http"
)

// LocalProviderInstall never enables the built-in Emisell pilot or production.
// Admission must verify engine enrollment independently of credential binding:
// credentials may only be connected after consent/installation has completed.
type LocalProviderInstall struct {
	Enrolled  map[string]string
	Admission func(context.Context, string, string, string) error
	Readiness installservice.ShippingProviderReadiness
}

func InternalHandlerWithLocalProviderApps(pool, caps *pgxpool.Pool, logger *slog.Logger, signer appservice.ManagedShippingSigner, config LocalProviderInstall) (http.Handler, error) {
	if pool == nil || caps == nil || signer == nil || config.Admission == nil || config.Readiness == nil || len(config.Enrolled) == 0 {
		return nil, fault.Unavailable
	}
	enrolled := map[string]string{}
	for app, provider := range config.Enrolled {
		if app == "" || provider == "" || provider == "emisell" {
			return nil, fault.Forbidden
		}
		enrolled[app] = provider
	}
	base := internalServices(pool, caps, logger, nil, signer, nil, config.Readiness)
	source := ProviderInstallSource{Repo: apprepo.Postgres{Pool: caps}, Managed: base.Testing.Managed, Environment: "development", Enrolled: enrolled, Admission: config.Admission}
	base.Intents.Managed = source
	base.Lifecycle.Intents = base.Intents
	base.Testing.ProviderReady = func(ctx context.Context, a appservice.Assignment, v appservice.ManagedShippingRelease) error {
		if enrolled[v.Manifest.AppID] != v.Manifest.Binding.ProviderCode {
			return fault.Forbidden
		}
		return config.Admission(ctx, a.MerchantID, v.Manifest.AppID, v.Manifest.Binding.ProviderCode)
	}
	return base.Handler(), nil
}

// ProviderInstallSource is opt-in composition for reviewed V2 releases. It must
// be paired with a real ShippingProviderReadiness enforcing gateway enrollment.
// A release being signed alone does not activate this source in the server.
type ProviderInstallSource struct {
	Repo        apprepo.Postgres
	Managed     appservice.ManagedShipping
	Environment string
	Enrolled    map[string]string
	Admission   func(context.Context, string, string, string) error
}

func (s ProviderInstallSource) WithRelease(ctx context.Context, merchant, app, version string, fn func(domain.IntentRelease) error) error {
	if s.Environment != "development" || s.Admission == nil {
		return fault.Unavailable
	}
	return s.Repo.WithManagedAssignment(ctx, merchant, app, version, func(a appservice.Assignment, v appservice.ManagedShippingRelease) error {
		m := v.Manifest
		if m.Binding.ProviderCode == "emisell" {
			return fault.Forbidden
		}
		if m.Policy != managedshipping.ProviderAppPolicy || a.Status != "approved" || a.ReleaseSHA256 != v.SHA256 || !s.Managed.Readiness(v).ConfigurationReady || s.Enrolled[m.AppID] != m.Binding.ProviderCode {
			return fault.Forbidden
		}
		if err := s.Admission(ctx, merchant, m.AppID, m.Binding.ProviderCode); err != nil {
			return err
		}
		return fn(domain.IntentRelease{InstallPolicy: domain.ProviderAppPolicy, AppID: m.AppID, Name: m.Name, DeveloperID: m.DeveloperID, Version: m.Version, ManifestDigest: v.SHA256, Scopes: m.Scopes, Capabilities: []string{m.Capability}, ExecutionProfile: domain.ProviderAppProfile,
			ShippingProvider: &appmanifest.ShippingProviderBinding{Engine: m.Binding.Engine, ProviderCode: m.Binding.ProviderCode}, ManagedSource: &domain.ManagedSource{ReleaseID: v.ID, AssignmentID: a.ID, MerchantID: merchant, Environment: s.Environment}})
	})
}
