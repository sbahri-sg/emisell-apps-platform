package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) ListApps(ctx context.Context, organizationID string, filter ports.AppFilter) ([]domain.App, ports.PageMeta, error) {
	offset, limit, err := pagination(filter)
	if err != nil {
		return nil, ports.PageMeta{}, err
	}
	status := string(filter.Status)
	search := filter.Search
	var total int
	if err := r.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM apps
		WHERE organization_id = $1::uuid
		  AND ($2::text = '' OR status = $2)
		  AND ($3::text = '' OR name ILIKE '%' || $3 || '%' OR slug ILIKE '%' || $3 || '%')`,
		organizationID, status, search).Scan(&total); err != nil {
		return nil, ports.PageMeta{}, mapError(err)
	}
	if offset > total {
		return nil, ports.PageMeta{}, fmt.Errorf("%w: cursor is out of range", domain.ErrValidation)
	}
	rows, err := r.pool.Query(ctx, `SELECT `+appColumns+`
		FROM apps
		WHERE organization_id = $1::uuid
		  AND ($2::text = '' OR status = $2)
		  AND ($3::text = '' OR name ILIKE '%' || $3 || '%' OR slug ILIKE '%' || $3 || '%')
		ORDER BY created_at DESC, id DESC
		LIMIT $4 OFFSET $5`, organizationID, status, search, limit, offset)
	if err != nil {
		return nil, ports.PageMeta{}, mapError(err)
	}
	defer rows.Close()

	items := make([]domain.App, 0, limit)
	for rows.Next() {
		app, err := scanApp(rows)
		if err != nil {
			return nil, ports.PageMeta{}, err
		}
		items = append(items, app)
	}
	if err := rows.Err(); err != nil {
		return nil, ports.PageMeta{}, mapError(err)
	}
	return items, pageMeta(offset, limit, total), nil
}

func (r *Repository) CreateApp(ctx context.Context, app domain.App, meta ports.MutationMeta) (domain.App, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.App{}, fmt.Errorf("begin create app: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := r.lockIdempotency(ctx, tx, app.OrganizationID, meta.Action, meta.IdempotencyKey); err != nil {
		return domain.App{}, err
	}
	if existingID, ok, err := r.idempotentResource(ctx, tx, app.OrganizationID, meta.Action, meta.IdempotencyKey); err != nil {
		return domain.App{}, err
	} else if ok {
		return getApp(ctx, tx, app.OrganizationID, existingID, false)
	}

	created, err := scanApp(tx.QueryRow(ctx, `
		INSERT INTO apps (
			id, organization_id, name, slug, description, distribution, status,
			app_url, contact_email, active_version_id, created_by, created_at, updated_at, revision
		) VALUES (
			$1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10::uuid, $11::uuid, $12, $13, $14
		)
		RETURNING `+appColumns,
		app.ID, app.OrganizationID, app.Name, app.Slug, app.Description, app.Distribution, app.Status,
		app.AppURL, app.ContactEmail, app.ActiveVersionID, app.CreatedBy, app.CreatedAt, app.UpdatedAt, app.Revision))
	if err != nil {
		return domain.App{}, mapError(err)
	}
	if err := r.storeIdempotency(ctx, tx, app.OrganizationID, meta.Action, meta.IdempotencyKey, "app", app.ID); err != nil {
		return domain.App{}, err
	}
	if err := r.appendAudit(ctx, tx, app.OrganizationID, meta, "app", app.ID, map[string]any{"slug": app.Slug}); err != nil {
		return domain.App{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.App{}, mapError(err)
	}
	return created, nil
}

func (r *Repository) GetApp(ctx context.Context, organizationID, appID string) (domain.App, error) {
	return getApp(ctx, r.pool, organizationID, appID, false)
}

func (r *Repository) UpdateApp(ctx context.Context, app domain.App, expectedRevision int64, meta ports.MutationMeta) (domain.App, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.App{}, fmt.Errorf("begin update app: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	current, err := getApp(ctx, tx, app.OrganizationID, app.ID, true)
	if err != nil {
		return domain.App{}, err
	}
	if current.Revision != expectedRevision {
		return domain.App{}, fmt.Errorf("%w: expected revision %d, current revision %d", domain.ErrConflict, expectedRevision, current.Revision)
	}
	app.Revision = current.Revision + 1
	app.UpdatedAt = r.now().UTC()
	updated, err := scanApp(tx.QueryRow(ctx, `
		UPDATE apps SET
			name = $3,
			slug = $4,
			description = $5,
			distribution = $6,
			app_url = $7,
			contact_email = $8,
			updated_at = $9,
			revision = $10
		WHERE organization_id = $1::uuid AND id = $2::uuid
		RETURNING `+appColumns,
		app.OrganizationID, app.ID, app.Name, app.Slug, app.Description, app.Distribution,
		app.AppURL, app.ContactEmail, app.UpdatedAt, app.Revision))
	if err != nil {
		return domain.App{}, mapError(err)
	}
	if err := r.appendAudit(ctx, tx, app.OrganizationID, meta, "app", app.ID, map[string]any{"revision": app.Revision}); err != nil {
		return domain.App{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.App{}, mapError(err)
	}
	return updated, nil
}

func (r *Repository) ArchiveApp(ctx context.Context, organizationID, appID string, meta ports.MutationMeta) error {
	action := meta.Action + ":" + appID
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin archive app: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := r.lockIdempotency(ctx, tx, organizationID, action, meta.IdempotencyKey); err != nil {
		return err
	}
	if _, ok, err := r.idempotentResource(ctx, tx, organizationID, action, meta.IdempotencyKey); err != nil {
		return err
	} else if ok {
		return nil
	}
	app, err := getApp(ctx, tx, organizationID, appID, true)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE apps
		SET status = 'archived', revision = revision + 1, updated_at = $3
		WHERE organization_id = $1::uuid AND id = $2::uuid`, organizationID, appID, r.now().UTC()); err != nil {
		return mapError(err)
	}
	if err := r.storeIdempotency(ctx, tx, organizationID, action, meta.IdempotencyKey, "app", appID); err != nil {
		return err
	}
	if err := r.appendAudit(ctx, tx, organizationID, meta, "app", app.ID, nil); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return mapError(err)
	}
	return nil
}

func (r *Repository) ListAuditEvents(ctx context.Context, organizationID string) ([]domain.AuditEvent, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, organization_id::text, actor_id::text, action, resource_type,
		       resource_id::text, metadata, created_at
		FROM audit_events
		WHERE organization_id = $1::uuid
		ORDER BY created_at ASC, id ASC`, organizationID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	items := make([]domain.AuditEvent, 0)
	for rows.Next() {
		var item domain.AuditEvent
		var metadata []byte
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.ActorID, &item.Action, &item.ResourceType, &item.ResourceID, &metadata, &item.CreatedAt); err != nil {
			return nil, mapError(err)
		}
		if err := json.Unmarshal(metadata, &item.Metadata); err != nil {
			return nil, fmt.Errorf("decode audit metadata: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return items, nil
}
