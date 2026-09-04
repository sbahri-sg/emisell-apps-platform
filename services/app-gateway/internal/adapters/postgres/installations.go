package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
	"github.com/jackc/pgx/v5"
)

const installationColumns = `
	id::text, app_id::text, merchant_id::text, merchant_name, merchant_domain,
	environment, status, installed_version_id::text, granted_scopes, installed_by::text,
	installed_at, created_at, updated_at, uninstalled_at, revision`

const installationColumnsAliased = `
	i.id::text, i.app_id::text, i.merchant_id::text, i.merchant_name, i.merchant_domain,
	i.environment, i.status, i.installed_version_id::text, i.granted_scopes, i.installed_by::text,
	i.installed_at, i.created_at, i.updated_at, i.uninstalled_at, i.revision`

func scanInstallation(row rowScanner) (domain.AppInstallation, error) {
	var installation domain.AppInstallation
	var environment, status string
	var grantedScopes []byte
	if err := row.Scan(
		&installation.ID, &installation.AppID, &installation.MerchantID, &installation.MerchantName,
		&installation.MerchantDomain, &environment, &status, &installation.InstalledVersionID,
		&grantedScopes, &installation.InstalledBy, &installation.InstalledAt, &installation.CreatedAt,
		&installation.UpdatedAt, &installation.UninstalledAt, &installation.Revision,
	); err != nil {
		return domain.AppInstallation{}, mapError(err)
	}
	if err := json.Unmarshal(grantedScopes, &installation.GrantedScopes); err != nil {
		return domain.AppInstallation{}, fmt.Errorf("decode granted scopes: %w", err)
	}
	installation.Environment = domain.Environment(environment)
	installation.Status = domain.InstallationStatus(status)
	return installation, nil
}

func getInstallation(ctx context.Context, source queryRower, organizationID, appID, installationID string, forUpdate bool) (domain.AppInstallation, error) {
	query := `SELECT ` + installationColumnsAliased + `
		FROM app_installations i
		JOIN apps a ON a.id = i.app_id
		WHERE a.organization_id = $1::uuid AND a.id = $2::uuid AND i.id = $3::uuid`
	if forUpdate {
		query += ` FOR UPDATE OF i`
	}
	return scanInstallation(source.QueryRow(ctx, query, organizationID, appID, installationID))
}

