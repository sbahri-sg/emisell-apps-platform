package repository

import (
	"context"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
)

const integrationColumns = `id,organization_id,manifest,sha256,package,status,revision,created_at,updated_at`

func (p Postgres) IntegrationWith(ctx context.Context, org, id string, fn func(service.IntegrationRelease) error) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	v, err := scanIntegration(tx.QueryRow(ctx, `SELECT `+integrationColumns+` FROM platform_app.integration_releases WHERE id=$1 AND organization_id=$2 FOR SHARE`, id, org))
	if err != nil {
		return err
	}
	return fn(v)
}

func scanIntegration(row pgx.Row) (service.IntegrationRelease, error) {
	var v service.IntegrationRelease
	var manifest, pack []byte
	err := row.Scan(&v.ID, &v.OrganizationID, &manifest, &v.SHA256, &pack, &v.Status, &v.Revision, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = fault.NotFound
	}
	if err == nil {
		err = json.Unmarshal(manifest, &v.Manifest)
	}
	if err == nil && pack != nil {
		err = json.Unmarshal(pack, &v.Package)
	}
	return v, err
}
func (p Postgres) IntegrationList(ctx context.Context, org string) ([]service.IntegrationRelease, error) {
	rows, err := p.Pool.Query(ctx, `SELECT `+integrationColumns+` FROM platform_app.integration_releases WHERE ($1='' OR organization_id=$1) ORDER BY created_at DESC,id LIMIT 200`, org)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []service.IntegrationRelease{}
	for rows.Next() {
		v, err := scanIntegration(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (p Postgres) IntegrationGet(ctx context.Context, org, id string) (service.IntegrationRelease, error) {
	return scanIntegration(p.Pool.QueryRow(ctx, `SELECT `+integrationColumns+` FROM platform_app.integration_releases WHERE id=$1 AND ($2='' OR organization_id=$2)`, id, org))
}
func (p Postgres) IntegrationHistory(ctx context.Context, id string) ([]service.CatalogAudit, error) {
	rows, err := p.Pool.Query(ctx, `SELECT id,actor_id,action,reason,occurred_at FROM platform_app.integration_audit WHERE release_id=$1 ORDER BY occurred_at,id LIMIT 200`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []service.CatalogAudit{}
	for rows.Next() {
		var v service.CatalogAudit
		if err := rows.Scan(&v.ID, &v.ActorID, &v.Action, &v.Reason, &v.OccurredAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Role checks precede replay. Receipts resolve CURRENT state, never old approval.
func (p Postgres) IntegrationMutate(ctx context.Context, org, actor, key, hash, id string, create *service.IntegrationRelease, reason string, change func(service.IntegrationRelease) (service.IntegrationRelease, error)) (service.IntegrationRelease, error) {
	var out service.IntegrationRelease
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "integration:"+actor+":"+key); err != nil {
		return out, err
	}
	var oldHash, oldID string
	err = tx.QueryRow(ctx, `SELECT request_hash,release_id FROM platform_app.integration_requests WHERE actor_id=$1 AND request_key=$2`, actor, key).Scan(&oldHash, &oldID)
	if err == nil {
		if oldHash != hash {
			return out, fault.Conflict
		}
		return scanIntegration(tx.QueryRow(ctx, `SELECT `+integrationColumns+` FROM platform_app.integration_releases WHERE id=$1 AND ($2='' OR organization_id=$2) FOR UPDATE`, oldID, org))
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	if create != nil {
		m := create.Manifest
		raw, err := json.Marshal(m)
		if err != nil {
			return out, err
		}
		out, err = scanIntegration(tx.QueryRow(ctx, `INSERT INTO platform_app.integration_releases(id,organization_id,app_id,submission_id,version,manifest,sha256,status) VALUES($1,$2,$3,$4,$5,$6,$7,'submitted') RETURNING `+integrationColumns, ids.New("intrel"), org, m.Metadata.AppID, m.SubmissionID, m.Metadata.Version, raw, create.SHA256))
		if err != nil {
			return out, conflictCatalog(err)
		}
	} else {
		out, err = scanIntegration(tx.QueryRow(ctx, `SELECT `+integrationColumns+` FROM platform_app.integration_releases WHERE id=$1 AND ($2='' OR organization_id=$2) FOR UPDATE`, id, org))
		if err != nil {
			return out, err
		}
		out, err = change(out)
		if err != nil {
			return out, err
		}
		var pack []byte
		if out.Package != nil {
			pack, err = json.Marshal(out.Package)
			if err != nil {
				return out, err
			}
		}
		out, err = scanIntegration(tx.QueryRow(ctx, `UPDATE platform_app.integration_releases SET status=$2,revision=$3,package=$4,updated_at=now() WHERE id=$1 RETURNING `+integrationColumns, id, out.Status, out.Revision, pack))
		if err != nil {
			return out, err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_app.integration_requests(actor_id,request_key,request_hash,release_id) VALUES($1,$2,$3,$4)`, actor, key, hash, out.ID); err != nil {
		return out, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_app.integration_audit(id,release_id,actor_id,action,reason) VALUES($1,$2,$3,$4,$5)`, ids.New("aud"), out.ID, actor, out.Status, reason); err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
