package postgres

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/ports"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
	id   ids.Generator
	now  func() time.Time
}

func NewRepository(pool *pgxpool.Pool, id ids.Generator, now func() time.Time) *Repository {
	return &Repository{pool: pool, id: id, now: now}
}

type rowScanner interface {
	Scan(dest ...any) error
}

type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

const appColumns = `
	id::text, organization_id::text, name, slug, description, distribution,
	status, app_url, contact_email, active_version_id::text, created_by::text,
	created_at, updated_at, revision`

const versionColumns = `
	id::text, app_id::text, version, status, release_note, snapshot,
	created_by::text, released_by::text, created_at, released_at`

const versionColumnsAliased = `
	v.id::text, v.app_id::text, v.version, v.status, v.release_note, v.snapshot,
	v.created_by::text, v.released_by::text, v.created_at, v.released_at`

func scanApp(row rowScanner) (domain.App, error) {
	var app domain.App
	var distribution string
	var status string
	if err := row.Scan(
		&app.ID,
		&app.OrganizationID,
		&app.Name,
		&app.Slug,
		&app.Description,
		&distribution,
		&status,
		&app.AppURL,
		&app.ContactEmail,
		&app.ActiveVersionID,
		&app.CreatedBy,
		&app.CreatedAt,
		&app.UpdatedAt,
		&app.Revision,
	); err != nil {
		return domain.App{}, mapError(err)
	}
	app.Distribution = domain.Distribution(distribution)
	app.Status = domain.AppStatus(status)
	return app, nil
}

func scanVersion(row rowScanner) (domain.AppVersion, error) {
	var version domain.AppVersion
	var status string
	var snapshot []byte
	if err := row.Scan(
		&version.ID,
		&version.AppID,
		&version.Version,
		&status,
		&version.ReleaseNote,
		&snapshot,
		&version.CreatedBy,
		&version.ReleasedBy,
		&version.CreatedAt,
		&version.ReleasedAt,
	); err != nil {
		return domain.AppVersion{}, mapError(err)
	}
	if err := json.Unmarshal(snapshot, &version.Snapshot); err != nil {
		return domain.AppVersion{}, fmt.Errorf("decode version snapshot: %w", err)
	}
	version.Status = domain.VersionStatus(status)
	return version, nil
}

func getApp(ctx context.Context, source queryRower, organizationID, appID string, forUpdate bool) (domain.App, error) {
	query := `SELECT ` + appColumns + ` FROM apps WHERE organization_id = $1::uuid AND id = $2::uuid`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	return scanApp(source.QueryRow(ctx, query, organizationID, appID))
}

func getVersion(ctx context.Context, source queryRower, organizationID, appID, versionID string, forUpdate bool) (domain.AppVersion, error) {
	query := `SELECT ` + versionColumnsAliased + `
		FROM app_versions v
		JOIN apps a ON a.id = v.app_id
		WHERE a.organization_id = $1::uuid AND a.id = $2::uuid AND v.id = $3::uuid`
	if forUpdate {
		query += ` FOR UPDATE OF v`
	}
	return scanVersion(source.QueryRow(ctx, query, organizationID, appID, versionID))
}

func (r *Repository) lockIdempotency(ctx context.Context, tx pgx.Tx, organizationID, action, key string) error {
	if key == "" {
		return nil
	}
	lockKey := organizationID + ":" + action + ":" + key
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
		return fmt.Errorf("lock idempotency key: %w", err)
	}
	return nil
}

func (r *Repository) idempotentResource(ctx context.Context, tx pgx.Tx, organizationID, action, key string) (string, bool, error) {
	if key == "" {
		return "", false, nil
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM idempotency_keys
		WHERE organization_id = $1::uuid AND action = $2 AND key = $3 AND expires_at <= now()`, organizationID, action, key); err != nil {
		return "", false, fmt.Errorf("expire idempotency key: %w", err)
	}
	var resourceID string
	err := tx.QueryRow(ctx, `
		SELECT resource_id::text
		FROM idempotency_keys
		WHERE organization_id = $1::uuid AND action = $2 AND key = $3`, organizationID, action, key).Scan(&resourceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read idempotency key: %w", err)
	}
	return resourceID, true, nil
}

func (r *Repository) storeIdempotency(ctx context.Context, tx pgx.Tx, organizationID, action, key, resourceType, resourceID string) error {
	if key == "" {
		return nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO idempotency_keys (organization_id, action, key, resource_type, resource_id)
		VALUES ($1::uuid, $2, $3, $4, $5::uuid)`, organizationID, action, key, resourceType, resourceID); err != nil {
		return mapError(err)
	}
	return nil
}

func (r *Repository) appendAudit(ctx context.Context, tx pgx.Tx, organizationID string, meta ports.MutationMeta, resourceType, resourceID string, metadata map[string]any) error {
	auditID, err := r.id()
	if err != nil {
		return fmt.Errorf("generate audit id: %w", err)
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("encode audit metadata: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_events (
			id, organization_id, actor_id, action, resource_type, resource_id, metadata, created_at
		) VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6::uuid, $7::jsonb, $8)`,
		auditID, organizationID, meta.ActorID, meta.Action, resourceType, resourceID, encoded, r.now().UTC()); err != nil {
		return mapError(err)
	}
	return nil
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.Code {
		case "23505":
			return fmt.Errorf("%w: uniqueness constraint %s", domain.ErrConflict, postgresError.ConstraintName)
		case "23503":
			return fmt.Errorf("%w: referenced resource does not exist", domain.ErrConflict)
		case "23514", "22P02":
			return fmt.Errorf("%w: invalid persisted value", domain.ErrValidation)
		case "40001", "40P01":
			return fmt.Errorf("%w: concurrent transaction; retry the request", domain.ErrConflict)
		}
	}
	return err
}

func decodeCursor(cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, fmt.Errorf("%w: invalid cursor", domain.ErrValidation)
	}
	offset, err := strconv.Atoi(string(decoded))
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("%w: invalid cursor", domain.ErrValidation)
	}
	return offset, nil
}

func encodeCursor(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}

func pagination(filter ports.AppFilter) (offset, limit int, err error) {
	offset, err = decodeCursor(filter.Cursor)
	if err != nil {
		return 0, 0, err
	}
	limit = filter.Limit
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	return offset, limit, nil
}

func pageMeta(offset, limit, total int) ports.PageMeta {
	meta := ports.PageMeta{HasMore: offset+limit < total}
	if meta.HasMore {
		next := encodeCursor(offset + limit)
		meta.NextCursor = &next
	}
	return meta
}

var _ ports.Repository = (*Repository)(nil)