func (r *Repository) ListInstallations(ctx context.Context, organizationID, appID string, filter ports.AppFilter) ([]domain.AppInstallation, ports.PageMeta, error) {
	if _, err := getApp(ctx, r.pool, organizationID, appID, false); err != nil {
		return nil, ports.PageMeta{}, err
	}
	offset, limit, err := pagination(filter)
	if err != nil {
		return nil, ports.PageMeta{}, err
	}
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM app_installations i JOIN apps a ON a.id = i.app_id WHERE a.organization_id = $1::uuid AND a.id = $2::uuid`, organizationID, appID).Scan(&total); err != nil {
		return nil, ports.PageMeta{}, mapError(err)
	}
	rows, err := r.pool.Query(ctx, `SELECT `+installationColumnsAliased+`
		FROM app_installations i
		JOIN apps a ON a.id = i.app_id
		WHERE a.organization_id = $1::uuid AND a.id = $2::uuid
		ORDER BY i.created_at DESC, i.id DESC OFFSET $3 LIMIT $4`, organizationID, appID, offset, limit)
	if err != nil {
		return nil, ports.PageMeta{}, mapError(err)
	}
	defer rows.Close()
	items := make([]domain.AppInstallation, 0)
	for rows.Next() {
		installation, err := scanInstallation(rows)
		if err != nil {
			return nil, ports.PageMeta{}, err
		}
		items = append(items, installation)
	}
	if err := rows.Err(); err != nil {
		return nil, ports.PageMeta{}, mapError(err)
	}
	return items, pageMeta(offset, limit, total), nil
}

func (r *Repository) CreateInstallation(ctx context.Context, organizationID string, installation domain.AppInstallation, expectedActiveVersionID string, meta ports.MutationMeta) (domain.AppInstallation, error) {
	action := meta.Action + ":" + installation.AppID
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.AppInstallation{}, fmt.Errorf("begin create installation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := r.lockIdempotency(ctx, tx, organizationID, action, meta.IdempotencyKey); err != nil {
		return domain.AppInstallation{}, err
	}
	if existingID, ok, err := r.idempotentResource(ctx, tx, organizationID, action, meta.IdempotencyKey); err != nil {
		return domain.AppInstallation{}, err
	} else if ok {
		return getInstallation(ctx, tx, organizationID, installation.AppID, existingID, false)
	}
	app, err := getApp(ctx, tx, organizationID, installation.AppID, true)
	if err != nil {
		return domain.AppInstallation{}, err
	}
	if app.ActiveVersionID == nil || *app.ActiveVersionID != expectedActiveVersionID {
		return domain.AppInstallation{}, fmt.Errorf("%w: active version changed", domain.ErrConflict)
	}
	grantedScopes, err := json.Marshal(installation.GrantedScopes)
	if err != nil {
		return domain.AppInstallation{}, fmt.Errorf("encode granted scopes: %w", err)
	}
	created, err := scanInstallation(tx.QueryRow(ctx, `
		INSERT INTO app_installations (
			id, app_id, merchant_id, merchant_name, merchant_domain, environment, status,
			installed_version_id, granted_scopes, installed_by, installed_at,
			created_at, updated_at, revision
		) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8::uuid, $9::jsonb, $10::uuid, $11, $12, $13, $14)
		RETURNING `+installationColumns,
		installation.ID, installation.AppID, installation.MerchantID, installation.MerchantName,
		installation.MerchantDomain, installation.Environment, installation.Status, installation.InstalledVersionID,
		grantedScopes, installation.InstalledBy, installation.InstalledAt, installation.CreatedAt,
		installation.UpdatedAt, installation.Revision))
	if err != nil {
		return domain.AppInstallation{}, mapError(err)
	}
	if err := r.storeIdempotency(ctx, tx, organizationID, action, meta.IdempotencyKey, "app_installation", installation.ID); err != nil {
		return domain.AppInstallation{}, err
	}
	if err := r.appendAudit(ctx, tx, organizationID, meta, "app_installation", installation.ID, map[string]any{"merchantId": installation.MerchantID, "environment": installation.Environment, "versionId": installation.InstalledVersionID}); err != nil {
		return domain.AppInstallation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AppInstallation{}, mapError(err)
	}
	return created, nil
}

func (r *Repository) GetInstallation(ctx context.Context, organizationID, appID, installationID string) (domain.AppInstallation, error) {
	return getInstallation(ctx, r.pool, organizationID, appID, installationID, false)
}

func (r *Repository) UpdateInstallation(ctx context.Context, organizationID string, installation domain.AppInstallation, expectedRevision int64, meta ports.MutationMeta) (domain.AppInstallation, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.AppInstallation{}, fmt.Errorf("begin update installation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	current, err := getInstallation(ctx, tx, organizationID, installation.AppID, installation.ID, true)
	if err != nil {
		return domain.AppInstallation{}, err
	}
	if current.Revision != expectedRevision {
		return domain.AppInstallation{}, fmt.Errorf("%w: expected revision %d, current revision %d", domain.ErrConflict, expectedRevision, current.Revision)
	}
	if current.Status == domain.InstallationStatusUninstalled {
		return domain.AppInstallation{}, fmt.Errorf("%w: uninstalled records cannot be updated", domain.ErrConflict)
	}
	installation.Revision = current.Revision + 1
	installation.UpdatedAt = r.now().UTC()
	updated, err := scanInstallation(tx.QueryRow(ctx, `
		UPDATE app_installations SET status = $4, updated_at = $5, revision = $6
		WHERE app_id = $1::uuid AND id = $2::uuid AND revision = $3
		RETURNING `+installationColumns,
		installation.AppID, installation.ID, expectedRevision, installation.Status, installation.UpdatedAt, installation.Revision))
	if err != nil {
		return domain.AppInstallation{}, mapError(err)
	}
	if err := r.appendAudit(ctx, tx, organizationID, meta, "app_installation", installation.ID, map[string]any{"status": installation.Status, "revision": installation.Revision}); err != nil {
		return domain.AppInstallation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AppInstallation{}, mapError(err)
	}
	return updated, nil
}

func (r *Repository) UpgradeInstallation(ctx context.Context, organizationID, appID, installationID, targetVersionID, expectedInstalledVersionID string, grantedScopes []string, expectedRevision int64, meta ports.MutationMeta) (domain.AppInstallation, error) {
	action := meta.Action + ":" + installationID
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.AppInstallation{}, fmt.Errorf("begin upgrade installation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := r.lockIdempotency(ctx, tx, organizationID, action, meta.IdempotencyKey); err != nil {
		return domain.AppInstallation{}, err
	}
	if existingID, ok, err := r.idempotentResource(ctx, tx, organizationID, action, meta.IdempotencyKey); err != nil {
		return domain.AppInstallation{}, err
	} else if ok {
		return getInstallation(ctx, tx, organizationID, appID, existingID, false)
	}
	app, err := getApp(ctx, tx, organizationID, appID, true)
	if err != nil {
		return domain.AppInstallation{}, err
	}
	if app.ActiveVersionID == nil || *app.ActiveVersionID != targetVersionID {
		return domain.AppInstallation{}, fmt.Errorf("%w: target version is no longer active", domain.ErrConflict)
	}
	current, err := getInstallation(ctx, tx, organizationID, appID, installationID, true)
	if err != nil {
		return domain.AppInstallation{}, err
	}
	if current.Status == domain.InstallationStatusUninstalled {
		return domain.AppInstallation{}, fmt.Errorf("%w: uninstalled records cannot be upgraded", domain.ErrConflict)
	}
	if current.Revision != expectedRevision {
		return domain.AppInstallation{}, fmt.Errorf("%w: expected revision %d, current revision %d", domain.ErrConflict, expectedRevision, current.Revision)
	}
	if current.InstalledVersionID != expectedInstalledVersionID {
		return domain.AppInstallation{}, fmt.Errorf("%w: installed version changed", domain.ErrConflict)
	}
	if current.InstalledVersionID == targetVersionID {
		return domain.AppInstallation{}, fmt.Errorf("%w: installation already uses the active version", domain.ErrConflict)
	}
	encodedScopes, err := json.Marshal(grantedScopes)
	if err != nil {
		return domain.AppInstallation{}, fmt.Errorf("encode granted scopes: %w", err)
	}
	now := r.now().UTC()
	updated, err := scanInstallation(tx.QueryRow(ctx, `
		UPDATE app_installations
		SET installed_version_id = $4::uuid, granted_scopes = $5::jsonb, updated_at = $6, revision = $7
		WHERE app_id = $1::uuid AND id = $2::uuid AND revision = $3
		RETURNING `+installationColumns,
		appID, installationID, expectedRevision, targetVersionID, encodedScopes, now, current.Revision+1))
	if err != nil {
		return domain.AppInstallation{}, mapError(err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE oauth_access_tokens
		SET scopes = $2::jsonb
		WHERE installation_id = $1::uuid AND revoked_at IS NULL`, installationID, encodedScopes); err != nil {
		return domain.AppInstallation{}, mapError(err)
	}
	if err := r.storeIdempotency(ctx, tx, organizationID, action, meta.IdempotencyKey, "app_installation", installationID); err != nil {
		return domain.AppInstallation{}, err
	}
	if err := r.appendAudit(ctx, tx, organizationID, meta, "app_installation", installationID, map[string]any{
		"fromVersionId": current.InstalledVersionID,
		"toVersionId":   targetVersionID,
		"grantedScopes": grantedScopes,
		"revision":      updated.Revision,
	}); err != nil {
		return domain.AppInstallation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AppInstallation{}, mapError(err)
	}
	return updated, nil
}

