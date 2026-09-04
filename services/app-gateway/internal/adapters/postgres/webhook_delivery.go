package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
	"github.com/jackc/pgx/v5"
)

const webhookDeliveryColumns = `
	d.id::text, d.subscription_id::text, d.event_id::text, e.event, e.source, e.merchant_id, e.installation_id::text, e.created_at, d.attempt, d.status,
	d.response_status, d.response_time_ms, d.error_code, d.attempted_at,
	d.next_attempt_at, d.completed_at, w.endpoint_url, e.payload,
	w.signing_secret_ciphertext, w.encryption_key_version`

func scanWebhookDelivery(row rowScanner) (domain.WebhookDelivery, error) {
	var delivery domain.WebhookDelivery
	var status string
	var payload []byte
	if err := row.Scan(
		&delivery.ID, &delivery.SubscriptionID, &delivery.EventID, &delivery.Event, &delivery.EventSource,
		&delivery.MerchantID, &delivery.InstallationID, &delivery.EventCreatedAt,
		&delivery.Attempt, &status, &delivery.ResponseStatus, &delivery.ResponseTimeMS,
		&delivery.ErrorCode, &delivery.AttemptedAt, &delivery.NextAttemptAt,
		&delivery.CompletedAt, &delivery.EndpointURL, &payload,
		&delivery.SigningCiphertext, &delivery.EncryptionVersion,
	); err != nil {
		return domain.WebhookDelivery{}, mapError(err)
	}
	if err := json.Unmarshal(payload, &delivery.Payload); err != nil {
		return domain.WebhookDelivery{}, fmt.Errorf("decode webhook payload: %w", err)
	}
	delivery.Status = domain.WebhookDeliveryStatus(status)
	return delivery, nil
}

