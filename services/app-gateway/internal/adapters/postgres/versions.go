package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) ListVersions(ctx context.Context, organizationID, appID string, filter ports.AppFilter) ([]domain.AppVersion, ports.PageMeta, error) {
	offset, limit, err := pagination(filter)
	if err != nil {
		return nil, ports.PageMeta{}, err
	}
	var total int
	if err := r.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM app_versions v
		JOIN apps a ON a.id = v.app_id
		WHERE a.organization_id = $1::uuid AND a.id = $2::uuid`, organizationID, appID).Scan(&total); err != nil {
		return nil, ports.PageMeta{}, mapError(err)
	}
	if total == 0 {
		var exists bool
		if err := r.pool.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM apps WHERE organization_id = $1::uuid AND id = $2::uuid)`,
			organizationID, appID).Scan(&exists); err != nil {
			return nil, ports.PageMeta{}, mapError(err)
		}
		if !exists {
			return nil, ports.PageMeta{}, domain.ErrNotFound
		}
	}
	if offset > total {
		return nil, ports.PageMeta{}, fmt.Errorf("%w: cursor is out of range", domain.ErrValidation)
	}
	rows, err := r.pool.Query(ctx, `SELECT `+versionColumnsAliased+`
		FROM app_versions v
		JOIN apps a ON a.id = v.app_id
		WHERE a.organization_id = $1::uuid AND a.id = $2::uuid
		ORDER BY v.created_at DESC, v.id DESC
		LIMIT $3 OFFSET $4`, organizationID, appID, limit, offset)
	if err != nil {
		return nil, ports.PageMeta{}, mapError(err)
	}
	defer rows.Close()

	items := make([]domain.AppVersion, 0, limit)
	for rows.Next() {
		version, err := scanVersion(rows)
		if err != nil {
			return nil, ports.PageMeta{}, err
		}
		items = append(items, version)
	}
	if err := rows.Err(); err != nil {
		return nil, ports.PageMeta{}, mapError(err)
	}
	return items, pageMeta(offset, limit, total), nil
}

func (r *Repository) CreateVersion(ctx context.Context, organizationID string, version domain.AppVersion, meta ports.MutationMeta) (domain.AppVersion, error) {
	action := meta.Action + ":" + version.AppID
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.AppVersion{}, fmt.Errorf("begin create version: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := r.lockIdempotency(ctx, tx, organizationID, action, meta.IdempotencyKey); err != nil {
		return domain.AppVersion{}, err
	}
	if existingID, ok, err := r.idempotentResource(ctx, tx, organizationID, action, meta.IdempotencyKey); err != nil {
		return domain.AppVersion{}, err
	} else if ok {
		return getVersion(ctx, tx, organizationID, version.AppID, existingID, false)
	}
	app, err := getApp(ctx, tx, organizationID, version.AppID, true)
	if err != nil {
		return domain.AppVersion{}, err
	}
	if app.Status == domain.AppStatusArchived {
		return domain.AppVersion{}, fmt.Errorf("%w: archived apps cannot create versions", domain.ErrConflict)
	}
	snapshot, err := json.Marshal(version.Snapshot)
	if err != nil {
		return domain.AppVersion{}, fmt.Errorf("encode version snapshot: %w", err)
	}
	created, err := scanVersion(tx.QueryRow(ctx, `
		INSERT INTO app_versions (
			id, app_id, version, status, release_note, snapshot, created_by,
			released_by, created_at, released_at
		) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6::jsonb, $7::uuid, $8::uuid, $9, $10)
		RETURNING `+versionColumns,
		version.ID, version.AppID, version.Version, version.Status, version.ReleaseNote, snapshot,
		version.CreatedBy, version.ReleasedBy, version.CreatedAt, version.ReleasedAt))
	if err != nil {
		return domain.AppVersion{}, mapError(err)
	}
	if err := r.storeIdempotency(ctx, tx, organizationID, action, meta.IdempotencyKey, "app_version", version.ID); err != nil {
		return domain.AppVersion{}, err
	}
	if err := r.appendAudit(ctx, tx, organizationID, meta, "app_version", version.ID, map[string]any{"version": version.Version}); err != nil {
		return domain.AppVersion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AppVersion{}, mapError(err)
	}
	return created, nil
}

