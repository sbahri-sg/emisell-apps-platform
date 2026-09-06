package postgres

import (
	"context"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"emisell.app/platform/internal/review"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ Pool *pgxpool.Pool }

const columns = `id,app_id,organization_id,submitter_id,draft_revision,version,snapshot,status,created_at,decided_at,COALESCE(reviewer_id,''),feedback`

func scan(row pgx.Row) (review.Submission, error) {
	var v review.Submission
	var raw []byte
	err := row.Scan(&v.ID, &v.AppID, &v.OrganizationID, &v.SubmitterID, &v.DraftRevision, &v.Version, &raw, &v.Status, &v.CreatedAt, &v.DecidedAt, &v.ReviewerID, &v.Feedback)
	if errors.Is(err, pgx.ErrNoRows) {
		err = fault.NotFound
	}
	if err == nil {
		err = json.Unmarshal(raw, &v.Snapshot)
	}
	return v, err
}
func (p Repository) List(ctx context.Context, org string) ([]review.Submission, error) {
	rows, err := p.Pool.Query(ctx, `SELECT `+columns+` FROM platform_review.submissions WHERE ($1='' OR organization_id=$1) ORDER BY created_at DESC,id LIMIT 200`, org)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []review.Submission{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (p Repository) Get(ctx context.Context, id string) (review.Submission, error) {
	return scan(p.Pool.QueryRow(ctx, `SELECT `+columns+` FROM platform_review.submissions WHERE id=$1`, id))
}
func (p Repository) History(ctx context.Context, id string) ([]review.Audit, error) {
	rows, err := p.Pool.Query(ctx, `SELECT id,actor_id,action,feedback,occurred_at FROM platform_review.audit WHERE submission_id=$1 ORDER BY occurred_at,id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []review.Audit{}
	for rows.Next() {
		var v review.Audit
		if err = rows.Scan(&v.ID, &v.ActorID, &v.Action, &v.Feedback, &v.OccurredAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Lock an actor's replay namespace before the app/submission lock, consistently.
func begin(ctx context.Context, pool *pgxpool.Pool, actor, key, hash string) (pgx.Tx, *review.Submission, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "review-request:"+actor); err != nil {
		tx.Rollback(ctx)
		return nil, nil, err
	}
	var previous string
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT request_hash,response FROM platform_review.requests WHERE actor_id=$1 AND request_key=$2`, actor, key).Scan(&previous, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return tx, nil, nil
	}
	if err != nil {
		tx.Rollback(ctx)
		return nil, nil, err
	}
	tx.Rollback(ctx)
	if previous != hash {
		return nil, nil, fault.Conflict
	}
	var v review.Submission
	err = json.Unmarshal(raw, &v)
	return nil, &v, err
}
func finish(ctx context.Context, tx pgx.Tx, actor, key, hash, action string, v review.Submission) (review.Submission, error) {
	raw, _ := json.Marshal(v)
	if _, err := tx.Exec(ctx, `INSERT INTO platform_review.requests(actor_id,request_key,request_hash,response) VALUES($1,$2,$3,$4)`, actor, key, hash, raw); err != nil {
		return v, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO platform_review.audit(id,submission_id,actor_id,action,feedback) VALUES($1,$2,$3,$4,$5)`, ids.New("aud"), v.ID, actor, action, v.Feedback); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}
func (p Repository) Submit(ctx context.Context, actor identity.PortalPrincipal, d service.Draft, expected int, key, hash string) (review.Submission, error) {
	var out review.Submission
	tx, previous, err := begin(ctx, p.Pool, actor.ID, key, "submit:"+hash)
	if err != nil {
		return out, err
	}
	if previous != nil {
		return *previous, nil
	}
	defer tx.Rollback(ctx)
	if expected != d.Revision {
		return out, fault.Conflict
	}
	if err = d.Document.Validate(true); err != nil {
		return out, err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "review-app:"+d.ID); err != nil {
		return out, err
	}
	var exists bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM platform_review.submissions WHERE app_id=$1 AND (status='submitted' OR draft_revision=$2 OR (version=$3 AND status IN ('approved','rejected'))))`, d.ID, d.Revision, d.Document.Version).Scan(&exists)
	if err != nil {
		return out, err
	}
	if exists {
		return out, fault.Conflict
	}
	raw, _ := json.Marshal(d.Document)
	out, err = scan(tx.QueryRow(ctx, `INSERT INTO platform_review.submissions(id,app_id,organization_id,submitter_id,draft_revision,version,snapshot,status) VALUES($1,$2,$3,$4,$5,$6,$7,'submitted') RETURNING `+columns, ids.New("sub"), d.ID, d.OrganizationID, actor.ID, d.Revision, d.Document.Version, raw))
	if err != nil {
		return out, err
	}
	return finish(ctx, tx, actor.ID, key, "submit:"+hash, "submitted", out)
}
func (p Repository) Decide(ctx context.Context, actor identity.PortalPrincipal, id, key, hash string, b review.Decision) (review.Submission, error) {
	var out review.Submission
	tx, previous, err := begin(ctx, p.Pool, actor.ID, key, "decision:"+hash)
	if err != nil {
		return out, err
	}
	if previous != nil {
		return *previous, nil
	}
	defer tx.Rollback(ctx)
	out, err = scan(tx.QueryRow(ctx, `SELECT `+columns+` FROM platform_review.submissions WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return out, err
	}
	if out.SubmitterID == actor.ID {
		return out, fault.Forbidden
	}
	if out.Status != "submitted" {
		return out, fault.Conflict
	}
	out, err = scan(tx.QueryRow(ctx, `UPDATE platform_review.submissions SET status=$2,feedback=$3,reviewer_id=$4,decided_at=now() WHERE id=$1 RETURNING `+columns, id, b.Status, b.Feedback, actor.ID))
	if err != nil {
		return out, err
	}
	return finish(ctx, tx, actor.ID, key, "decision:"+hash, b.Status, out)
}
