package repository

import (
	"context"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/platform/fault"
)

// Release first, then assignment: same lock order as an assignment approval.
// A separate gate pool keeps these locks until the installation transaction ends.
func (p Postgres) WithManagedAssignment(ctx context.Context, merchant, app, version string, fn func(service.Assignment, service.ManagedShippingRelease) error) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT r.id FROM platform_app.managed_shipping_releases r
 WHERE r.manifest->>'appId'=$1 AND r.manifest->>'version'=$2
 AND EXISTS(SELECT 1 FROM platform_app.test_assignments a WHERE a.managed_release_id=r.id AND a.merchant_id=$3 AND a.status='approved')
 ORDER BY r.id LIMIT 2 FOR SHARE OF r`, app, version, merchant)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(ids) != 1 {
		return fault.Forbidden
	}
	a, err := scanAssignment(tx.QueryRow(ctx, `SELECT `+assignmentJSON+` FROM platform_app.test_assignments WHERE managed_release_id=$1 AND merchant_id=$2 AND status='approved' FOR SHARE`, ids[0], merchant))
	if err != nil {
		return err
	}
	r, err := scanManagedShipping(tx.QueryRow(ctx, `SELECT `+managed_shippingColumns+` FROM platform_app.managed_shipping_releases WHERE id=$1 AND organization_id=$2`, a.ReleaseID, a.OrganizationID))
	if err != nil {
		return err
	}
	if err = fn(a, r); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
