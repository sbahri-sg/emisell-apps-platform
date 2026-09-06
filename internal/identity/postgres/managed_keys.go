package postgres

import (
	"context"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/platform/fault"
	"errors"
	"github.com/jackc/pgx/v5"
)

const managedColumns = `s.id,m.name,s.tenant_id,s.scopes,s.created_at,s.expires_at,s.revoked_at,
 CASE WHEN s.revoked_at IS NOT NULL THEN 'revoked' WHEN s.expires_at<=now() THEN 'expired' ELSE 'active' END`
const managedFrom = ` FROM platform_identity.managed_service_keys m JOIN platform_identity.service_accounts s ON s.id=m.service_id `

func scanManaged(row pgx.Row) (identity.ManagedKey, error) {
	var v identity.ManagedKey
	err := row.Scan(&v.ID, &v.Name, &v.TenantID, &v.Scopes, &v.CreatedAt, &v.ExpiresAt, &v.RevokedAt, &v.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		err = fault.NotFound
	}
	return v, err
}
func (p Repository) ListManagedKeys(ctx context.Context) ([]identity.ManagedKey, error) {
	rows, err := p.Pool.Query(ctx, `SELECT `+managedColumns+managedFrom+`ORDER BY s.created_at DESC,s.id LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []identity.ManagedKey{}
	for rows.Next() {
		v, e := scanManaged(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (p Repository) CreateManagedKey(ctx context.Context, actor, request, fingerprint string, v identity.ManagedKey, hash string) (identity.ManagedKey, bool, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return v, false, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, actor+":"+request); err != nil {
		return v, false, err
	}
	var storedHash, id string
	err = tx.QueryRow(ctx, `SELECT request_hash,service_id FROM platform_identity.managed_service_keys WHERE actor_id=$1 AND request_key=$2`, actor, request).Scan(&storedHash, &id)
	if err == nil {
		if storedHash != fingerprint {
			return v, false, fault.Conflict
		}
		out, e := scanManaged(tx.QueryRow(ctx, `SELECT `+managedColumns+managedFrom+`WHERE s.id=$1`, id))
		return out, false, e
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return v, false, err
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM platform_identity.workspaces WHERE id=$1)`, v.TenantID).Scan(&exists); err != nil {
		return v, false, err
	}
	if !exists {
		return v, false, fault.NotFound
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_identity.service_accounts(id,tenant_id,token_hash,scopes,expires_at) VALUES($1,$2,$3,$4,$5)`, v.ID, v.TenantID, hash, v.Scopes, v.ExpiresAt); err != nil {
		return v, false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_identity.managed_service_keys(service_id,name,actor_id,request_key,request_hash) VALUES($1,$2,$3,$4,$5)`, v.ID, v.Name, actor, request, fingerprint); err != nil {
		return v, false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_identity.service_account_audit(service_id,tenant_id,action,actor) VALUES($1,$2,'issued',$3)`, v.ID, v.TenantID, actor); err != nil {
		return v, false, err
	}
	out, err := scanManaged(tx.QueryRow(ctx, `SELECT `+managedColumns+managedFrom+`WHERE s.id=$1`, v.ID))
	if err != nil {
		return v, false, err
	}
	return out, true, tx.Commit(ctx)
}
func (p Repository) RevokeManagedKey(ctx context.Context, actor, id string) (identity.ManagedKey, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return identity.ManagedKey{}, err
	}
	defer tx.Rollback(ctx)
	v, err := scanManaged(tx.QueryRow(ctx, `SELECT `+managedColumns+managedFrom+`WHERE s.id=$1 FOR UPDATE OF s`, id))
	if err != nil {
		return v, err
	}
	if v.RevokedAt == nil {
		if _, err = tx.Exec(ctx, `UPDATE platform_identity.service_accounts SET revoked_at=now() WHERE id=$1`, id); err != nil {
			return v, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO platform_identity.service_account_audit(service_id,tenant_id,action,actor) VALUES($1,$2,'revoked',$3)`, id, v.TenantID, actor); err != nil {
			return v, err
		}
	}
	v, err = scanManaged(tx.QueryRow(ctx, `SELECT `+managedColumns+managedFrom+`WHERE s.id=$1`, id))
	if err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}
