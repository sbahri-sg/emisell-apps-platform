package bootstrap

import (
	"context"
	"crypto/ed25519"
	apprepo "emisell.app/platform/internal/app/repository"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/installation/domain"
	installservice "emisell.app/platform/internal/installation/service"
	"emisell.app/platform/internal/oauth"
	"emisell.app/platform/internal/oauth/appclient"
	clientrepo "emisell.app/platform/internal/oauth/appclient/postgres"
	"emisell.app/platform/internal/oauth/embedded"
	launchrepo "emisell.app/platform/internal/oauth/embedded/postgres"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/resourceclient"
	"emisell.app/platform/internal/transport/connectapi"
	"emisell.app/platform/pkg/uirelease"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"net/http"
	"net/url"
)

type ReviewedUIRuntime struct {
	Source ReviewedUIInstallSource
	Key    ed25519.PrivateKey
}

func NewReviewedUIRuntime(releases, clients *pgxpool.Pool, key, launchKey ed25519.PrivateKey, parent string) (*ReviewedUIRuntime, error) {
	if releases == nil || clients == nil || len(key) != ed25519.PrivateKeySize || len(launchKey) != ed25519.PrivateKeySize {
		return nil, fault.Invalid
	}
	u, err := url.Parse(parent)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, fault.Invalid
	}
	s := ReviewedUIInstallSource{Releases: apprepo.Postgres{Pool: releases}, Clients: clientrepo.Repository{Pool: clients}, ReleaseKey: key.Public().(ed25519.PublicKey), Current: embedded.CurrentBinding{Clients: appclient.Service{Repo: clientrepo.Repository{Pool: clients}}, Launches: launchrepo.Repository{Pool: clients}, Key: launchKey.Public().(ed25519.PublicKey), ParentOrigin: parent}}
	return &ReviewedUIRuntime{s, key}, nil
}
func InternalHandlerWithReviewedUI(pool, caps *pgxpool.Pool, logger *slog.Logger, signer service.IntegrationSigner, managed service.ManagedShippingSigner, runtime ReviewedUIRuntime, connections ...*oauth.Service) (http.Handler, error) {
	base := internalServices(pool, caps, logger, signer, managed, nil, nil, connections...)
	if err := EnableReviewedUI(&base, runtime.Source, runtime.Key); err != nil {
		return nil, err
	}
	return base.Handler(), nil
}

// ReviewedUIInstallSource is an explicit composition, not a catalog adapter.
// Use separate pools for release/assignment, client/launch and installation locks.
// Callers must keep the callback bounded; no network requests inside it.
type ReviewedUIInstallSource struct {
	Releases    apprepo.Postgres
	Clients     clientrepo.Repository
	Current     embedded.CurrentBinding
	ReleaseKey  ed25519.PublicKey
	ResourceKey ed25519.PrivateKey
	Products    *resourceclient.Products
}

type uiAndExistingSources struct {
	UI       ReviewedUIInstallSource
	Existing installservice.ManagedReleases
}

func (s uiAndExistingSources) WithRelease(ctx context.Context, merchant, app, version string, fn func(domain.IntentRelease) error) error {
	resource, err := s.UI.Releases.HasResourceUIApp(ctx, app)
	if err != nil {
		return err
	}
	if resource {
		return s.UI.WithResourceRelease(ctx, merchant, app, version, fn)
	}
	found, err := s.UI.Releases.HasUIApp(ctx, app)
	if err != nil {
		return err
	}
	if found {
		return s.UI.WithRelease(ctx, merchant, app, version, fn)
	}
	if s.Existing == nil {
		return fault.NotFound
	}
	return s.Existing.WithRelease(ctx, merchant, app, version, fn)
}

