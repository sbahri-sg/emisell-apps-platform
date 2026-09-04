package postgres

import (
	"context"
	"fmt"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
	"github.com/jackc/pgx/v5"
)

const webhookColumns = `
	id::text, app_id::text, event, endpoint_url, status, signing_secret_fingerprint,
	created_at, updated_at, revision, signing_secret_ciphertext, encryption_key_version`

const webhookColumnsAliased = `
	w.id::text, w.app_id::text, w.event, w.endpoint_url, w.status, w.signing_secret_fingerprint,
	w.created_at, w.updated_at, w.revision, w.signing_secret_ciphertext, w.encryption_key_version`

func scanWebhook(row rowScanner) (domain.WebhookSubscription, error) {
	var subscription domain.WebhookSubscription
	var status string
	if err := row.Scan(
		&subscription.ID, &subscription.AppID, &subscription.Event, &subscription.EndpointURL,
		&status, &subscription.SigningSecretFingerprint, &subscription.CreatedAt, &subscription.UpdatedAt,
		&subscription.Revision, &subscription.SigningSecretCiphertext, &subscription.EncryptionKeyVersion,
	); err != nil {
		return domain.WebhookSubscription{}, mapError(err)
	}
	subscription.Status = domain.WebhookStatus(status)
	return subscription, nil
}

func getWebhook(ctx context.Context, source queryRower, organizationID, appID, webhookID string, forUpdate bool) (domain.WebhookSubscription, error) {
	query := `SELECT ` + webhookColumnsAliased + `
		FROM webhook_subscriptions w
		JOIN apps a ON a.id = w.app_id
		WHERE a.organization_id = $1::uuid AND a.id = $2::uuid AND w.id = $3::uuid AND w.status <> 'disabled'`
	if forUpdate {
		query += ` FOR UPDATE OF w`
	}
	return scanWebhook(source.QueryRow(ctx, query, organizationID, appID, webhookID))
}

