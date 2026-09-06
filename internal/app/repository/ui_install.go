package repository

import (
	"context"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/platform/fault"
)

func (p Postgres) HasUIApp(ctx context.Context, app string) (bool, error) {
	var found bool
	err := p.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM platform_app.ui_releases WHERE app_id=$1)`, app).Scan(&found)
	return found, err
}

// Keep release then assignment locked through the installation transaction.
// An absent approved assignment is denial, never permission to try another source.
func (p Postgres) WithUIAssignment(ctx context.Context, merchant, app, version string, fn func(service.Assignment, service.UIRelease) error) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	r, err := scanUI(tx.QueryRow(ctx, `SELECT document FROM platform_app.ui_releases
 WHERE app_id=$1 AND version=$2 FOR SHARE`, app, version))
	if err != nil {
		return err
	}
	a, err := scanAssignment(tx.QueryRow(ctx, `SELECT `+assignmentJSON+` FROM platform_app.test_assignments
 WHERE ui_release_id=$1 AND merchant_id=$2 AND status='approved' FOR SHARE`, r.ID, merchant))
	if err != nil {
		return err
	}
	if a.OrganizationID != r.Manifest.DeveloperID || a.ReleaseKind != "ui" || a.Status != "approved" {
		return fault.Forbidden
	}
	if err = fn(a, r); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
