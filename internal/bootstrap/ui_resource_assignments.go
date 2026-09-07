package bootstrap

import (
	"context"
	"emisell.app/platform/internal/platform/fault"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresResourceAssignments reads only explicit approved distribution rows.
// The source subsequently validates their signed release and current client.
type PostgresResourceAssignments struct{ Pool *pgxpool.Pool }

func (s PostgresResourceAssignments) WithApproved(ctx context.Context, merchant, app, version string, fn func(string, string, string) error) error {
	if s.Pool == nil {
		return fault.Unavailable
	}
	if merchant == "" || app == "" || version == "" || fn == nil {
		return fault.Invalid
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var org, release, client string
	err = tx.QueryRow(ctx, `SELECT organization_id,release_id,client_id FROM platform_app.ui_resource_assignments WHERE merchant_id=$1 AND app_id=$2 AND version=$3 AND status='approved' FOR SHARE`, merchant, app, version).Scan(&org, &release, &client)
	if errors.Is(err, pgx.ErrNoRows) {
		return fault.NotFound
	}
	if err != nil {
		return err
	}
	if err = fn(org, release, client); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
