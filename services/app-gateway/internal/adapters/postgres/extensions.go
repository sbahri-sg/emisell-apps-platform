package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
	"github.com/jackc/pgx/v5"
)

const extensionColumns = `
	id::text, app_id::text, name, type, status, runtime_url, configuration,
	created_at, updated_at, revision`

const extensionColumnsAliased = `
	e.id::text, e.app_id::text, e.name, e.type, e.status, e.runtime_url, e.configuration,
	e.created_at, e.updated_at, e.revision`

func scanExtension(row rowScanner) (domain.AppExtension, error) {
	var extension domain.AppExtension
	var extensionType string
	var status string
	var configuration []byte
	if err := row.Scan(
		&extension.ID, &extension.AppID, &extension.Name, &extensionType, &status,
		&extension.RuntimeURL, &configuration, &extension.CreatedAt, &extension.UpdatedAt, &extension.Revision,
	); err != nil {
		return domain.AppExtension{}, mapError(err)
	}
	if err := json.Unmarshal(configuration, &extension.Configuration); err != nil {
		return domain.AppExtension{}, fmt.Errorf("decode extension configuration: %w", err)
	}
	extension.Type = domain.ExtensionType(extensionType)
	extension.Status = domain.ExtensionStatus(status)
	return extension, nil
}

func getExtension(ctx context.Context, source queryRower, organizationID, appID, extensionID string, forUpdate bool) (domain.AppExtension, error) {
	query := `SELECT ` + extensionColumnsAliased + `
		FROM app_extensions e
		JOIN apps a ON a.id = e.app_id
		WHERE a.organization_id = $1::uuid AND a.id = $2::uuid AND e.id = $3::uuid`
	if forUpdate {
		query += ` FOR UPDATE OF e`
	}
	return scanExtension(source.QueryRow(ctx, query, organizationID, appID, extensionID))
}

func (r *Repository) ListExtensions(ctx context.Context, organizationID, appID string) ([]domain.AppExtension, error) {
	if _, err := getApp(ctx, r.pool, organizationID, appID, false); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `SELECT `+extensionColumnsAliased+`
		FROM app_extensions e
		JOIN apps a ON a.id = e.app_id
		WHERE a.organization_id = $1::uuid AND a.id = $2::uuid
		ORDER BY e.created_at DESC, e.id DESC`, organizationID, appID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	items := make([]domain.AppExtension, 0)
	for rows.Next() {
		extension, err := scanExtension(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, extension)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return items, nil
}

func (r *Repository) CreateExtension(ctx context.Context, organizationID string, extension domain.AppExtension, meta ports.MutationMeta) (domain.AppExtension, error) {
	action := meta.Action + ":" + extension.AppID
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.AppExtension{}, fmt.Errorf("begin create extension: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := r.lockIdempotency(ctx, tx, organizationID, action, meta.IdempotencyKey); err != nil {
		return domain.AppExtension{}, err
	}
	if existingID, ok, err := r.idempotentResource(ctx, tx, organizationID, action, meta.IdempotencyKey); err != nil {
		return domain.AppExtension{}, err
	} else if ok {
		return getExtension(ctx, tx, organizationID, extension.AppID, existingID, false)
	}
	if _, err := getApp(ctx, tx, organizationID, extension.AppID, true); err != nil {
		return domain.AppExtension{}, err
	}
	configuration, err := json.Marshal(extension.Configuration)
	if err != nil {
		return domain.AppExtension{}, fmt.Errorf("encode extension configuration: %w", err)
	}
	created, err := scanExtension(tx.QueryRow(ctx, `
		INSERT INTO app_extensions (
			id, app_id, name, type, status, runtime_url, configuration, created_at, updated_at, revision
		) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7::jsonb, $8, $9, $10)
		RETURNING `+extensionColumns,
		extension.ID, extension.AppID, extension.Name, extension.Type, extension.Status,
		extension.RuntimeURL, configuration, extension.CreatedAt, extension.UpdatedAt, extension.Revision))
	if err != nil {
		return domain.AppExtension{}, mapError(err)
	}
	if err := r.storeIdempotency(ctx, tx, organizationID, action, meta.IdempotencyKey, "app_extension", extension.ID); err != nil {
		return domain.AppExtension{}, err
	}
	if err := r.appendAudit(ctx, tx, organizationID, meta, "app_extension", extension.ID, map[string]interface{}{"type": extension.Type}); err != nil {
		return domain.AppExtension{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AppExtension{}, mapError(err)
	}
	return created, nil
}

func (r *Repository) GetExtension(ctx context.Context, organizationID, appID, extensionID string) (domain.AppExtension, error) {
	return getExtension(ctx, r.pool, organizationID, appID, extensionID, false)
}

func (r *Repository) UpdateExtension(ctx context.Context, organizationID string, extension domain.AppExtension, expectedRevision int64, meta ports.MutationMeta) (domain.AppExtension, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.AppExtension{}, fmt.Errorf("begin update extension: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	current, err := getExtension(ctx, tx, organizationID, extension.AppID, extension.ID, true)
	if err != nil {
		return domain.AppExtension{}, err
	}
	if current.Revision != expectedRevision {
		return domain.AppExtension{}, fmt.Errorf("%w: expected revision %d, current revision %d", domain.ErrConflict, expectedRevision, current.Revision)
	}
	configuration, err := json.Marshal(extension.Configuration)
	if err != nil {
		return domain.AppExtension{}, fmt.Errorf("encode extension configuration: %w", err)
	}
	extension.Revision = current.Revision + 1
	extension.UpdatedAt = r.now().UTC()
	updated, err := scanExtension(tx.QueryRow(ctx, `
		UPDATE app_extensions SET
			name = $4, status = $5, runtime_url = $6, configuration = $7::jsonb,
			updated_at = $8, revision = $9
		WHERE app_id = $1::uuid AND id = $2::uuid AND revision = $3
		RETURNING `+extensionColumns,
		extension.AppID, extension.ID, expectedRevision, extension.Name, extension.Status,
		extension.RuntimeURL, configuration, extension.UpdatedAt, extension.Revision))
	if err != nil {
		return domain.AppExtension{}, mapError(err)
	}
	if err := r.appendAudit(ctx, tx, organizationID, meta, "app_extension", extension.ID, map[string]interface{}{"revision": extension.Revision}); err != nil {
		return domain.AppExtension{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AppExtension{}, mapError(err)
	}
	return updated, nil
}

func (r *Repository) DisableExtension(ctx context.Context, organizationID, appID, extensionID string, meta ports.MutationMeta) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin disable extension: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	extension, err := getExtension(ctx, tx, organizationID, appID, extensionID, true)
	if err != nil {
		return err
	}
	if extension.Status == domain.ExtensionStatusDisabled {
		return tx.Commit(ctx)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE app_extensions
		SET status = 'disabled', revision = revision + 1, updated_at = $3
		WHERE app_id = $1::uuid AND id = $2::uuid`, appID, extensionID, r.now().UTC()); err != nil {
		return mapError(err)
	}
	if err := r.appendAudit(ctx, tx, organizationID, meta, "app_extension", extensionID, nil); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return mapError(err)
	}
	return nil
}
