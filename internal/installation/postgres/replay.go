package postgres

import (
	"context"
	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/internal/platform/fault"
	"errors"
	"github.com/jackc/pgx/v5"
)

// Current receipts remain readable during source/signer outages. This never
// performs a mutation; missing receipts still take the full locked release path.
func (p Repository) ReplayAccess(ctx context.Context, o domain.IntentOwner, key, hash string) (*domain.AccessResult, error) {
	tx, e := p.Pool.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	if e = lock(ctx, tx, o.TenantID); e != nil {
		return nil, e
	}
	r, found, e := receipt(ctx, tx, o, key, hash)
	if e != nil || !found {
		return nil, e
	}
	return &r, nil
}
func (p Repository) ReplayIntent(ctx context.Context, o domain.IntentOwner, key, hash string) (*domain.InstallIntent, error) {
	var oldHash, id string
	e := p.Pool.QueryRow(ctx, `SELECT request_hash,intent_id FROM platform_installation.intent_requests WHERE tenant_id=$1 AND service_id=$2 AND actor_id=$3 AND request_key=$4`, o.TenantID, o.ServiceID, o.ActorID, key).Scan(&oldHash, &id)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	if oldHash != hash {
		return nil, fault.Conflict
	}
	v, e := p.GetIntent(ctx, o, id)
	return &v, e
}
