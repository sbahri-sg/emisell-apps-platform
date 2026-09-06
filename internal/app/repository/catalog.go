package repository

import (
	"context"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"emisell.app/platform/pkg/catalogmanifest"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const catalogColumns = `id,submission_id,package,status,revision,created_at,updated_at`

func scanCatalog(row pgx.Row) (service.CatalogRelease, error) {
	var v service.CatalogRelease
	var raw []byte
	err := row.Scan(&v.ID, &v.SubmissionID, &raw, &v.Status, &v.Revision, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = fault.NotFound
	}
	if err == nil {
		err = json.Unmarshal(raw, &v.Package)
	}
	return v, err
}
func (p Postgres) CatalogGet(ctx context.Context, org, id string) (service.CatalogRelease, error) {
	return scanCatalog(p.Pool.QueryRow(ctx, `SELECT `+catalogColumns+` FROM platform_app.catalog_releases WHERE id=$1 AND ($2='' OR organization_id=$2)`, id, org))
}
func catalogRows(rows pgx.Rows) ([]service.CatalogRelease, error) {
	defer rows.Close()
	out := []service.CatalogRelease{}
	for rows.Next() {
		v, err := scanCatalog(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (p Postgres) CatalogList(ctx context.Context, org string) ([]service.CatalogRelease, error) {
	rows, err := p.Pool.Query(ctx, `SELECT `+catalogColumns+` FROM platform_app.catalog_releases WHERE ($1='' OR organization_id=$1) ORDER BY created_at DESC,id LIMIT 200`, org)
	if err != nil {
		return nil, err
	}
	return catalogRows(rows)
}
func catalogReplay(ctx context.Context, tx pgx.Tx, actor, key, hash string) (service.CatalogRelease, bool, error) {
	var v service.CatalogRelease
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "catalog-request:"+actor+":"+key); err != nil {
		return v, false, err
	}
	var old string
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT request_hash,response FROM platform_app.catalog_requests WHERE actor_id=$1 AND request_key=$2`, actor, key).Scan(&old, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, false, nil
	}
	if err != nil {
		return v, false, err
	}
	if old != hash {
		return v, false, fault.Conflict
	}
	err = json.Unmarshal(raw, &v)
	return v, true, err
}
func saveCatalogAction(ctx context.Context, tx pgx.Tx, actor, key, hash, action, reason string, v service.CatalogRelease) error {
	raw, _ := json.Marshal(v)
	if _, err := tx.Exec(ctx, `INSERT INTO platform_app.catalog_requests(actor_id,request_key,request_hash,response) VALUES($1,$2,$3,$4)`, actor, key, hash, raw); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO platform_app.catalog_audit(id,release_id,actor_id,action,reason) VALUES($1,$2,$3,$4,$5)`, ids.New("aud"), v.ID, actor, action, reason)
	return err
}
func conflictCatalog(err error) error {
	var e *pgconn.PgError
	if errors.As(err, &e) && e.Code == "23505" {
		return fault.Conflict
	}
	return err
}
func (p Postgres) CatalogSign(ctx context.Context, actor, key, hash string, c service.CatalogCandidate, pack catalogmanifest.Package) (service.CatalogRelease, error) {
	var out service.CatalogRelease
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	out, replayed, err := catalogReplay(ctx, tx, actor, key, hash)
	if err != nil || replayed {
		return out, err
	}
	raw, _ := json.Marshal(pack)
	out, err = scanCatalog(tx.QueryRow(ctx, `INSERT INTO platform_app.catalog_releases(id,app_id,organization_id,submission_id,version,package,status) VALUES($1,$2,$3,$4,$5,$6,'signed') RETURNING `+catalogColumns, ids.New("cat"), c.AppID, c.OrganizationID, c.SubmissionID, c.Document.Version, raw))
	if err != nil {
		return out, conflictCatalog(err)
	}
	if err = saveCatalogAction(ctx, tx, actor, key, hash, "signed", "catalog-metadata/v1: metadata validated, no executable artifact", out); err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
func (p Postgres) CatalogTransition(ctx context.Context, actor, id, key, hash string, b service.CatalogAction) (service.CatalogRelease, error) {
	var out service.CatalogRelease
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	out, replayed, err := catalogReplay(ctx, tx, actor, key, hash)
	if err != nil || replayed {
		return out, err
	}
	out, err = scanCatalog(tx.QueryRow(ctx, `SELECT `+catalogColumns+` FROM platform_app.catalog_releases WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return out, err
	}
	if out.Revision != b.Revision || out.Status == b.Status || (b.Status == "suspended" && out.Status != "published") {
		return out, fault.Conflict
	}
	out, err = scanCatalog(tx.QueryRow(ctx, `UPDATE platform_app.catalog_releases SET status=$2,revision=revision+1,updated_at=now() WHERE id=$1 RETURNING `+catalogColumns, id, b.Status))
	if err != nil {
		return out, conflictCatalog(err)
	}
	if err = saveCatalogAction(ctx, tx, actor, key, hash, b.Status, b.Reason, out); err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
func (p Postgres) CatalogHistory(ctx context.Context, id string) ([]service.CatalogAudit, error) {
	rows, err := p.Pool.Query(ctx, `SELECT id,actor_id,action,reason,occurred_at FROM platform_app.catalog_audit WHERE release_id=$1 ORDER BY occurred_at,id LIMIT 200`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []service.CatalogAudit{}
	for rows.Next() {
		var v service.CatalogAudit
		if err = rows.Scan(&v.ID, &v.ActorID, &v.Action, &v.Reason, &v.OccurredAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (p Postgres) CatalogPublic(ctx context.Context, q service.CatalogQuery) ([]service.CatalogRelease, int, error) {
	// Literal substring search: user %/_ are not SQL wildcards. Count and page
	// share one repeatable-read snapshot to avoid inconsistent pagination.
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback(ctx)
	const where = ` FROM platform_app.catalog_releases WHERE status='published' AND ($1='' OR strpos(lower((package->'manifest'->>'name')||' '||(package->'manifest'->>'summary')),lower($1))>0) AND ($2='' OR package->'manifest'->>'capability'=$2)`
	var total int
	if err = tx.QueryRow(ctx, `SELECT count(*)`+where, q.Search, q.Capability).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := tx.Query(ctx, `SELECT `+catalogColumns+where+` ORDER BY lower(package->'manifest'->>'name'),id LIMIT 20 OFFSET $3`, q.Search, q.Capability, (q.Page-1)*20)
	if err != nil {
		return nil, 0, err
	}
	values, err := catalogRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return values, total, tx.Commit(ctx)
}
