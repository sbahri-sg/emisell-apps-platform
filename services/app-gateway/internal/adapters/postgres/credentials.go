package postgres

import (
	"context"
	"fmt"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
	"github.com/jackc/pgx/v5"
)

const credentialColumns = `
	id::text, app_id::text, environment, client_id, secret_fingerprint, status,
	last_used_at, expires_at, created_by::text, created_at, rotated_at, revoked_at,
	secret_ciphertext, encryption_key_version`

const credentialColumnsAliased = `
	c.id::text, c.app_id::text, c.environment, c.client_id, c.secret_fingerprint, c.status,
	c.last_used_at, c.expires_at, c.created_by::text, c.created_at, c.rotated_at, c.revoked_at,
	c.secret_ciphertext, c.encryption_key_version`

func scanCredential(row rowScanner) (domain.AppCredential, error) {
	var credential domain.AppCredential
	var environment, status string
	if err := row.Scan(
		&credential.ID, &credential.AppID, &environment, &credential.ClientID,
		&credential.SecretFingerprint, &status, &credential.LastUsedAt, &credential.ExpiresAt,
		&credential.CreatedBy, &credential.CreatedAt, &credential.RotatedAt, &credential.RevokedAt,
		&credential.SecretCiphertext, &credential.EncryptionKeyVersion,
	); err != nil {
		return domain.AppCredential{}, mapError(err)
	}
	credential.Environment = domain.Environment(environment)
	credential.Status = domain.CredentialStatus(status)
	return credential, nil
}

func getCredential(ctx context.Context, source queryRower, organizationID, appID, credentialID string, forUpdate bool) (domain.AppCredential, error) {
	query := `SELECT ` + credentialColumnsAliased + `
		FROM app_credentials c
		JOIN apps a ON a.id = c.app_id
		WHERE a.organization_id = $1::uuid AND a.id = $2::uuid AND c.id = $3::uuid`
	if forUpdate {
		query += ` FOR UPDATE OF c`
	}
	return scanCredential(source.QueryRow(ctx, query, organizationID, appID, credentialID))
}

