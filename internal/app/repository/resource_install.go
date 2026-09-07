package repository

import (
	"context"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/platform/fault"
)

func (p Postgres) HasResourceUIApp(ctx context.Context, app string) (bool, error) {
	var found bool
	err := p.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM platform_app.ui_resource_releases WHERE app_id=$1)`, app).Scan(&found)
	return found, err
}

// Release -> assignment -> client/launch -> installation, matching distribution
// approval lock order. Revocation cannot race a successful access callback.
func (p Postgres) WithResourceUIAssignment(ctx context.Context, merchant, app, version string, fn func(service.Assignment, service.UIResourceRelease) error) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	r, err := scanUIResource(tx.QueryRow(ctx, `SELECT document FROM platform_app.ui_resource_releases WHERE app_id=$1 AND version=$2 FOR SHARE`, app, version))
	if err != nil {
		return err
	}
	a, err := scanAssignment(tx.QueryRow(ctx, `SELECT `+assignmentJSON+` FROM platform_app.test_assignments WHERE resource_release_id=$1 AND merchant_id=$2 AND status='approved' FOR SHARE`, r.ID, merchant))
	if err != nil {
		return err
	}
	if a.OrganizationID != r.Manifest.UI.DeveloperID || a.ReleaseKind != "ui_resource" {
		return fault.Forbidden
	}
	if err = fn(a, r); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
