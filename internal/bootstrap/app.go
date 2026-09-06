package bootstrap

import (
	"context"
	"crypto/ed25519"
	apprepo "emisell.app/platform/internal/app/repository"
	appservice "emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/capability"
	caprepo "emisell.app/platform/internal/capability/postgres"
	"emisell.app/platform/internal/developer"
	developerrepo "emisell.app/platform/internal/developer/postgres"
	"emisell.app/platform/internal/identity"
	identityrepo "emisell.app/platform/internal/identity/postgres"
	installrepo "emisell.app/platform/internal/installation/postgres"
	installservice "emisell.app/platform/internal/installation/service"
	"emisell.app/platform/internal/oauth"
	"emisell.app/platform/internal/oauth/appclient"
	clientrepo "emisell.app/platform/internal/oauth/appclient/postgres"
	"emisell.app/platform/internal/oauth/embedded"
	embeddedrepo "emisell.app/platform/internal/oauth/embedded/postgres"
	"emisell.app/platform/internal/platform/localfiles"
	"emisell.app/platform/internal/review"
	reviewrepo "emisell.app/platform/internal/review/postgres"
	"emisell.app/platform/internal/runtime/remote"
	"emisell.app/platform/internal/runtime/simulator"
	"emisell.app/platform/internal/transport/connectapi"
	"emisell.app/platform/internal/transport/httpapi"
	"emisell.app/platform/internal/webhook"
	webhookrepo "emisell.app/platform/internal/webhook/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"net/http"
)

// The capability pool is separate: gated invocations retain a lifecycle
// connection, and must not starve waiting for another lease from the same pool.
func Handler(pool, capabilityPool *pgxpool.Pool, origin string, logger *slog.Logger, connections ...*oauth.Service) http.Handler {
	return HandlerWithCatalog(pool, capabilityPool, origin, logger, nil, connections...)
}

func HandlerWithCatalog(pool, capabilityPool *pgxpool.Pool, origin string, logger *slog.Logger, signer appservice.CatalogSigner, connections ...*oauth.Service) http.Handler {
	return HandlerWithReleases(pool, capabilityPool, origin, logger, signer, nil, connections...)
}

func HandlerWithReleases(pool, capabilityPool *pgxpool.Pool, origin string, logger *slog.Logger, signer appservice.CatalogSigner, integrationSigner appservice.IntegrationSigner, connections ...*oauth.Service) http.Handler {
	return HandlerWithAppClients(pool, capabilityPool, nil, origin, logger, signer, integrationSigner, nil, connections...)
}

func HandlerWithAppClients(pool, capabilityPool, clientPool *pgxpool.Pool, origin string, logger *slog.Logger, signer appservice.CatalogSigner, integrationSigner appservice.IntegrationSigner, verifier appclient.Verifier, connections ...*oauth.Service) http.Handler {
	return HandlerWithManagedShipping(pool, capabilityPool, clientPool, origin, logger, signer, integrationSigner, nil, verifier, connections...)
}

func HandlerWithManagedShipping(pool, capabilityPool, clientPool *pgxpool.Pool, origin string, logger *slog.Logger, signer appservice.CatalogSigner, integrationSigner appservice.IntegrationSigner, managedSigner appservice.ManagedShippingSigner, verifier appclient.Verifier, connections ...*oauth.Service) http.Handler {
	return handlerWithManagedShipping(pool, capabilityPool, clientPool, origin, logger, signer, integrationSigner, managedSigner, verifier, false, nil, connections...)
}
func HandlerWithLocalManagedShipping(pool, capabilityPool, clientPool *pgxpool.Pool, origin string, logger *slog.Logger, signer appservice.CatalogSigner, integrationSigner appservice.IntegrationSigner, managedSigner appservice.ManagedShippingSigner, verifier appclient.Verifier, connections ...*oauth.Service) http.Handler {
	return handlerWithManagedShipping(pool, capabilityPool, clientPool, origin, logger, signer, integrationSigner, managedSigner, verifier, true, nil, connections...)
}

