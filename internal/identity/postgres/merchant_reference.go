package postgres

import (
	"context"
	"errors"

	"emisell.app/platform/internal/platform/fault"
	"github.com/jackc/pgx/v5"
)

// Reference-only registration: no updates to existing names, memberships,
// app grants or installation state. Concurrent retries converge on one row.
func (p Repository) EnsureMerchantReference(ctx context.Context, serviceID, merchantID, actorID string) (bool, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var activeID string
	// Serialize with key revocation, including revocation after HTTP auth.
	err = tx.QueryRow(ctx, `SELECT id FROM platform_identity.core_platform_keys WHERE id=$1 AND revoked_at IS NULL FOR SHARE`, serviceID).Scan(&activeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, fault.Unauthenticated
	}
	if err != nil {
		return false, err
	}
	result, err := tx.Exec(ctx, `INSERT INTO platform_identity.workspaces(id,name) VALUES($1,'Merchant ' || left($1,51)) ON CONFLICT(id) DO NOTHING`, merchantID)
	if err != nil {
		return false, err
	}
	created := result.RowsAffected() == 1
	if created {
		_, err = tx.Exec(ctx, `INSERT INTO platform_identity.merchant_reference_audit(merchant_id,service_id,core_actor_id) VALUES($1,$2,$3)`, merchantID, activeID, actorID)
		if err != nil {
			return false, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return created, nil
}