func (r *Repository) ListWebhooks(ctx context.Context, organizationID, appID string, filter ports.AppFilter) ([]domain.WebhookSubscription, ports.PageMeta, error) {
	if _, err := getApp(ctx, r.pool, organizationID, appID, false); err != nil {
		return nil, ports.PageMeta{}, err
	}
	offset, limit, err := pagination(filter)
	if err != nil {
		return nil, ports.PageMeta{}, err
	}
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM webhook_subscriptions w JOIN apps a ON a.id = w.app_id WHERE a.organization_id = $1::uuid AND a.id = $2::uuid AND w.status <> 'disabled'`, organizationID, appID).Scan(&total); err != nil {
		return nil, ports.PageMeta{}, mapError(err)
	}
	rows, err := r.pool.Query(ctx, `SELECT `+webhookColumnsAliased+`
		FROM webhook_subscriptions w
		JOIN apps a ON a.id = w.app_id
		WHERE a.organization_id = $1::uuid AND a.id = $2::uuid AND w.status <> 'disabled'
		ORDER BY w.created_at DESC, w.id DESC OFFSET $3 LIMIT $4`, organizationID, appID, offset, limit)
	if err != nil {
		return nil, ports.PageMeta{}, mapError(err)
	}
	defer rows.Close()
	items := make([]domain.WebhookSubscription, 0)
	for rows.Next() {
		subscription, err := scanWebhook(rows)
		if err != nil {
			return nil, ports.PageMeta{}, err
		}
		items = append(items, subscription)
	}
	if err := rows.Err(); err != nil {
		return nil, ports.PageMeta{}, mapError(err)
	}
	return items, pageMeta(offset, limit, total), nil
}

func (r *Repository) CreateWebhook(ctx context.Context, organizationID string, subscription domain.WebhookSubscription, meta ports.MutationMeta) (domain.WebhookSubscription, error) {
	action := meta.Action + ":" + subscription.AppID
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.WebhookSubscription{}, fmt.Errorf("begin create webhook: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := r.lockIdempotency(ctx, tx, organizationID, action, meta.IdempotencyKey); err != nil {
		return domain.WebhookSubscription{}, err
	}
	if existingID, ok, err := r.idempotentResource(ctx, tx, organizationID, action, meta.IdempotencyKey); err != nil {
		return domain.WebhookSubscription{}, err
	} else if ok {
		return getWebhook(ctx, tx, organizationID, subscription.AppID, existingID, false)
	}
	if _, err := getApp(ctx, tx, organizationID, subscription.AppID, true); err != nil {
		return domain.WebhookSubscription{}, err
	}
	created, err := scanWebhook(tx.QueryRow(ctx, `
		INSERT INTO webhook_subscriptions (
			id, app_id, event, endpoint_url, status, signing_secret_ciphertext,
			signing_secret_fingerprint, encryption_key_version, created_at, updated_at, revision
		) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING `+webhookColumns,
		subscription.ID, subscription.AppID, subscription.Event, subscription.EndpointURL,
		subscription.Status, subscription.SigningSecretCiphertext, subscription.SigningSecretFingerprint,
		subscription.EncryptionKeyVersion, subscription.CreatedAt, subscription.UpdatedAt, subscription.Revision))
	if err != nil {
		return domain.WebhookSubscription{}, mapError(err)
	}
	if err := r.storeIdempotency(ctx, tx, organizationID, action, meta.IdempotencyKey, "webhook_subscription", subscription.ID); err != nil {
		return domain.WebhookSubscription{}, err
	}
	if err := r.appendAudit(ctx, tx, organizationID, meta, "webhook_subscription", subscription.ID, map[string]any{"event": subscription.Event}); err != nil {
		return domain.WebhookSubscription{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.WebhookSubscription{}, mapError(err)
	}
	return created, nil
}

func (r *Repository) GetWebhook(ctx context.Context, organizationID, appID, webhookID string) (domain.WebhookSubscription, error) {
	return getWebhook(ctx, r.pool, organizationID, appID, webhookID, false)
}

func (r *Repository) UpdateWebhook(ctx context.Context, organizationID string, subscription domain.WebhookSubscription, expectedRevision int64, meta ports.MutationMeta) (domain.WebhookSubscription, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.WebhookSubscription{}, fmt.Errorf("begin update webhook: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	current, err := getWebhook(ctx, tx, organizationID, subscription.AppID, subscription.ID, true)
	if err != nil {
		return domain.WebhookSubscription{}, err
	}
	if current.Revision != expectedRevision {
		return domain.WebhookSubscription{}, fmt.Errorf("%w: expected revision %d, current revision %d", domain.ErrConflict, expectedRevision, current.Revision)
	}
	subscription.Revision = current.Revision + 1
	subscription.UpdatedAt = r.now().UTC()
	updated, err := scanWebhook(tx.QueryRow(ctx, `
		UPDATE webhook_subscriptions
		SET endpoint_url = $4, status = $5, updated_at = $6, revision = $7
		WHERE app_id = $1::uuid AND id = $2::uuid AND revision = $3
		RETURNING `+webhookColumns,
		subscription.AppID, subscription.ID, expectedRevision, subscription.EndpointURL,
		subscription.Status, subscription.UpdatedAt, subscription.Revision))
	if err != nil {
		return domain.WebhookSubscription{}, mapError(err)
	}
	if err := r.appendAudit(ctx, tx, organizationID, meta, "webhook_subscription", subscription.ID, map[string]any{"revision": subscription.Revision, "status": subscription.Status}); err != nil {
		return domain.WebhookSubscription{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.WebhookSubscription{}, mapError(err)
	}
	return updated, nil
}

func (r *Repository) DisableWebhook(ctx context.Context, organizationID, appID, webhookID string, meta ports.MutationMeta) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin disable webhook: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := getWebhook(ctx, tx, organizationID, appID, webhookID, true); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE webhook_subscriptions SET status = 'disabled', updated_at = $3, revision = revision + 1 WHERE app_id = $1::uuid AND id = $2::uuid`, appID, webhookID, r.now().UTC()); err != nil {
		return mapError(err)
	}
	if err := r.appendAudit(ctx, tx, organizationID, meta, "webhook_subscription", webhookID, nil); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
