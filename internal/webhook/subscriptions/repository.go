package subscriptions

import (
	"context"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"slices"
	"time"
)

type Input struct {
	DraftRevision int    `json:"draftRevision"`
	Version       string `json:"version"`
	Topic         string `json:"topic"`
	Endpoint      string `json:"endpoint"`
}
type Record struct {
	ID              string    `json:"id"`
	AppID           string    `json:"appId"`
	DraftRevision   int       `json:"draftRevision"`
	Version         string    `json:"version"`
	Topic           string    `json:"topic"`
	RequiredScope   string    `json:"requiredScope"`
	Endpoint        string    `json:"endpoint"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"createdAt"`
	DeliveryEnabled bool      `json:"deliveryEnabled"`
}
type Repository struct{ Pool *pgxpool.Pool }

const columns = `id,app_id,draft_revision,version,topic,required_scope,endpoint,status,created_at`

func scan(row pgx.Row) (Record, error) {
	var r Record
	e := row.Scan(&r.ID, &r.AppID, &r.DraftRevision, &r.Version, &r.Topic, &r.RequiredScope, &r.Endpoint, &r.Status, &r.CreatedAt)
	if errors.Is(e, pgx.ErrNoRows) {
		e = fault.NotFound
	}
	return r, e
}

func (p Repository) List(ctx context.Context, org, app, after string) ([]Record, error) {
	rows, e := p.Pool.Query(ctx, `SELECT `+columns+` FROM platform_webhook.subscription_requests WHERE organization_id=$1 AND app_id=$2 AND id>$3 ORDER BY id LIMIT 51`, org, app, after)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Record{}
	for rows.Next() {
		r, e := scan(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (p Repository) Create(ctx context.Context, org, actor, app, key string, b Input) (Record, error) {
	scope, e := requiredScope(b.Topic)
	if e != nil || !ValidEndpoint(b.Endpoint) || !service.ValidRequestKey(key) || b.DraftRevision < 1 {
		return Record{}, fault.Invalid
	}
	tx, e := p.Pool.Begin(ctx)
	if e != nil {
		return Record{}, e
	}
	defer tx.Rollback(ctx)
	// Serialize authoring with draft updates and with other subscription requests.
	var rev int
	var doc service.AppDocument
	e = tx.QueryRow(ctx, `SELECT revision,document FROM platform_app.drafts WHERE id=$1 AND organization_id=$2 FOR UPDATE`, app, org).Scan(&rev, &doc)
	if errors.Is(e, pgx.ErrNoRows) {
		return Record{}, fault.NotFound
	}
	if e != nil {
		return Record{}, e
	}
	hash := service.RequestHash(struct {
		App  string
		Body Input
	}{app, b})
	var oldHash, id string
	e = tx.QueryRow(ctx, `SELECT request_hash,id FROM platform_webhook.subscription_requests WHERE organization_id=$1 AND actor_id=$2 AND request_key=$3`, org, actor, key).Scan(&oldHash, &id)
	if e == nil {
		if oldHash != hash {
			return Record{}, fault.Conflict
		}
		return scan(tx.QueryRow(ctx, `SELECT `+columns+` FROM platform_webhook.subscription_requests WHERE id=$1`, id))
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return Record{}, e
	}
	if rev != b.DraftRevision || doc.Version != b.Version {
		return Record{}, fault.Conflict
	}
	if doc.AccessScopes == nil || doc.AccessScopes.Validate() != nil || (!slices.Contains(doc.AccessScopes.Required, scope) && !slices.Contains(doc.AccessScopes.Optional, scope)) {
		return Record{}, fault.Forbidden
	}
	var count int
	if e = tx.QueryRow(ctx, `SELECT count(*) FROM platform_webhook.subscription_requests WHERE app_id=$1 AND status='pending'`, app).Scan(&count); e != nil {
		return Record{}, e
	}
	if count >= 50 {
		return Record{}, fault.Conflict
	}
	id = ids.New("whsub")
	r, e := scan(tx.QueryRow(ctx, `INSERT INTO platform_webhook.subscription_requests(id,organization_id,app_id,draft_revision,version,topic,required_scope,endpoint,actor_id,request_key,request_hash) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING `+columns, id, org, app, b.DraftRevision, b.Version, b.Topic, scope, b.Endpoint, actor, key, hash))
	var pe *pgconn.PgError
	if errors.As(e, &pe) && pe.Code == "23505" {
		return Record{}, fault.Conflict
	}
	if e != nil {
		return Record{}, e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO platform_webhook.subscription_audit(id,subscription_id,actor_id,action) VALUES($1,$2,$3,'requested')`, ids.New("audit"), id, actor); e != nil {
		return Record{}, e
	}
	return r, tx.Commit(ctx)
}

func (p Repository) Revoke(ctx context.Context, org, actor, app, id string) (Record, error) {
	tx, e := p.Pool.Begin(ctx)
	if e != nil {
		return Record{}, e
	}
	defer tx.Rollback(ctx)
	r, e := scan(tx.QueryRow(ctx, `SELECT `+columns+` FROM platform_webhook.subscription_requests WHERE id=$1 AND organization_id=$2 AND app_id=$3 FOR UPDATE`, id, org, app))
	if e != nil {
		return Record{}, e
	}
	if r.Status == "revoked" {
		return r, nil
	}
	if _, e = tx.Exec(ctx, `UPDATE platform_webhook.subscription_requests SET status='revoked',revoked_at=now() WHERE id=$1`, id); e != nil {
		return Record{}, e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO platform_webhook.subscription_audit(id,subscription_id,actor_id,action) VALUES($1,$2,$3,'revoked')`, ids.New("audit"), id, actor); e != nil {
		return Record{}, e
	}
	r.Status = "revoked"
	return r, tx.Commit(ctx)
}