type EmbeddedReviewConfig struct {
	Runtime      *ReviewedUIRuntime
	UIReleaseKey ed25519.PrivateKey
	Key          ed25519.PrivateKey
	ParentOrigin string
}

// Explicit composition: no auto-generated signing keys or implicit origin.
func HandlerWithEmbeddedReviews(pool, capabilityPool, clientPool *pgxpool.Pool, origin string, logger *slog.Logger, integrationSigner appservice.IntegrationSigner, verifier appclient.Verifier, config EmbeddedReviewConfig) http.Handler {
	return handlerWithManagedShipping(pool, capabilityPool, clientPool, origin, logger, nil, integrationSigner, nil, verifier, false, &config)
}

func HandlerWithReviewedUIRuntime(pool, caps, clients *pgxpool.Pool, origin string, logger *slog.Logger, catalog appservice.CatalogSigner, integration appservice.IntegrationSigner, managed appservice.ManagedShippingSigner, verifier appclient.Verifier, localManaged bool, config EmbeddedReviewConfig, connections ...*oauth.Service) http.Handler {
	return handlerWithManagedShipping(pool, caps, clients, origin, logger, catalog, integration, managed, verifier, localManaged, &config, connections...)
}

func handlerWithManagedShipping(pool, capabilityPool, clientPool *pgxpool.Pool, origin string, logger *slog.Logger, signer appservice.CatalogSigner, integrationSigner appservice.IntegrationSigner, managedSigner appservice.ManagedShippingSigner, verifier appclient.Verifier, managedInstallEnabled bool, embeddedConfig *EmbeddedReviewConfig, connections ...*oauth.Service) http.Handler {
	auth := identity.Service{Repo: identityrepo.Repository{Pool: pool}}
	portals := identity.Portals{Repo: identityrepo.Repository{Pool: pool}}
	developers := developer.Service{Repo: developerrepo.Repository{Pool: pool}}
	drafts := appservice.Drafts{Repo: apprepo.Postgres{Pool: pool}, Developers: developers}
	reviews := review.Service{Repo: reviewrepo.Repository{Pool: pool}, Drafts: drafts, Developers: developers}
	catalog := appservice.Catalog{Repo: apprepo.Postgres{Pool: pool}, Source: reviews, Signer: signer, Developers: developers}
	integrations := appservice.Integrations{Repo: apprepo.Postgres{Pool: pool}, Source: reviews, Signer: integrationSigner, Developers: developers}
	clientReleaseGate := integrations
	clientReleaseGate.Repo = apprepo.Postgres{Pool: capabilityPool}
	uiKey, _ := localfiles.ReadUIReleaseKey() // main validates malformed local configuration before serving
	if embeddedConfig != nil && embeddedConfig.UIReleaseKey != nil {
		uiKey = embeddedConfig.UIReleaseKey
	}
	uiGate := appservice.UIReleases{Repo: apprepo.Postgres{Pool: capabilityPool}, Developers: developers, Key: uiKey}
	testing := appservice.Testing{Repo: apprepo.Postgres{Pool: pool}, Releases: clientReleaseGate, Merchants: identity.MerchantDirectory{Repo: identityrepo.Repository{Pool: pool}}}
	testing.UI = uiGate
	// Dedicated client pool prevents cap→main and main→cap lease inversion.
	var appClients appclient.Service
	if clientPool != nil {
		appClients = appclient.Service{Repo: clientrepo.Repository{Pool: clientPool}, Releases: clientReleases{Integrations: clientReleaseGate, UI: uiGate}, Developers: developers, Verifier: verifier}
	}
	registry := appservice.Registry{Repo: apprepo.Postgres{Pool: pool}}
	installs := installservice.Service{Repo: installrepo.Repository{Pool: pool}, Apps: registry, Auth: auth}
	caps := capability.Service{Installations: installs, Repo: caprepo.Repository{Pool: capabilityPool}, Runtime: simulator.Runtime{}}
	var connection *oauth.Service
	if len(connections) > 0 && connections[0] != nil {
		connection = connections[0]
		installs.Connections = connection
		caps.Installations = installs
		caps.Remote = remote.Client{Connections: connection}
	}
	webhooks := webhook.Monitor{Repo: webhookrepo.Repository{Pool: capabilityPool}, Auth: auth, Gate: installrepo.Repository{Pool: pool}, Apps: appservice.Registry{Repo: apprepo.Postgres{Pool: capabilityPool}}}
	monitor := installservice.Monitor{Repo: installrepo.Repository{Pool: pool}, Auth: auth, Apps: registry, Connections: installs.Connections}
	payments := capability.Payments{Repo: caprepo.Repository{Pool: capabilityPool}, Auth: auth, Gate: installrepo.Repository{Pool: pool}, Apps: appservice.Registry{Repo: apprepo.Postgres{Pool: capabilityPool}}}
	if connection != nil {
		payments.Connections = connection
	}
	appAccess := installservice.Lifecycle{Repo: installrepo.Repository{Pool: pool}, Intents: installservice.Intents{Apps: appservice.Registry{Repo: apprepo.Postgres{Pool: capabilityPool}}}}
	managed := appservice.ManagedShipping{Repo: apprepo.Postgres{Pool: pool}, Drafts: apprepo.Postgres{Pool: pool}, Developers: developers, Signer: managedSigner}
	testing.Managed = managed
	testing.ManagedInstallEnabled = managedInstallEnabled
	testing.Managed.Repo = apprepo.Postgres{Pool: capabilityPool}
	var launchReviews embedded.Reviews
	if embeddedConfig != nil && clientPool != nil {
		launchReviews = embedded.Reviews{Repo: embeddedrepo.Repository{Pool: pool}, Clients: appClients, Key: embeddedConfig.Key, ParentOrigin: embeddedConfig.ParentOrigin}
		if embeddedConfig.Runtime != nil {
			temp := connectapi.Server{Testing: testing}
			if EnableReviewedUI(&temp, embeddedConfig.Runtime.Source, embeddedConfig.Runtime.Key) == nil {
				testing = temp.Testing
			}
		}
	}
	return httpapi.Server{EmbeddedLaunches: launchReviews, ManagedShipping: managed, Testing: testing, AppClients: appClients, Integrations: integrations, AppAccess: appAccess, PlatformKeys: identity.PlatformKeys{Repo: identityrepo.Repository{Pool: pool}}, ManagedKeys: identity.ManagedKeys{Repo: identityrepo.Repository{Pool: pool}}, Catalog: catalog, Portals: portals, Developers: developers, Drafts: drafts, Reviews: reviews, Identity: auth, Apps: registry, Installations: installs, Capabilities: caps, OAuth: connection, Webhooks: webhooks, Connections: monitor, Payments: payments, Origin: origin, Logger: logger, Ready: func(ctx context.Context) error { return pool.Ping(ctx) }}.Handler()
}