func (r *Repository) EnqueueWebhookEvent(ctx context.Context, organizationID string, event domain.WebhookEvent, subscriptionIDs []string, meta ports.MutationMeta) ([]domain.WebhookDelivery, error) {
	action := meta.Action + ":" + event.AppID
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin webhook enqueue: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := r.lockIdempotency(ctx, tx, organizationID, action, meta.IdempotencyKey); err != nil {
		return nil, err
	}
	if existingID, ok, err := r.idempotentResource(ctx, tx, organizationID, action, meta.IdempotencyKey); err != nil {
		return nil, err
	} else if ok {
		return listWebhookDeliveriesForEvent(ctx, tx, existingID)
	}
	if _, err := getApp(ctx, tx, organizationID, event.AppID, true); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return nil, fmt.Errorf("encode webhook payload: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO webhook_events (id, app_id, event, source, merchant_id, installation_id, payload, created_at)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6::uuid, $7::jsonb, $8)`,
		event.ID, event.AppID, event.Event, event.Source, event.MerchantID, event.InstallationID, payload, event.CreatedAt); err != nil {
		return nil, mapError(err)
	}
	deliveries := make([]domain.WebhookDelivery, 0, len(subscriptionIDs))
	for _, subscriptionID := range subscriptionIDs {
		subscription, err := getWebhook(ctx, tx, organizationID, event.AppID, subscriptionID, true)
		if err != nil {
			return nil, err
		}
		if subscription.Status != domain.WebhookStatusActive || subscription.Event != event.Event {
			continue
		}
		deliveryID, err := r.id()
		if err != nil {
			return nil, fmt.Errorf("generate webhook delivery id: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO webhook_deliveries (
				id, subscription_id, event_id, attempt, status, attempted_at, next_attempt_at
			) VALUES ($1::uuid, $2::uuid, $3::uuid, 1, 'pending', $4, $4)`,
			deliveryID, subscription.ID, event.ID, event.CreatedAt); err != nil {
			return nil, mapError(err)
		}
		deliveries = append(deliveries, domain.WebhookDelivery{
			ID: deliveryID, SubscriptionID: subscription.ID, EventID: event.ID, Event: event.Event,
			EventSource: event.Source, MerchantID: event.MerchantID, InstallationID: event.InstallationID,
			Attempt: 1, Status: domain.WebhookDeliveryStatusPending, AttemptedAt: event.CreatedAt,
			NextAttemptAt: &event.CreatedAt,
		})
	}
	if len(deliveries) == 0 {
		return nil, fmt.Errorf("%w: no released webhook subscription is currently active", domain.ErrConflict)
	}
	if err := r.storeIdempotency(ctx, tx, organizationID, action, meta.IdempotencyKey, "webhook_event", event.ID); err != nil {
		return nil, err
	}
	if err := r.appendAudit(ctx, tx, organizationID, meta, "webhook_event", event.ID, map[string]any{"appId": event.AppID, "event": event.Event, "deliveries": len(deliveries)}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, mapError(err)
	}
	return deliveries, nil
}

func listWebhookDeliveriesForEvent(ctx context.Context, source interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, eventID string) ([]domain.WebhookDelivery, error) {
	rows, err := source.Query(ctx, `
		SELECT `+webhookDeliveryColumns+`
		FROM webhook_deliveries d
		JOIN webhook_subscriptions w ON w.id = d.subscription_id
		JOIN webhook_events e ON e.id = d.event_id
		WHERE e.id = $1::uuid ORDER BY d.attempted_at, d.id`, eventID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	items := make([]domain.WebhookDelivery, 0)
	for rows.Next() {
		item, err := scanWebhookDelivery(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, mapError(rows.Err())
}

func (r *Repository) ListWebhookDeliveries(ctx context.Context, organizationID, appID, webhookID string, filter ports.AppFilter) ([]domain.WebhookDelivery, ports.PageMeta, error) {
	if _, err := getWebhook(ctx, r.pool, organizationID, appID, webhookID, false); err != nil {
		return nil, ports.PageMeta{}, err
	}
	offset, limit, err := pagination(filter)
	if err != nil {
		return nil, ports.PageMeta{}, err
	}
	var total int
	if err := r.pool.QueryRow(ctx, `
		SELECT count(*) FROM webhook_deliveries d
		JOIN webhook_subscriptions w ON w.id = d.subscription_id
		JOIN apps a ON a.id = w.app_id
		WHERE a.organization_id = $1::uuid AND a.id = $2::uuid AND w.id = $3::uuid`, organizationID, appID, webhookID).Scan(&total); err != nil {
		return nil, ports.PageMeta{}, mapError(err)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT `+webhookDeliveryColumns+`
		FROM webhook_deliveries d
		JOIN webhook_subscriptions w ON w.id = d.subscription_id
		JOIN webhook_events e ON e.id = d.event_id
		JOIN apps a ON a.id = w.app_id
		WHERE a.organization_id = $1::uuid AND a.id = $2::uuid AND w.id = $3::uuid
		ORDER BY d.attempted_at DESC, d.id DESC OFFSET $4 LIMIT $5`, organizationID, appID, webhookID, offset, limit)
	if err != nil {
		return nil, ports.PageMeta{}, mapError(err)
	}
	defer rows.Close()
	items := make([]domain.WebhookDelivery, 0)
	for rows.Next() {
		item, err := scanWebhookDelivery(rows)
		if err != nil {
			return nil, ports.PageMeta{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, ports.PageMeta{}, mapError(err)
	}
	return items, pageMeta(offset, limit, total), nil
}

func (r *Repository) ClaimWebhookDeliveries(ctx context.Context, limit int, now, leaseExpiredBefore time.Time) ([]domain.WebhookDelivery, error) {
	if limit < 1 {
		limit = 25
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin claim deliveries: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
		SELECT `+webhookDeliveryColumns+`
		FROM webhook_deliveries d
		JOIN webhook_subscriptions w ON w.id = d.subscription_id
		JOIN webhook_events e ON e.id = d.event_id
		WHERE d.status = 'pending' AND d.next_attempt_at <= $1
			AND (d.claimed_at IS NULL OR d.claimed_at < $2)
			AND w.status IN ('active', 'failing')
		ORDER BY d.next_attempt_at, d.id
		FOR UPDATE OF d SKIP LOCKED LIMIT $3`, now, leaseExpiredBefore, limit)
	if err != nil {
		return nil, mapError(err)
	}
	items := make([]domain.WebhookDelivery, 0)
	for rows.Next() {
		item, err := scanWebhookDelivery(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	for _, item := range items {
		if _, err := tx.Exec(ctx, `UPDATE webhook_deliveries SET claimed_at = $2 WHERE id = $1::uuid`, item.ID, now); err != nil {
			return nil, mapError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, mapError(err)
	}
	return items, nil
}

func (r *Repository) CompleteWebhookDelivery(ctx context.Context, delivery domain.WebhookDelivery, retry *domain.WebhookDelivery) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin complete delivery: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Exec(ctx, `
		UPDATE webhook_deliveries
		SET status = $2, response_status = $3, response_time_ms = $4, error_code = $5,
			completed_at = $6, next_attempt_at = NULL
		WHERE id = $1::uuid AND status = 'pending'`, delivery.ID, delivery.Status,
		delivery.ResponseStatus, delivery.ResponseTimeMS, delivery.ErrorCode, delivery.CompletedAt)
	if err != nil {
		return mapError(err)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("%w: webhook delivery is no longer pending", domain.ErrConflict)
	}
	if retry != nil {
		if _, err := tx.Exec(ctx, `
			INSERT INTO webhook_deliveries (
				id, subscription_id, event_id, attempt, status, attempted_at, next_attempt_at
			) VALUES ($1::uuid, $2::uuid, $3::uuid, $4, 'pending', $5, $5)`,
			retry.ID, retry.SubscriptionID, retry.EventID, retry.Attempt, retry.AttemptedAt); err != nil {
			return mapError(err)
		}
	}
	status := domain.WebhookStatusFailing
	if delivery.Status == domain.WebhookDeliveryStatusDelivered {
		status = domain.WebhookStatusActive
	}
	if _, err := tx.Exec(ctx, `
		UPDATE webhook_subscriptions SET status = $2, updated_at = $3, revision = revision + 1
		WHERE id = $1::uuid AND status IN ('active', 'failing')`, delivery.SubscriptionID, status, r.now().UTC()); err != nil {
		return mapError(err)
	}
	return tx.Commit(ctx)
}
