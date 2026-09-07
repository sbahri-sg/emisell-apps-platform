package bootstrap

import (
	"crypto/ed25519"
	apprepo "emisell.app/platform/internal/app/repository"
	appservice "emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/developer"
	developerrepo "emisell.app/platform/internal/developer/postgres"
	"emisell.app/platform/internal/identity"
	identityrepo "emisell.app/platform/internal/identity/postgres"
	"emisell.app/platform/internal/transport/httpapi"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"net/http"
	"strings"
)

// AddUIResourceReleaseRoutes preserves every existing route and pilot configuration.
func AddUIResourceReleaseRoutes(base http.Handler, pool *pgxpool.Pool, origin string, logger *slog.Logger, key ed25519.PrivateKey) http.Handler {
	ui := UIResourceReleaseHandler(pool, origin, logger, key)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, prefix := range []string{"/api/v1/admin/ui-resource-releases", "/api/v1/developer/ui-resource-releases"} {
			if r.URL.Path == prefix || strings.HasPrefix(r.URL.Path, prefix+"/") {
				ui.ServeHTTP(w, r)
				return
			}
		}
		base.ServeHTTP(w, r)
	})
}

// UIResourceReleaseHandler is an opt-in portal API composition. It is not mounted by
// the default server. Nil signing key still permits rejection and suspension.
func UIResourceReleaseHandler(pool *pgxpool.Pool, origin string, logger *slog.Logger, key ed25519.PrivateKey) http.Handler {
	developers := developer.Service{Repo: developerrepo.Repository{Pool: pool}}
	return httpapi.Server{Origin: origin, Logger: logger,
		Portals: identity.Portals{Repo: identityrepo.Repository{Pool: pool}}, Developers: developers,
		UIResourceReleases: appservice.UIResourceReleases{Repo: apprepo.Postgres{Pool: pool}, Developers: developers, Key: key},
	}.Handler()
}
