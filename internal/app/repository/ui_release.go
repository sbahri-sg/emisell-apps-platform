package repository

import (
	"context"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/platform/fault"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func scanUI(row pgx.Row) (service.UIRelease, error) {
	var raw []byte
	var v service.UIRelease
	err := row.Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, fault.NotFound
	}
	if err != nil {
		return v, err
	}
	err = json.Unmarshal(raw, &v)
	return v, err
}

func (p Postgres) UIList(ctx context.Context, org, after string) ([]service.UIRelease, error) {
	rows, err := p.Pool.Query(ctx, `SELECT document FROM platform_app.ui_releases WHERE ($1='' OR organization_id=$1) AND id>$2 ORDER BY id LIMIT 20`, org, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []service.UIRelease{}
	for rows.Next() {
		v, err := scanUI(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}
func (p Postgres) UIGet(ctx context.Context, org, id string) (service.UIRelease, error) {
	return scanUI(p.Pool.QueryRow(ctx, `SELECT document FROM platform_app.ui_releases WHERE id=$1 AND ($2='' OR organization_id=$2)`, id, org))
}

func (p Postgres) UIWith(ctx context.Context, org, id string, fn func(service.UIRelease) error) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	v, err := scanUI(tx.QueryRow(ctx, `SELECT document FROM platform_app.ui_releases WHERE id=$1 AND organization_id=$2 FOR SHARE`, id, org))
	if err != nil {
		return err
	}
	if err = fn(v); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (p Postgres) UIChange(ctx context.Context, org, actor, key, hash, target string, create *service.UIRelease, reason string, change func(service.UIRelease) (service.UIRelease, error)) (service.UIRelease, error) {
	var zero service.UIRelease
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return zero, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,19))`, actor+":"+key); err != nil {
		return zero, err
	}
	var previousHash, id string
	err = tx.QueryRow(ctx, `SELECT request_hash,release_id FROM platform_app.ui_release_requests WHERE actor_id=$1 AND request_key=$2`, actor, key).Scan(&previousHash, &id)
	if err == nil {
		if previousHash != hash {
			return zero, fault.Conflict
		}
		return scanUI(tx.QueryRow(ctx, `SELECT document FROM platform_app.ui_releases WHERE id=$1 AND ($2='' OR organization_id=$2)`, id, org))
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return zero, err
	}
	var v service.UIRelease
	if create != nil {
		if target != "" {
			var owned bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM platform_app.ui_releases WHERE app_id=$1 AND organization_id=$2)`, target, org).Scan(&owned); err != nil {
				return zero, err
			}
			if !owned {
				return zero, fault.NotFound
			}
		}
		v = *create
		raw, _ := json.Marshal(v)
		_, err = tx.Exec(ctx, `INSERT INTO platform_app.ui_releases(id,app_id,organization_id,version,document) VALUES($1,$2,$3,$4,$5)`, v.ID, v.Manifest.AppID, org, v.Manifest.Version, raw)
	} else {
		v, err = scanUI(tx.QueryRow(ctx, `SELECT document FROM platform_app.ui_releases WHERE id=$1 FOR UPDATE`, target))
		if err != nil {
			return zero, err
		}
		v, err = change(v)
		if err != nil {
			return zero, err
		}
		raw, _ := json.Marshal(v)
		_, err = tx.Exec(ctx, `UPDATE platform_app.ui_releases SET document=$2 WHERE id=$1`, target, raw)
	}
	if err != nil {
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23505" {
			return zero, fault.Conflict
		}
		return zero, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_app.ui_release_requests VALUES($1,$2,$3,$4)`, actor, key, hash, v.ID); err != nil {
		return zero, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_app.ui_release_audit(release_id,actor_id,action,reason) VALUES($1,$2,$3,$4)`, v.ID, actor, v.Status, reason); err != nil {
		return zero, err
	}
	return v, tx.Commit(ctx)
}