func (r *Repository) UninstallInstallation(ctx context.Context, organizationID, appID, installationID string, meta ports.MutationMeta) error {
	action := meta.Action + ":" + installationID
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin uninstall: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := r.lockIdempotency(ctx, tx, organizationID, action, meta.IdempotencyKey); err != nil {
		return err
	}
	if _, ok, err := r.idempotentResource(ctx, tx, organizationID, action, meta.IdempotencyKey); err != nil {
		return err
	} else if ok {
		return tx.Commit(ctx)
	}
	installation, err := getInstallation(ctx, tx, organizationID, appID, installationID, true)
	if err != nil {
		return err
	}
	if installation.Status != domain.InstallationStatusUninstalled {
		uninstalledAt := r.now().UTC()
		if _, err := tx.Exec(ctx, `
			UPDATE app_installations
			SET status = 'uninstalled', uninstalled_at = $3, updated_at = $3, revision = revision + 1
			WHERE app_id = $1::uuid AND id = $2::uuid`, appID, installationID, uninstalledAt); err != nil {
			return mapError(err)
		}
		if _, err := tx.Exec(ctx, `UPDATE oauth_access_tokens SET revoked_at = $2 WHERE installation_id = $1::uuid AND revoked_at IS NULL`, installationID, uninstalledAt); err != nil {
			return mapError(err)
		}
		if err := r.enqueueAppUninstalledEvent(ctx, tx, organizationID, installation, uninstalledAt); err != nil {
			return err
		}
		if err := r.appendAudit(ctx, tx, organizationID, meta, "app_installation", installationID, nil); err != nil {
			return err
		}
	}
	if err := r.storeIdempotency(ctx, tx, organizationID, action, meta.IdempotencyKey, "app_installation", installationID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) enqueueAppUninstalledEvent(ctx context.Context, tx pgx.Tx, organizationID string, installation domain.AppInstallation, uninstalledAt time.Time) error {
	version, err := getVersion(ctx, tx, organizationID, installation.AppID, installation.InstalledVersionID, false)
	if err != nil {
		return err
	}
	subscriptionIDs := make([]string, 0)
	for _, snapshot := range version.Snapshot.WebhookSubscriptions {
		if snapshot.Event != "app/uninstalled" {
			continue
		}
		subscription, err := getWebhook(ctx, tx, organizationID, installation.AppID, snapshot.SubscriptionID, true)
		if errors.Is(err, domain.ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if subscription.Status == domain.WebhookStatusActive || subscription.Status == domain.WebhookStatusFailing {
			subscriptionIDs = append(subscriptionIDs, subscription.ID)
		}
	}
	if len(subscriptionIDs) == 0 {
		return nil
	}
	eventID, err := r.id()
	if err != nil {
		return fmt.Errorf("generate app uninstall event id: %w", err)
	}
	payload, err := json.Marshal(map[string]any{
		"appId": installation.AppID, "uninstalledAt": uninstalledAt,
	})
	if err != nil {
		return fmt.Errorf("encode app uninstall event: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO webhook_events (id, app_id, event, source, merchant_id, installation_id, payload, created_at)
		VALUES ($1::uuid, $2::uuid, 'app/uninstalled', 'app_platform', $3, $4::uuid, $5::jsonb, $6)`,
		eventID, installation.AppID, installation.MerchantID, installation.ID, payload, uninstalledAt); err != nil {
		return mapError(err)
	}
	for _, subscriptionID := range subscriptionIDs {
		deliveryID, err := r.id()
		if err != nil {
			return fmt.Errorf("generate app uninstall delivery id: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO webhook_deliveries (id, subscription_id, event_id, attempt, status, attempted_at, next_attempt_at)
			VALUES ($1::uuid, $2::uuid, $3::uuid, 1, 'pending', $4, $4)`,
			deliveryID, subscriptionID, eventID, uninstalledAt); err != nil {
			return mapError(err)
		}
	}
	return nil
}
