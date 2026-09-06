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

const managed_shippingColumns = `id,organization_id,manifest,sha256,package,status,revision,created_at,updated_at`

func (p Postgres) ManagedShippingWith(ctx context.Context, org, id string, fn func(service.ManagedShippingRelease) error) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	v, err := scanManagedShipping(tx.QueryRow(ctx, `SELECT `+managed_shippingColumns+` FROM platform_app.managed_shipping_releases WHERE id=$1 AND organization_id=$2 FOR SHARE`, id, org))
	if err != nil {
		return err
	}
	return fn(v)
}

func (p Postgres) ManagedShippingReplay(ctx context.Context, org, actor, key, hash string) (*service.ManagedShippingRelease, error) {
	var oldHash, id string
	err := p.Pool.QueryRow(ctx, `SELECT r.request_hash,r.release_id FROM platform_app.managed_shipping_requests r JOIN platform_app.managed_shipping_releases v ON v.id=r.release_id WHERE r.actor_id=$1 AND r.request_key=$2 AND v.organization_id=$3`, actor, key, org).Scan(&oldHash, &id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if oldHash != hash {
		return nil, fault.Conflict
	}
	v, err := p.ManagedShippingGet(ctx, org, id)
	return &v, err
}

func scanManagedShipping(row pgx.Row) (service.ManagedShippingRelease, error) {
	var v service.ManagedShippingRelease
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
func (p Postgres) ManagedShippingList(ctx context.Context, org string) ([]service.ManagedShippingRelease, error) {
	rows, err := p.Pool.Query(ctx, `SELECT `+managed_shippingColumns+` FROM platform_app.managed_shipping_releases WHERE ($1='' OR organization_id=$1) ORDER BY created_at DESC,id LIMIT 200`, org)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []service.ManagedShippingRelease{}
	for rows.Next() {
		v, err := scanManagedShipping(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (p Postgres) ManagedShippingGet(ctx context.Context, org, id string) (service.ManagedShippingRelease, error) {
	return scanManagedShipping(p.Pool.QueryRow(ctx, `SELECT `+managed_shippingColumns+` FROM platform_app.managed_shipping_releases WHERE id=$1 AND ($2='' OR organization_id=$2)`, id, org))
}
func (p Postgres) ManagedShippingHistory(ctx context.Context, id string) ([]service.CatalogAudit, error) {
	rows, err := p.Pool.Query(ctx, `SELECT id,actor_id,action,reason,occurred_at FROM platform_app.managed_shipping_audit WHERE release_id=$1 ORDER BY occurred_at,id LIMIT 200`, id)
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
func (p Postgres) ManagedShippingMutate(ctx context.Context, org, actor, key, hash, id string, create *service.ManagedShippingRelease, reason string, change func(service.ManagedShippingRelease) (service.ManagedShippingRelease, error)) (service.ManagedShippingRelease, error) {
	var out service.ManagedShippingRelease
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "managed_shipping:"+actor+":"+key); err != nil {
		return out, err
	}
	var oldHash, oldID string
	err = tx.QueryRow(ctx, `SELECT request_hash,release_id FROM platform_app.managed_shipping_requests WHERE actor_id=$1 AND request_key=$2`, actor, key).Scan(&oldHash, &oldID)
	if err == nil {
		if oldHash != hash {
			return out, fault.Conflict
		}
		return scanManagedShipping(tx.QueryRow(ctx, `SELECT `+managed_shippingColumns+` FROM platform_app.managed_shipping_releases WHERE id=$1 AND ($2='' OR organization_id=$2) FOR UPDATE`, oldID, org))
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	if create != nil {
		m := create.Manifest
		source, err := scanDraft(tx.QueryRow(ctx, "SELECT "+draftColumns+" FROM platform_app.drafts WHERE id=$1 AND organization_id=$2 FOR SHARE", m.AppID, org))
		if err != nil {
			return out, err
		}
		if source.Revision != m.DraftRevision || service.RequestHash(source) != m.SourceSHA256 {
			return out, fault.Conflict
		}
		raw, err := json.Marshal(m)
		if err != nil {
			return out, err
		}
		out, err = scanManagedShipping(tx.QueryRow(ctx, `INSERT INTO platform_app.managed_shipping_releases(id,organization_id,app_id,draft_revision,version,manifest,sha256,status) VALUES($1,$2,$3,$4,$5,$6,$7,'submitted') RETURNING `+managed_shippingColumns, ids.New("msrel"), org, m.AppID, m.DraftRevision, m.Version, raw, create.SHA256))
		if err != nil {
			return out, conflictCatalog(err)
		}
	} else {
		out, err = scanManagedShipping(tx.QueryRow(ctx, `SELECT `+managed_shippingColumns+` FROM platform_app.managed_shipping_releases WHERE id=$1 AND ($2='' OR organization_id=$2) FOR UPDATE`, id, org))
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
		out, err = scanManagedShipping(tx.QueryRow(ctx, `UPDATE platform_app.managed_shipping_releases SET status=$2,revision=$3,package=$4,updated_at=now() WHERE id=$1 RETURNING `+managed_shippingColumns, id, out.Status, out.Revision, pack))
		if err != nil {
			return out, err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_app.managed_shipping_requests(actor_id,request_key,request_hash,release_id) VALUES($1,$2,$3,$4)`, actor, key, hash, out.ID); err != nil {
		return out, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_app.managed_shipping_audit(id,release_id,actor_id,action,reason) VALUES($1,$2,$3,$4,$5)`, ids.New("aud"), out.ID, actor, out.Status, reason); err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
