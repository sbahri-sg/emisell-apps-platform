package postgres

import (
	"context"
	"errors"

	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/platform/fault"
	"github.com/jackc/pgx/v5"
)

const platformColumns = `id,name,created_at,revoked_at,CASE WHEN revoked_at IS NULL THEN 'active' ELSE 'revoked' END`

func scanPlatformKey(row pgx.Row) (identity.PlatformKey, error) {
	v := identity.PlatformKey{Access: "platform_full"}
	err := row.Scan(&v.ID, &v.Name, &v.CreatedAt, &v.RevokedAt, &v.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		err = fault.NotFound
	}
	return v, err
}
func (p Repository) ListPlatformKeys(ctx context.Context) ([]identity.PlatformKey, error) {
	rows, err := p.Pool.Query(ctx, `SELECT `+platformColumns+` FROM platform_identity.core_platform_keys ORDER BY created_at DESC,id LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []identity.PlatformKey{}
	for rows.Next() {
		v, e := scanPlatformKey(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (p Repository) CreatePlatformKey(ctx context.Context, actor, request, fingerprint string, v identity.PlatformKey, hash string) (identity.PlatformKey, bool, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return v, false, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "platformkey:"+actor+":"+request); err != nil {
		return v, false, err
	}
	var storedHash, id string
	err = tx.QueryRow(ctx, `SELECT request_hash,id FROM platform_identity.core_platform_keys WHERE actor_id=$1 AND request_key=$2`, actor, request).Scan(&storedHash, &id)
	if err == nil {
		if storedHash != fingerprint {
			return v, false, fault.Conflict
		}
		out, e := scanPlatformKey(tx.QueryRow(ctx, `SELECT `+platformColumns+` FROM platform_identity.core_platform_keys WHERE id=$1`, id))
		return out, false, e
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return v, false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_identity.core_platform_keys(id,name,token_hash,actor_id,request_key,request_hash) VALUES($1,$2,$3,$4,$5,$6)`, v.ID, v.Name, hash, actor, request, fingerprint); err != nil {
		return v, false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_identity.core_platform_key_audit(key_id,action,actor_id) VALUES($1,'issued',$2)`, v.ID, actor); err != nil {
		return v, false, err
	}
	out, err := scanPlatformKey(tx.QueryRow(ctx, `SELECT `+platformColumns+` FROM platform_identity.core_platform_keys WHERE id=$1`, v.ID))
	if err != nil {
		return v, false, err
	}
	return out, true, tx.Commit(ctx)
}
func (p Repository) RevokePlatformKey(ctx context.Context, actor, id string) (identity.PlatformKey, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return identity.PlatformKey{}, err
	}
	defer tx.Rollback(ctx)
	v, err := scanPlatformKey(tx.QueryRow(ctx, `SELECT `+platformColumns+` FROM platform_identity.core_platform_keys WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return v, err
	}
	if v.RevokedAt == nil {
		if _, err = tx.Exec(ctx, `UPDATE platform_identity.core_platform_keys SET revoked_at=now() WHERE id=$1`, id); err != nil {
			return v, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO platform_identity.core_platform_key_audit(key_id,action,actor_id) VALUES($1,'revoked',$2)`, id, actor); err != nil {
			return v, err
		}
	}
	v, err = scanPlatformKey(tx.QueryRow(ctx, `SELECT `+platformColumns+` FROM platform_identity.core_platform_keys WHERE id=$1`, id))
	if err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}
func (p Repository) FindPlatformService(ctx context.Context, hash string) (identity.ServicePrincipal, error) {
	var id string
	err := p.Pool.QueryRow(ctx, `SELECT id FROM platform_identity.core_platform_keys WHERE token_hash=$1 AND revoked_at IS NULL`, hash).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = fault.Unauthenticated
	}
	if err != nil {
		return identity.ServicePrincipal{}, err
	}
	return identity.ServicePrincipal{ID: id, PlatformFull: true}, nil
}
func (p Repository) PlatformServiceAllowed(ctx context.Context, id, tenant string) (bool, error) {
	var ok bool
	err := p.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM platform_identity.core_platform_keys k WHERE k.id=$1 AND k.revoked_at IS NULL) AND EXISTS(SELECT 1 FROM platform_identity.workspaces WHERE id=$2)`, id, tenant).Scan(&ok)
	return ok, err
}
