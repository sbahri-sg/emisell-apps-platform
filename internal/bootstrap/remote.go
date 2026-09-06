package bootstrap

import (
	apprepo "emisell.app/platform/internal/app/repository"
	appservice "emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/identity"
	identityrepo "emisell.app/platform/internal/identity/postgres"
	installrepo "emisell.app/platform/internal/installation/postgres"
	"emisell.app/platform/internal/oauth"
	oauthrepo "emisell.app/platform/internal/oauth/postgres"
	"emisell.app/platform/internal/platform/localfiles"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Connections uses its own pool: lifecycle/capability transactions retain leases.
func Connections(lifecyclePool, connectionPool *pgxpool.Pool, cfg localfiles.RemoteConfig) (*oauth.Service, error) {
	return oauth.New(oauthrepo.Repository{Pool: connectionPool}, installrepo.Repository{Pool: lifecyclePool}, appservice.Registry{Repo: apprepo.Postgres{Pool: connectionPool}}, identity.Service{Repo: identityrepo.Repository{Pool: connectionPool}}, cfg)
}
