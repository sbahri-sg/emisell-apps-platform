package bootstrap

import (
	"context"
	"emisell.app/platform/internal/oauth/appidentity"
	"emisell.app/platform/internal/platform/localfiles"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Explicit upgrade of existing drafts, without rotating legacy clients or existing identities.
func BackfillApplicationCredentials(ctx context.Context, pool *pgxpool.Pool) error {
	box, err := localfiles.ReadApplicationCredentialBox()
	if err != nil {
		return err
	}
	repo := appidentity.Repository{Pool: pool, Box: box}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT organization_id,id FROM platform_app.drafts ORDER BY organization_id,id`)
	if err != nil {
		return err
	}
	type app struct{ org, id string }
	apps := []app{}
	for rows.Next() {
		var a app
		if err = rows.Scan(&a.org, &a.id); err != nil {
			rows.Close()
			return err
		}
		apps = append(apps, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, a := range apps {
		if err = repo.EnsureTx(ctx, tx, a.org, a.id, "system-credential-upgrade"); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