// EnableReviewedUI preserves any managed-shipping source. An owned UI app never
// falls back to that source on denial. Call only with explicit operator keys.
func EnableReviewedUI(base *connectapi.Server, source ReviewedUIInstallSource, key ed25519.PrivateKey) error {
	if base == nil || len(key) != ed25519.PrivateKeySize || !key.Public().(ed25519.PublicKey).Equal(source.ReleaseKey) || len(source.Current.Key) != ed25519.PublicKeySize || source.Current.ParentOrigin == "" {
		return fault.Invalid
	}
	base.Intents.Managed = uiAndExistingSources{source, base.Intents.Managed}
	base.Lifecycle.Intents = base.Intents
	base.Lifecycle.ReviewedUIKey = source.Current.Key
	base.Testing.UI = service.UIReleases{Repo: source.Releases, Key: key}
	if len(source.ResourceKey) == ed25519.PrivateKeySize {
		base.Testing.Resources = service.UIResourceReleases{Repo: source.Releases, Key: source.ResourceKey}
		if source.Products != nil {
			base.Lifecycle.Resources = source
			base.ResourceProducts = source.Products
			base.ResourceClients = appclient.Service{Repo: source.Clients, Releases: clientReleases{Resources: base.Testing.Resources}}
			base.Testing.ResourceReady = func(ctx context.Context, a service.Assignment) error {
				v, err := source.Releases.UIResourceGet(ctx, a.OrganizationID, a.ReleaseID)
				if err != nil {
					return err
				}
				return source.WithResourceRelease(ctx, a.MerchantID, v.Manifest.UI.AppID, v.Manifest.UI.Version, func(r domain.IntentRelease) error {
					if r.ManifestDigest != a.ReleaseSHA256 || r.UIBinding.AssignmentID != a.ID {
						return fault.Forbidden
					}
					return nil
				})
			}
		}
	}
	base.Testing.UIReady = func(ctx context.Context, a service.Assignment) error {
		v, err := source.Releases.UIGet(ctx, a.OrganizationID, a.ReleaseID)
		if err != nil {
			return err
		}
		return source.WithRelease(ctx, a.MerchantID, v.Manifest.AppID, v.Manifest.Version, func(r domain.IntentRelease) error {
			if r.UIBinding.AssignmentID != a.ID || r.ManifestDigest != a.ReleaseSHA256 {
				return fault.Forbidden
			}
			return nil
		})
	}
	if len(source.ResourceKey) == ed25519.PrivateKeySize && source.Products != nil {
		enablePrivateProducts(base, source)
	}
	return nil
}

func (s ReviewedUIInstallSource) WithRelease(ctx context.Context, merchant, app, version string, fn func(domain.IntentRelease) error) error {
	if fn == nil || len(s.ReleaseKey) != ed25519.PublicKeySize {
		return fault.Unavailable
	}
	return s.Releases.WithUIAssignment(ctx, merchant, app, version, func(a service.Assignment, v service.UIRelease) error {
		if v.Status != "signed" || v.Package == nil || v.Manifest != v.Package.Manifest ||
			uirelease.Verify(*v.Package, s.ReleaseKey) != nil || a.ReleaseSHA256 != v.Package.SHA256 {
			return fault.Forbidden
		}
		m := v.Manifest
		c, err := s.Clients.ForRelease(ctx, m.DeveloperID, v.ID)
		if err != nil {
			return err
		}
		expected := appclient.Binding{ReleaseID: v.ID, OrganizationID: m.DeveloperID, AppID: m.AppID, Version: m.Version, Name: m.Name, Digest: v.Package.SHA256, Endpoint: m.URL}
		return s.Current.WithBinding(ctx, c.ID, expected, func(b embedded.Binding) error {
			// Review cannot replace the URL/mode that was signed in the UI release.
			mode := b.Launch.Mode
			if mode == "" {
				mode = "embedded"
			}
			if b.Launch.URL != m.URL || mode != m.Mode {
				return fault.Forbidden
			}
			return fn(domain.IntentRelease{AppID: m.AppID, Name: m.Name, DeveloperID: m.DeveloperID, Version: m.Version,
				ManifestDigest: v.Package.SHA256, InstallPolicy: domain.ReviewedUIPolicy, ExecutionProfile: domain.ReviewedUIPolicy,
				Scopes: []string{}, Capabilities: []string{}, UIBinding: &domain.UIBinding{AssignmentID: a.ID, ReleaseID: v.ID, Launch: b.Launch, Signature: b.Signature}})
		})
	})
}