// InternalHandler shares domain/use cases but uses service accounts, not browser sessions.
func InternalHandler(pool, capabilityPool *pgxpool.Pool, logger *slog.Logger, connections ...*oauth.Service) http.Handler {
	return InternalHandlerWithReleases(pool, capabilityPool, logger, nil, connections...)
}

func InternalHandlerWithReleases(pool, capabilityPool *pgxpool.Pool, logger *slog.Logger, signer appservice.IntegrationSigner, connections ...*oauth.Service) http.Handler {
	return internalHandler(pool, capabilityPool, logger, signer, nil, nil, connections...)
}

func InternalHandlerWithManagedReleases(pool, capabilityPool *pgxpool.Pool, logger *slog.Logger, signer appservice.IntegrationSigner, managedSigner appservice.ManagedShippingSigner, connections ...*oauth.Service) http.Handler {
	return internalHandlerWithManaged(pool, capabilityPool, logger, signer, managedSigner, nil, nil, connections...)
}

// InternalHandlerWithLocalShipping is an explicit local test composition. Normal
// server startup does not register this bridge or seed the reference release.
func InternalHandlerWithLocalShipping(pool, capabilityPool *pgxpool.Pool, logger *slog.Logger, shipping capability.LocalShippingRuntime) http.Handler {
	return internalHandler(pool, capabilityPool, logger, nil, shipping, nil)
}

