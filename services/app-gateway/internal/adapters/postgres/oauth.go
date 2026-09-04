package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
	"github.com/jackc/pgx/v5"
)

const oauthAuthorizationColumns = `
	id::text, organization_id::text, app_id::text, credential_id::text, client_id,
	code_hash, redirect_uri, code_challenge, merchant_id::text, merchant_name,
	merchant_domain, environment, installed_version_id::text, granted_scopes,
	approved_by::text, created_at, expires_at, consumed_at`

func scanOAuthAuthorization(row rowScanner) (domain.OAuthAuthorization, error) {
	var authorization domain.OAuthAuthorization
	var environment string
	var scopes []byte
	if err := row.Scan(
		&authorization.ID, &authorization.OrganizationID, &authorization.AppID,
		&authorization.CredentialID, &authorization.ClientID, &authorization.CodeHash,
		&authorization.RedirectURI, &authorization.CodeChallenge, &authorization.MerchantID,
		&authorization.MerchantName, &authorization.MerchantDomain, &environment,
		&authorization.InstalledVersionID, &scopes, &authorization.ApprovedBy,
		&authorization.CreatedAt, &authorization.ExpiresAt, &authorization.ConsumedAt,
	); err != nil {
		return domain.OAuthAuthorization{}, mapError(err)
	}
	if err := json.Unmarshal(scopes, &authorization.GrantedScopes); err != nil {
		return domain.OAuthAuthorization{}, fmt.Errorf("decode OAuth scopes: %w", err)
	}
	authorization.Environment = domain.Environment(environment)
	return authorization, nil
}

func (r *Repository) GetOAuthClient(ctx context.Context, clientID string) (domain.OAuthClient, error) {
	var client domain.OAuthClient
	var environment, status string
	err := r.pool.QueryRow(ctx, `
		SELECT a.organization_id::text, `+credentialColumnsAliased+`
		FROM app_credentials c
		JOIN apps a ON a.id = c.app_id
		WHERE c.client_id = $1`, clientID).Scan(
		&client.OrganizationID, &client.Credential.ID, &client.Credential.AppID,
		&environment, &client.Credential.ClientID, &client.Credential.SecretFingerprint,
		&status, &client.Credential.LastUsedAt, &client.Credential.ExpiresAt,
		&client.Credential.CreatedBy, &client.Credential.CreatedAt, &client.Credential.RotatedAt,
		&client.Credential.RevokedAt, &client.Credential.SecretCiphertext,
		&client.Credential.EncryptionKeyVersion,
	)
	if err != nil {
		return domain.OAuthClient{}, mapError(err)
	}
	client.Credential.Environment = domain.Environment(environment)
	client.Credential.Status = domain.CredentialStatus(status)
	return client, nil
}