func (r *Repository) ListCredentials(ctx context.Context, organizationID, appID string) ([]domain.AppCredential, error) {
	if _, err := getApp(ctx, r.pool, organizationID, appID, false); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `SELECT `+credentialColumnsAliased+`
		FROM app_credentials c
		JOIN apps a ON a.id = c.app_id
		WHERE a.organization_id = $1::uuid AND a.id = $2::uuid
		ORDER BY c.created_at DESC, c.id DESC`, organizationID, appID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	items := make([]domain.AppCredential, 0)
	for rows.Next() {
		credential, err := scanCredential(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, credential)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return items, nil
}

func (r *Repository) CreateCredential(ctx context.Context, organizationID string, credential domain.AppCredential, meta ports.MutationMeta) (domain.AppCredential, error) {
	action := meta.Action + ":" + credential.AppID
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.AppCredential{}, fmt.Errorf("begin create credential: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := r.lockIdempotency(ctx, tx, organizationID, action, meta.IdempotencyKey); err != nil {
		return domain.AppCredential{}, err
	}
	if existingID, ok, err := r.idempotentResource(ctx, tx, organizationID, action, meta.IdempotencyKey); err != nil {
		return domain.AppCredential{}, err
	} else if ok {
		return getCredential(ctx, tx, organizationID, credential.AppID, existingID, false)
	}
	if _, err := getApp(ctx, tx, organizationID, credential.AppID, true); err != nil {
		return domain.AppCredential{}, err
	}
	created, err := scanCredential(tx.QueryRow(ctx, `
		INSERT INTO app_credentials (
			id, app_id, environment, client_id, secret_ciphertext, secret_fingerprint,
			encryption_key_version, status, expires_at, created_by, created_at
		) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10::uuid, $11)
		RETURNING `+credentialColumns,
		credential.ID, credential.AppID, credential.Environment, credential.ClientID,
		credential.SecretCiphertext, credential.SecretFingerprint, credential.EncryptionKeyVersion,
		credential.Status, credential.ExpiresAt, credential.CreatedBy, credential.CreatedAt))
	if err != nil {
		return domain.AppCredential{}, mapError(err)
	}
	if err := r.storeIdempotency(ctx, tx, organizationID, action, meta.IdempotencyKey, "app_credential", credential.ID); err != nil {
		return domain.AppCredential{}, err
	}
	if err := r.appendAudit(ctx, tx, organizationID, meta, "app_credential", credential.ID, map[string]any{"environment": credential.Environment}); err != nil {
		return domain.AppCredential{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AppCredential{}, mapError(err)
	}
	return created, nil
}

func (r *Repository) RotateCredential(ctx context.Context, organizationID, appID, credentialID string, ciphertext []byte, fingerprint string, keyVersion int, meta ports.MutationMeta) (domain.AppCredential, error) {
	action := meta.Action + ":" + credentialID
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.AppCredential{}, fmt.Errorf("begin rotate credential: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := r.lockIdempotency(ctx, tx, organizationID, action, meta.IdempotencyKey); err != nil {
		return domain.AppCredential{}, err
	}
	if existingID, ok, err := r.idempotentResource(ctx, tx, organizationID, action, meta.IdempotencyKey); err != nil {
		return domain.AppCredential{}, err
	} else if ok {
		return getCredential(ctx, tx, organizationID, appID, existingID, false)
	}
	current, err := getCredential(ctx, tx, organizationID, appID, credentialID, true)
	if err != nil {
		return domain.AppCredential{}, err
	}
	if current.Status != domain.CredentialStatusActive || (current.ExpiresAt != nil && !current.ExpiresAt.After(r.now().UTC())) {
		return domain.AppCredential{}, fmt.Errorf("%w: only active, unexpired credentials can be rotated", domain.ErrConflict)
	}
	rotated, err := scanCredential(tx.QueryRow(ctx, `
		UPDATE app_credentials
		SET secret_ciphertext = $4, secret_fingerprint = $5, encryption_key_version = $6, rotated_at = $7
		WHERE app_id = $1::uuid AND id = $2::uuid AND status = $3
		RETURNING `+credentialColumns,
		appID, credentialID, domain.CredentialStatusActive, ciphertext, fingerprint, keyVersion, r.now().UTC()))
	if err != nil {
		return domain.AppCredential{}, mapError(err)
	}
	if err := r.storeIdempotency(ctx, tx, organizationID, action, meta.IdempotencyKey, "app_credential", credentialID); err != nil {
		return domain.AppCredential{}, err
	}
	if err := r.appendAudit(ctx, tx, organizationID, meta, "app_credential", credentialID, map[string]any{"fingerprint": fingerprint}); err != nil {
		return domain.AppCredential{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AppCredential{}, mapError(err)
	}
	return rotated, nil
}

func (r *Repository) RevokeCredential(ctx context.Context, organizationID, appID, credentialID string, meta ports.MutationMeta) error {
	action := meta.Action + ":" + credentialID
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin revoke credential: %w", err)
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
	credential, err := getCredential(ctx, tx, organizationID, appID, credentialID, true)
	if err != nil {
		return err
	}
	if credential.Status != domain.CredentialStatusRevoked {
		if _, err := tx.Exec(ctx, `UPDATE app_credentials SET status = 'revoked', revoked_at = $3 WHERE app_id = $1::uuid AND id = $2::uuid`, appID, credentialID, r.now().UTC()); err != nil {
			return mapError(err)
		}
		if _, err := tx.Exec(ctx, `UPDATE oauth_access_tokens SET revoked_at = $2 WHERE credential_id = $1::uuid AND revoked_at IS NULL`, credentialID, r.now().UTC()); err != nil {
			return mapError(err)
		}
		if err := r.appendAudit(ctx, tx, organizationID, meta, "app_credential", credentialID, nil); err != nil {
			return err
		}
	}
	if err := r.storeIdempotency(ctx, tx, organizationID, action, meta.IdempotencyKey, "app_credential", credentialID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
