package bootstrap

import (
	"context"
	apprepo "emisell.app/platform/internal/app/repository"
	appservice "emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/installation/domain"
	installrepo "emisell.app/platform/internal/installation/postgres"
	installservice "emisell.app/platform/internal/installation/service"
	"emisell.app/platform/internal/oauth"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/appmanifest"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"net/http"
)

type managedInstallSource struct {
	Repo    apprepo.Postgres
	Managed appservice.ManagedShipping
}

func (s managedInstallSource) WithRelease(ctx context.Context, merchant, app, version string, fn func(domain.IntentRelease) error) error {
	return s.Repo.WithManagedAssignment(ctx, merchant, app, version, func(a appservice.Assignment, v appservice.ManagedShippingRelease) error {
		if a.Status != "approved" || a.ReleaseSHA256 != v.SHA256 || !s.Managed.Readiness(v).ConfigurationReady {
			return fault.Forbidden
		}
		m := v.Manifest
		return fn(domain.IntentRelease{InstallPolicy: domain.ManagedShippingPolicy, AppID: m.AppID, Name: m.Name, DeveloperID: m.DeveloperID, Version: m.Version, ManifestDigest: v.SHA256,
			Scopes: m.Scopes, Capabilities: []string{m.Capability}, ExecutionProfile: domain.ManagedShippingProfile,
			ShippingProvider: &appmanifest.ShippingProviderBinding{Engine: m.Binding.Engine, ProviderCode: m.Binding.ProviderCode},
			ManagedSource:    &domain.ManagedSource{ReleaseID: v.ID, AssignmentID: a.ID, MerchantID: merchant, Environment: "local-isolated"}})
	})
}

// LocalManaged enables the separately authenticated engine pilot only when the
// local composition explicitly supplies its readiness checker and private key.
type LocalManaged struct {
	ReviewedUI    *ReviewedUIRuntime
	EngineKey     string
	Readiness     installservice.ShippingProviderReadiness
	EmbeddedPilot *installservice.LocalEmbeddedPilot
}

func (c LocalManaged) Valid() bool { return len(c.EngineKey) == 64 && c.Readiness != nil }

func InternalHandlerWithLocalManaged(pool, caps *pgxpool.Pool, logger *slog.Logger, signer appservice.IntegrationSigner, managedSigner appservice.ManagedShippingSigner, local LocalManaged, connections ...*oauth.Service) (http.Handler, error) {
	if !local.Valid() || managedSigner == nil {
		return nil, errors.New("incomplete local managed engine configuration")
	}
	base := internalServices(pool, caps, logger, signer, managedSigner, nil, local.Readiness, connections...)
	source := managedInstallSource{apprepo.Postgres{Pool: caps}, base.Testing.Managed}
	base.Intents.Managed = source
	base.Intents.EmbeddedPilot = local.EmbeddedPilot
	base.Lifecycle.Intents = base.Intents
	base.Testing.ManagedInstallEnabled = true
	base.EngineKey = local.EngineKey
	if local.ReviewedUI != nil {
		if err := EnableReviewedUI(&base, local.ReviewedUI.Source, local.ReviewedUI.Key); err != nil {
			return nil, err
		}
	}
	base.EngineCheck = func(ctx context.Context, merchant, provider, operation string) (domain.Access, error) {
		var result domain.Access
		if provider != "emisell" || (operation != "rates.read" && operation != "settings.read") {
			return result, fault.Forbidden
		}
		repo := installrepo.Repository{Pool: pool}
		target, err := repo.ManagedTarget(ctx, merchant, provider)
		if err != nil {
			return result, err
		}
		err = source.WithRelease(ctx, merchant, target.Release.AppID, target.Release.Version, func(release domain.IntentRelease) error {
			if installservice.RequestHash(release) != installservice.RequestHash(target.Release) {
				return fault.Forbidden
			}
			var e error
			result, e = repo.WithManagedAccess(ctx, merchant, target.Installation.ID, release)
			return e
		})
		return result, err
	}
	return base.Handler(), nil
}