func (r *Repository) CreateOAuthAuthorization(ctx context.Context, authorization domain.OAuthAuthorization, meta ports.MutationMeta) (domain.OAuthAuthorization, error) {
	scopes, err := json.Marshal(authorization.GrantedScopes)
	if err != nil {
		return domain.OAuthAuthorization{}, fmt.Errorf("encode OAuth scopes: %w", err)
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.OAuthAuthorization{}, fmt.Errorf("begin OAuth authorization: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := getApp(ctx, tx, authorization.OrganizationID, authorization.AppID, true); err != nil {
		return domain.OAuthAuthorization{}, err
	}
	created, err := scanOAuthAuthorization(tx.QueryRow(ctx, `
		INSERT INTO oauth_authorizations (
			id, organization_id, app_id, credential_id, client_id, code_hash, redirect_uri,
			code_challenge, merchant_id, merchant_name, merchant_domain, environment,
			installed_version_id, granted_scopes, approved_by, created_at, expires_at
		) VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6, $7, $8, $9,
			$10, $11, $12, $13::uuid, $14::jsonb, $15::uuid, $16, $17)
		RETURNING `+oauthAuthorizationColumns,
		authorization.ID, authorization.OrganizationID, authorization.AppID,
		authorization.CredentialID, authorization.ClientID, authorization.CodeHash,
		authorization.RedirectURI, authorization.CodeChallenge, authorization.MerchantID,
		authorization.MerchantName, authorization.MerchantDomain, authorization.Environment,
		authorization.InstalledVersionID, scopes, authorization.ApprovedBy,
		authorization.CreatedAt, authorization.ExpiresAt))
	if err != nil {
		return domain.OAuthAuthorization{}, mapError(err)
	}
	if authorization.TestInstallRequestID != nil {
		result, err := tx.Exec(ctx, `
			UPDATE development_install_requests
			SET status = 'authorized', authorization_id = $2::uuid,
				authorized_at = $3, updated_at = $3
			WHERE id = $1::uuid
			  AND app_id = $4::uuid
			  AND merchant_id = $5
			  AND environment = $6
			  AND version_id = $7::uuid
			  AND status = 'pending'
			  AND expires_at > $3`,
			*authorization.TestInstallRequestID, authorization.ID, authorization.CreatedAt,
			authorization.AppID, authorization.MerchantID, authorization.Environment,
			authorization.InstalledVersionID)
		if err != nil {
			return domain.OAuthAuthorization{}, mapError(err)
		}
		if result.RowsAffected() != 1 {
			return domain.OAuthAuthorization{}, fmt.Errorf("%w: development install request is unavailable", domain.ErrNotFound)
		}
	}
	if err := r.appendAudit(ctx, tx, authorization.OrganizationID, meta, "oauth_authorization", authorization.ID, map[string]any{
		"appId": authorization.AppID, "merchantId": authorization.MerchantID,
		"environment": authorization.Environment, "expiresAt": authorization.ExpiresAt,
	}); err != nil {
		return domain.OAuthAuthorization{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.OAuthAuthorization{}, mapError(err)
	}
	return created, nil
}

func (r *Repository) GetOAuthAuthorizationByCodeHash(ctx context.Context, codeHash string) (domain.OAuthAuthorization, error) {
	return scanOAuthAuthorization(r.pool.QueryRow(ctx, `SELECT `+oauthAuthorizationColumns+` FROM oauth_authorizations WHERE code_hash = $1`, codeHash))
}

func (r *Repository) GetInstallationAccessContextByTokenHash(ctx context.Context, tokenHash string, now time.Time) (domain.InstallationAccessContext, error) {
	var access domain.InstallationAccessContext
	var environment, installationStatus string
	var tokenScopesJSON, installationScopesJSON []byte
	err := r.pool.QueryRow(ctx, `
		SELECT a.organization_id::text, a.id::text, a.name, a.slug,
		       i.id::text, i.merchant_id::text, i.merchant_name, i.merchant_domain,
		       i.environment, i.status, i.installed_version_id::text,
		       t.scopes, i.granted_scopes, t.expires_at
		FROM oauth_access_tokens t
		JOIN app_installations i ON i.id = t.installation_id AND i.app_id = t.app_id
		JOIN apps a ON a.id = t.app_id
		JOIN organizations o ON o.id = a.organization_id
		JOIN app_credentials c ON c.id = t.credential_id AND c.app_id = t.app_id
		WHERE t.token_hash = $1
		  AND t.revoked_at IS NULL
		  AND t.expires_at > $2
		  AND c.status = 'active'
		  AND (c.expires_at IS NULL OR c.expires_at > $2)
		  AND a.status = 'active'
		  AND o.status = 'active'`, tokenHash, now).Scan(
		&access.OrganizationID, &access.AppID, &access.AppName, &access.AppSlug,
		&access.InstallationID, &access.MerchantID, &access.MerchantName, &access.MerchantDomain,
		&environment, &installationStatus, &access.InstalledVersionID,
		&tokenScopesJSON, &installationScopesJSON, &access.TokenExpiresAt,
	)
	if err != nil {
		return domain.InstallationAccessContext{}, mapError(err)
	}
	var tokenScopes, installationScopes []string
	if err := json.Unmarshal(tokenScopesJSON, &tokenScopes); err != nil {
		return domain.InstallationAccessContext{}, fmt.Errorf("decode access token scopes: %w", err)
	}
	if err := json.Unmarshal(installationScopesJSON, &installationScopes); err != nil {
		return domain.InstallationAccessContext{}, fmt.Errorf("decode installation scopes: %w", err)
	}
	granted := make(map[string]struct{}, len(installationScopes))
	for _, scope := range installationScopes {
		granted[scope] = struct{}{}
	}
	access.Scopes = make([]string, 0, len(tokenScopes))
	for _, scope := range tokenScopes {
		if _, ok := granted[scope]; ok {
			access.Scopes = append(access.Scopes, scope)
		}
	}
	sort.Strings(access.Scopes)
	access.Environment = domain.Environment(environment)
	access.InstallationStatus = domain.InstallationStatus(installationStatus)
	return access, nil
}

func (r *Repository) ConsumeOAuthAuthorization(ctx context.Context, authorizationID string, token domain.OAuthAccessToken, installation domain.AppInstallation, consumedAt time.Time) (domain.AppInstallation, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.AppInstallation{}, fmt.Errorf("begin OAuth exchange: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	authorization, err := scanOAuthAuthorization(tx.QueryRow(ctx, `SELECT `+oauthAuthorizationColumns+` FROM oauth_authorizations WHERE id = $1::uuid FOR UPDATE`, authorizationID))
	if err != nil {
		return domain.AppInstallation{}, err
	}
	if authorization.ConsumedAt != nil || !authorization.ExpiresAt.After(consumedAt) {
		return domain.AppInstallation{}, fmt.Errorf("%w: authorization code is no longer usable", domain.ErrInvalidGrant)
	}
	var credentialStatus string
	var credentialExpires *time.Time
	var activeVersionID *string
	if err := tx.QueryRow(ctx, `
		SELECT c.status, c.expires_at, a.active_version_id::text
		FROM app_credentials c JOIN apps a ON a.id = c.app_id
		WHERE c.id = $1::uuid AND c.app_id = $2::uuid
		FOR UPDATE OF c, a`, authorization.CredentialID, authorization.AppID).Scan(&credentialStatus, &credentialExpires, &activeVersionID); err != nil {
		return domain.AppInstallation{}, mapError(err)
	}
	if credentialStatus != string(domain.CredentialStatusActive) || (credentialExpires != nil && !credentialExpires.After(consumedAt)) || activeVersionID == nil || *activeVersionID != authorization.InstalledVersionID {
		return domain.AppInstallation{}, fmt.Errorf("%w: client or active app version changed", domain.ErrInvalidGrant)
	}
	grantedScopes, err := json.Marshal(installation.GrantedScopes)
	if err != nil {
		return domain.AppInstallation{}, fmt.Errorf("encode installation scopes: %w", err)
	}
	created, err := scanInstallation(tx.QueryRow(ctx, `
		INSERT INTO app_installations (
			id, app_id, merchant_id, merchant_name, merchant_domain, environment, status,
			installed_version_id, granted_scopes, installed_by, installed_at, created_at, updated_at, revision
		) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8::uuid, $9::jsonb, $10::uuid, $11, $12, $13, $14)
		RETURNING `+installationColumns,
		installation.ID, installation.AppID, installation.MerchantID, installation.MerchantName,
		installation.MerchantDomain, installation.Environment, installation.Status,
		installation.InstalledVersionID, grantedScopes, installation.InstalledBy,
		installation.InstalledAt, installation.CreatedAt, installation.UpdatedAt, installation.Revision))
	if err != nil {
		return domain.AppInstallation{}, mapError(err)
	}
	tokenScopes, err := json.Marshal(token.Scopes)
	if err != nil {
		return domain.AppInstallation{}, fmt.Errorf("encode token scopes: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO oauth_access_tokens (
			id, app_id, installation_id, credential_id, token_hash, scopes, created_at, expires_at
		) VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6::jsonb, $7, $8)`,
		token.ID, token.AppID, token.InstallationID, token.CredentialID,
		token.TokenHash, tokenScopes, token.CreatedAt, token.ExpiresAt); err != nil {
		return domain.AppInstallation{}, mapError(err)
	}
	result, err := tx.Exec(ctx, `UPDATE oauth_authorizations SET consumed_at = $2 WHERE id = $1::uuid AND consumed_at IS NULL`, authorizationID, consumedAt)
	if err != nil {
		return domain.AppInstallation{}, mapError(err)
	}
	if result.RowsAffected() != 1 {
		return domain.AppInstallation{}, fmt.Errorf("%w: authorization code was already consumed", domain.ErrInvalidGrant)
	}
	if _, err := tx.Exec(ctx, `UPDATE app_credentials SET last_used_at = $2 WHERE id = $1::uuid`, authorization.CredentialID, consumedAt); err != nil {
		return domain.AppInstallation{}, mapError(err)
	}
	if err := r.appendAudit(ctx, tx, authorization.OrganizationID, ports.MutationMeta{ActorID: authorization.ApprovedBy, Action: "oauth.authorization.exchanged"}, "app_installation", installation.ID, map[string]any{
		"authorizationId": authorization.ID, "credentialId": authorization.CredentialID,
		"merchantId": installation.MerchantID, "versionId": installation.InstalledVersionID,
	}); err != nil {
		return domain.AppInstallation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AppInstallation{}, mapError(err)
	}
	return created, nil
}