func (r *Repository) GetVersion(ctx context.Context, organizationID, appID, versionID string) (domain.AppVersion, error) {
	return getVersion(ctx, r.pool, organizationID, appID, versionID, false)
}

func (r *Repository) ActivateVersion(ctx context.Context, organizationID, appID, versionID string, expectedStatus domain.VersionStatus, expectedActiveVersionID *string, meta ports.MutationMeta) (domain.AppVersion, error) {
	action := meta.Action + ":" + versionID
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return domain.AppVersion{}, fmt.Errorf("begin activate version: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := r.lockIdempotency(ctx, tx, organizationID, action, meta.IdempotencyKey); err != nil {
		return domain.AppVersion{}, err
	}
	if existingID, ok, err := r.idempotentResource(ctx, tx, organizationID, action, meta.IdempotencyKey); err != nil {
		return domain.AppVersion{}, err
	} else if ok {
		return getVersion(ctx, tx, organizationID, appID, existingID, false)
	}
	app, err := getApp(ctx, tx, organizationID, appID, true)
	if err != nil {
		return domain.AppVersion{}, err
	}
	if expectedActiveVersionID != nil && (app.ActiveVersionID == nil || *app.ActiveVersionID != *expectedActiveVersionID) {
		return domain.AppVersion{}, fmt.Errorf("%w: active version changed", domain.ErrConflict)
	}
	target, err := getVersion(ctx, tx, organizationID, appID, versionID, true)
	if err != nil {
		return domain.AppVersion{}, err
	}
	if target.Status != expectedStatus {
		return domain.AppVersion{}, fmt.Errorf("%w: expected version status %s, current status %s", domain.ErrConflict, expectedStatus, target.Status)
	}

	now := r.now().UTC()
	if _, err := tx.Exec(ctx, `
		UPDATE app_versions
		SET status = 'released'
		WHERE app_id = $1::uuid AND status = 'active'`, appID); err != nil {
		return domain.AppVersion{}, mapError(err)
	}
	activated, err := scanVersion(tx.QueryRow(ctx, `
		UPDATE app_versions
		SET status = 'active', released_by = $3::uuid, released_at = $4
		WHERE app_id = $1::uuid AND id = $2::uuid
		RETURNING `+versionColumns, appID, versionID, meta.ActorID, now))
	if err != nil {
		return domain.AppVersion{}, mapError(err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE apps
		SET status = 'active', active_version_id = $3::uuid, revision = revision + 1, updated_at = $4
		WHERE organization_id = $1::uuid AND id = $2::uuid`, organizationID, appID, versionID, now); err != nil {
		return domain.AppVersion{}, mapError(err)
	}
	extensionIDs := make([]string, 0, len(target.Snapshot.Extensions))
	for _, extension := range target.Snapshot.Extensions {
		extensionIDs = append(extensionIDs, extension.ExtensionID)
	}
	if len(extensionIDs) > 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE app_extensions
			SET status = 'active', updated_at = $3
			WHERE app_id = $1::uuid AND id = ANY($2::uuid[])`, appID, extensionIDs, now); err != nil {
			return domain.AppVersion{}, mapError(err)
		}
	}
	if err := r.storeIdempotency(ctx, tx, organizationID, action, meta.IdempotencyKey, "app_version", versionID); err != nil {
		return domain.AppVersion{}, err
	}
	if err := r.appendAudit(ctx, tx, organizationID, meta, "app_version", versionID, map[string]any{"version": activated.Version}); err != nil {
		return domain.AppVersion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AppVersion{}, mapError(err)
	}
	return activated, nil
}
