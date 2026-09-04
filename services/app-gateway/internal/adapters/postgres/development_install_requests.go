package postgres

import (
	"context"
	"fmt"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
	"github.com/jackc/pgx/v5"
)

const developmentInstallRequestColumns = `
	id::text, app_id::text, merchant_id, merchant_name, merchant_domain,
	environment, version_id::text, status, launch_url, requested_by::text,
	created_at, updated_at, expires_at, authorized_at, authorization_id::text`

const developmentInstallRequestColumnsAliased = `
	r.id::text, r.app_id::text, r.merchant_id, r.merchant_name, r.merchant_domain,
	r.environment, r.version_id::text, r.status, r.launch_url, r.requested_by::text,
	r.created_at, r.updated_at, r.expires_at, r.authorized_at, r.authorization_id::text`

func scanDevelopmentInstallRequest(row rowScanner) (domain.DevelopmentInstallRequest, error) {
	var request domain.DevelopmentInstallRequest
	var environment, status string
	if err := row.Scan(
		&request.ID, &request.AppID, &request.MerchantID, &request.MerchantName,
		&request.MerchantDomain, &environment, &request.VersionID, &status,
		&request.LaunchURL, &request.RequestedBy, &request.CreatedAt, &request.UpdatedAt,
		&request.ExpiresAt, &request.AuthorizedAt, &request.AuthorizationID,
	); err != nil {
		return domain.DevelopmentInstallRequest{}, mapError(err)
	}
	request.Environment = domain.Environment(environment)
	request.Status = domain.DevelopmentInstallRequestStatus(status)
	return request, nil
}

func (r *Repository) ListDevelopmentInstallRequests(ctx context.Context, organizationID, appID string) ([]domain.DevelopmentInstallRequest, error) {
	if _, err := getApp(ctx, r.pool, organizationID, appID, false); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `SELECT `+developmentInstallRequestColumnsAliased+`
		FROM development_install_requests r
		JOIN apps a ON a.id = r.app_id
		WHERE a.organization_id = $1::uuid AND r.app_id = $2::uuid
		ORDER BY r.created_at DESC, r.id DESC`, organizationID, appID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	items := make([]domain.DevelopmentInstallRequest, 0)
	for rows.Next() {
		request, err := scanDevelopmentInstallRequest(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, request)
	}
	return items, mapError(rows.Err())
}

func (r *Repository) CreateDevelopmentInstallRequest(ctx context.Context, organizationID string, request domain.DevelopmentInstallRequest, meta ports.MutationMeta) (domain.DevelopmentInstallRequest, error) {
	action := meta.Action + ":" + request.AppID
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.DevelopmentInstallRequest{}, fmt.Errorf("begin development install request: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := r.lockIdempotency(ctx, tx, organizationID, action, meta.IdempotencyKey); err != nil {
		return domain.DevelopmentInstallRequest{}, err
	}
	if existingID, ok, err := r.idempotentResource(ctx, tx, organizationID, action, meta.IdempotencyKey); err != nil {
		return domain.DevelopmentInstallRequest{}, err
	} else if ok {
		return scanDevelopmentInstallRequest(tx.QueryRow(ctx, `SELECT `+developmentInstallRequestColumnsAliased+`
			FROM development_install_requests r JOIN apps a ON a.id = r.app_id
			WHERE a.organization_id = $1::uuid AND r.app_id = $2::uuid AND r.id = $3::uuid`, organizationID, request.AppID, existingID))
	}
	app, err := getApp(ctx, tx, organizationID, request.AppID, true)
	if err != nil {
		return domain.DevelopmentInstallRequest{}, err
	}
	if app.ActiveVersionID == nil || *app.ActiveVersionID != request.VersionID {
		return domain.DevelopmentInstallRequest{}, fmt.Errorf("%w: active version changed", domain.ErrConflict)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE development_install_requests
		SET status = 'expired', updated_at = $3
		WHERE app_id = $1::uuid AND merchant_id = $2
		  AND status = 'pending' AND expires_at <= $3`, request.AppID, request.MerchantID, request.CreatedAt); err != nil {
		return domain.DevelopmentInstallRequest{}, mapError(err)
	}
	created, err := scanDevelopmentInstallRequest(tx.QueryRow(ctx, `
		INSERT INTO development_install_requests (
			id, app_id, merchant_id, merchant_name, merchant_domain, environment,
			version_id, status, launch_url, requested_by, created_at, updated_at, expires_at
		) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7::uuid, $8, $9, $10::uuid, $11, $12, $13)
		RETURNING `+developmentInstallRequestColumns,
		request.ID, request.AppID, request.MerchantID, request.MerchantName,
		request.MerchantDomain, request.Environment, request.VersionID, request.Status,
		request.LaunchURL, request.RequestedBy, request.CreatedAt, request.UpdatedAt, request.ExpiresAt))
	if err != nil {
		return domain.DevelopmentInstallRequest{}, mapError(err)
	}
	if err := r.storeIdempotency(ctx, tx, organizationID, action, meta.IdempotencyKey, "development_install_request", request.ID); err != nil {
		return domain.DevelopmentInstallRequest{}, err
	}
	if err := r.appendAudit(ctx, tx, organizationID, meta, "development_install_request", request.ID, map[string]any{
		"appId": request.AppID, "merchantId": request.MerchantID, "versionId": request.VersionID,
	}); err != nil {
		return domain.DevelopmentInstallRequest{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.DevelopmentInstallRequest{}, mapError(err)
	}
	return created, nil
}

func (r *Repository) GetDevelopmentInstallRequest(ctx context.Context, organizationID, appID, requestID string) (domain.DevelopmentInstallRequest, error) {
	return scanDevelopmentInstallRequest(r.pool.QueryRow(ctx, `SELECT `+developmentInstallRequestColumnsAliased+`
		FROM development_install_requests r
		JOIN apps a ON a.id = r.app_id
		WHERE a.organization_id = $1::uuid AND r.app_id = $2::uuid AND r.id = $3::uuid`, organizationID, appID, requestID))
}
