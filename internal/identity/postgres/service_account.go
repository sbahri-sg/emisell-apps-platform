package postgres

import (
	"context"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/platform/fault"
	"errors"
	"github.com/jackc/pgx/v5"
)

func (p Repository) FindService(ctx context.Context, hash string) (identity.ServicePrincipal, error) {
	var v identity.ServicePrincipal
	err := p.Pool.QueryRow(ctx, `SELECT id,tenant_id,scopes,expires_at FROM platform_identity.service_accounts WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at>now()`, hash).Scan(&v.ID, &v.TenantID, &v.Scopes, &v.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = fault.Unauthenticated
	}
	return v, err
}
func (p Repository) ServiceAllowed(ctx context.Context, id, tenant string) (bool, error) {
	var ok bool
	err := p.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM platform_identity.service_accounts WHERE id=$1 AND tenant_id=$2 AND revoked_at IS NULL AND expires_at>now())`, id, tenant).Scan(&ok)
	return ok, err
}
func (p Repository) PutService(ctx context.Context, v identity.ServicePrincipal, hash string, rotate bool, actor string) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	action := "issued"
	if rotate {
		tag, e := tx.Exec(ctx, `UPDATE platform_identity.service_accounts SET token_hash=$1,scopes=$2,expires_at=$3,revoked_at=NULL WHERE id=$4 AND tenant_id=$5`, hash, v.Scopes, v.ExpiresAt, v.ID, v.TenantID)
		err = e
		if err == nil && tag.RowsAffected() != 1 {
			return fault.NotFound
		}
		action = "rotated"
	} else {
		tag, e := tx.Exec(ctx, `INSERT INTO platform_identity.service_accounts(id,tenant_id,token_hash,scopes,expires_at) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, v.ID, v.TenantID, hash, v.Scopes, v.ExpiresAt)
		err = e
		if err == nil && tag.RowsAffected() != 1 {
			return fault.Conflict
		}
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_identity.service_account_audit(service_id,tenant_id,action,actor) VALUES($1,$2,$3,$4)`, v.ID, v.TenantID, action, actor); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (p Repository) RevokeService(ctx context.Context, id, actor string) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var tenant string
	err = tx.QueryRow(ctx, `UPDATE platform_identity.service_accounts SET revoked_at=now() WHERE id=$1 AND revoked_at IS NULL RETURNING tenant_id`, id).Scan(&tenant)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_identity.service_account_audit(service_id,tenant_id,action,actor) VALUES($1,$2,'revoked',$3)`, id, tenant, actor); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
