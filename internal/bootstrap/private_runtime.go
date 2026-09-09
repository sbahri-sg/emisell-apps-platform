package bootstrap

import (
	"context"
	"crypto/ed25519"
	"log/slog"
	"net/http"

	apprepo "emisell.app/platform/internal/app/repository"
	appservice "emisell.app/platform/internal/app/service"
	clientrepo "emisell.app/platform/internal/oauth/appclient/postgres"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/localfiles"
	"emisell.app/platform/internal/resourceclient"
	"emisell.app/platform/internal/transport/connectapi"
	"emisell.app/platform/pkg/appmanifest"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Production private apps reuse the signed release/consent/grant lifecycle, not
// the embedded pilot. They cannot enable reviewed UI, shipping or fixture apps.
func InternalHandlerWithPrivateProducts(pool, caps, clients *pgxpool.Pool, logger *slog.Logger, integration appservice.IntegrationSigner, managed appservice.ManagedShippingSigner, key ed25519.PrivateKey, products *resourceclient.Products) (http.Handler, error) {
	base := internalServices(pool, caps, logger, integration, managed, nil, nil)
	if err := EnablePrivateProducts(&base, pool, caps, clients, key, products); err != nil {
		return nil, err
	}
	return base.Handler(), nil
}

func EnablePrivateProducts(base *connectapi.Server, pool, caps, clients *pgxpool.Pool, key ed25519.PrivateKey, products *resourceclient.Products) error {
	if base == nil || pool == nil || caps == nil || clients == nil || len(key) != ed25519.PrivateKeySize || products == nil {
		return fault.Invalid
	}
	if _, err := localfiles.ReadApplicationCredentialBox(); err != nil {
		return fault.Unavailable
	}
	source := ReviewedUIInstallSource{Releases: apprepo.Postgres{Pool: caps}, Clients: clientrepo.Repository{Pool: clients}, ResourceKey: key, Products: products}
	// No fallback to the fixture registry or another source in this composition.
	base.Intents.Apps = noFixtureRegistry{}
	base.Intents.Managed = nil
	base.ResourceProducts = products
	enablePrivateProducts(base, source)
	return nil
}

type noFixtureRegistry struct{}

func (noFixtureRegistry) Get(context.Context, string) (appmanifest.Manifest, error) {
	return appmanifest.Manifest{}, fault.NotFound
}
func (noFixtureRegistry) List(context.Context) ([]appmanifest.Manifest, error) {
	return []appmanifest.Manifest{}, nil
}
