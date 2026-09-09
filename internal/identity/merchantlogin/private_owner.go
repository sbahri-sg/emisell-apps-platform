package merchantlogin

import (
	"context"
	"emisell.app/platform/internal/platform/fault"
	"github.com/jackc/pgx/v5"
)

// Core has freshly verified membership/apps permission for the selected store.
// Compare its actor assertion with the app owner's linked Core identity. The
// cached stores JSON is display-only and is never used as an authorization list.
func (p Repository) WithCoreOwner(ctx context.Context, account, subject string, fn func(pgx.Tx) error) error {
	if account == "" || subject == "" || fn == nil {
		return fault.Forbidden
	}
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var linked string
	err = tx.QueryRow(ctx, `SELECT l.core_subject FROM platform_identity.developer_core_links l JOIN platform_identity.portal_accounts a ON a.id=l.account_id WHERE l.account_id=$1 AND a.enabled AND a.surface='developer' AND a.role='developer' FOR SHARE OF l,a`, account).Scan(&linked)
	if err != nil || linked != subject {
		return fault.Forbidden
	}
	return fn(tx)
}
