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

func scanDraft(row pgx.Row) (service.Draft, error) {
	var v service.Draft
	var raw []byte
	err := row.Scan(&v.ID, &v.OrganizationID, &v.Revision, &raw, &v.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = fault.NotFound
	}
	if err == nil {
		err = json.Unmarshal(raw, &v.Document)
	}
	return v, err
}

const draftColumns = `id,organization_id,revision,document,updated_at`

func (p Postgres) Drafts(ctx context.Context, org string) ([]service.Draft, error) {
	rows, err := p.Pool.Query(ctx, `SELECT `+draftColumns+` FROM platform_app.drafts WHERE organization_id=$1 ORDER BY updated_at DESC,id LIMIT 200`, org)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []service.Draft{}
	for rows.Next() {
		v, err := scanDraft(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (p Postgres) Draft(ctx context.Context, org, id string) (service.Draft, error) {
	return scanDraft(p.Pool.QueryRow(ctx, `SELECT `+draftColumns+` FROM platform_app.drafts WHERE organization_id=$1 AND id=$2`, org, id))
}
func (p Postgres) SaveDraft(ctx context.Context, org, actor, id, key, hash string, b service.SaveDraft) (service.Draft, error) {
	var out service.Draft
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "draft:"+org); err != nil {
		return out, err
	}
	// Ownership is checked before idempotent replay as well.
	if id != "" {
		var exists bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM platform_app.drafts WHERE organization_id=$1 AND id=$2)`, org, id).Scan(&exists); err != nil {
			return out, err
		}
		if !exists {
			return out, fault.NotFound
		}
	}
	var oldHash string
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT request_hash,response FROM platform_app.draft_requests WHERE organization_id=$1 AND request_key=$2`, org, key).Scan(&oldHash, &raw)
	if err == nil {
		if oldHash != hash {
			return out, fault.Conflict
		}
		err = json.Unmarshal(raw, &out)
		return out, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	doc, _ := json.Marshal(b.Document)
	action := "draft_updated"
	if id == "" {
		id = ids.New("app")
		action = "draft_created"
		out, err = scanDraft(tx.QueryRow(ctx, `INSERT INTO platform_app.drafts(id,organization_id,revision,document) VALUES($1,$2,1,$3) RETURNING `+draftColumns, id, org, doc))
	} else {
		out, err = scanDraft(tx.QueryRow(ctx, `UPDATE platform_app.drafts SET revision=revision+1,document=$4,updated_at=now() WHERE id=$1 AND organization_id=$2 AND revision=$3 RETURNING `+draftColumns, id, org, b.Revision, doc))
		if errors.Is(err, fault.NotFound) {
			err = fault.Conflict
		}
	}
	if err != nil {
		return out, err
	}
	raw, _ = json.Marshal(out)
	if _, err = tx.Exec(ctx, `INSERT INTO platform_app.draft_requests(organization_id,request_key,request_hash,response) VALUES($1,$2,$3,$4)`, org, key, hash, raw); err != nil {
		return out, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_app.draft_audit(id,app_id,organization_id,actor_id,revision,action) VALUES($1,$2,$3,$4,$5,$6)`, ids.New("aud"), id, org, actor, out.Revision, action); err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