// InternalHandlerWithLocalShippingProviders enables only provider-bound lifecycle
// tests. No normal server or public catalog uses this composition.
func InternalHandlerWithLocalShippingProviders(pool, capabilityPool *pgxpool.Pool, logger *slog.Logger, providers installservice.ShippingProviderReadiness) http.Handler {
	return internalHandler(pool, capabilityPool, logger, nil, nil, providers)
}

func internalHandler(pool, capabilityPool *pgxpool.Pool, logger *slog.Logger, signer appservice.IntegrationSigner, shipping capability.LocalShippingRuntime, providers installservice.ShippingProviderReadiness, connections ...*oauth.Service) http.Handler {
	return internalHandlerWithManaged(pool, capabilityPool, logger, signer, nil, shipping, providers, connections...)
}

func internalHandlerWithManaged(pool, capabilityPool *pgxpool.Pool, logger *slog.Logger, signer appservice.IntegrationSigner, managedSigner appservice.ManagedShippingSigner, shipping capability.LocalShippingRuntime, providers installservice.ShippingProviderReadiness, connections ...*oauth.Service) http.Handler {
	return internalServices(pool, capabilityPool, logger, signer, managedSigner, shipping, providers, connections...).Handler()
}
func internalServices(pool, capabilityPool *pgxpool.Pool, logger *slog.Logger, signer appservice.IntegrationSigner, managedSigner appservice.ManagedShippingSigner, shipping capability.LocalShippingRuntime, providers installservice.ShippingProviderReadiness, connections ...*oauth.Service) connectapi.Server {
	testing := appservice.Testing{Repo: apprepo.Postgres{Pool: pool}, Releases: appservice.Integrations{Repo: apprepo.Postgres{Pool: capabilityPool}, Signer: signer}}
	testing.Managed = appservice.ManagedShipping{Repo: apprepo.Postgres{Pool: capabilityPool}, Signer: managedSigner}
	auth := identity.ServiceAccounts{Repo: identityrepo.Repository{Pool: pool}}
	registry := appservice.Registry{Repo: apprepo.Postgres{Pool: pool}}
	installs := installservice.Service{Repo: installrepo.Repository{Pool: pool}, Apps: registry, Auth: auth}
	caps := capability.Service{Installations: installs, Repo: caprepo.Repository{Pool: capabilityPool}, Runtime: simulator.Runtime{}, Shipping: shipping}
	if len(connections) > 0 && connections[0] != nil {
		caps.Remote = remote.Client{Connections: connections[0]}
	}
	// Intent decisions retain a lifecycle transaction while rechecking the release;
	// use the separate pool so waiters cannot starve that registry read.
	intentRegistry := appservice.Registry{Repo: apprepo.Postgres{Pool: capabilityPool}}
	intents := installservice.Intents{Repo: installrepo.Repository{Pool: pool}, Apps: intentRegistry, Auth: auth}
	lifecycle := installservice.Lifecycle{Repo: installrepo.Repository{Pool: pool}, Intents: intents, Shipping: shipping, ShippingProviders: providers}
	if len(connections) > 0 && connections[0] != nil {
		lifecycle.Connections = connections[0]
	}
	return connectapi.Server{Testing: testing, Accounts: auth, Capabilities: caps, Intents: intents, Lifecycle: lifecycle, Logger: logger}
}
