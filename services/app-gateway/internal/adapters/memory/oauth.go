package memory

import (
	"context"
	"fmt"
	"sort"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

func (r *Repository) GetOAuthClient(_ context.Context, clientID string) (domain.OAuthClient, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for appID, credentials := range r.credentials {
		for _, credential := range credentials {
			if credential.ClientID == clientID {
				app, ok := r.apps[appID]
				if !ok {
					break
				}
				return domain.OAuthClient{OrganizationID: app.OrganizationID, Credential: cloneCredential(credential)}, nil
			}
		}
	}
	return domain.OAuthClient{}, domain.ErrNotFound
}

func (r *Repository) CreateOAuthAuthorization(_ context.Context, authorization domain.OAuthAuthorization, meta ports.MutationMeta) (domain.OAuthAuthorization, error) {
	auditID, err := r.id()
	if err != nil {
		return domain.OAuthAuthorization{}, fmt.Errorf("generate audit id: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	app, ok := r.apps[authorization.AppID]
	if !ok || app.OrganizationID != authorization.OrganizationID {
		return domain.OAuthAuthorization{}, domain.ErrNotFound
	}
	for _, existing := range r.oauthAuthorizations {
		if existing.CodeHash == authorization.CodeHash {
			return domain.OAuthAuthorization{}, fmt.Errorf("%w: authorization code collision", domain.ErrConflict)
		}
	}
	if authorization.TestInstallRequestID != nil {
		request, exists := r.developmentInstallRequests[authorization.AppID][*authorization.TestInstallRequestID]
		if !exists || request.Status != domain.DevelopmentInstallRequestStatusPending ||
			!request.ExpiresAt.After(authorization.CreatedAt) || request.MerchantID != authorization.MerchantID ||
			request.Environment != authorization.Environment || request.VersionID != authorization.InstalledVersionID {
			return domain.OAuthAuthorization{}, fmt.Errorf("%w: development install request is unavailable", domain.ErrNotFound)
		}
		authorizedAt := authorization.CreatedAt
		authorizationID := authorization.ID
		request.Status = domain.DevelopmentInstallRequestStatusAuthorized
		request.AuthorizedAt = &authorizedAt
		request.AuthorizationID = &authorizationID
		request.UpdatedAt = authorization.CreatedAt
		r.developmentInstallRequests[authorization.AppID][request.ID] = cloneDevelopmentInstallRequest(request)
	}
	r.oauthAuthorizations[authorization.ID] = cloneOAuthAuthorization(authorization)
	r.appendAuditLocked(auditID, authorization.OrganizationID, meta, "oauth_authorization", authorization.ID, map[string]any{"appId": authorization.AppID, "merchantId": authorization.MerchantID})
	return cloneOAuthAuthorization(authorization), nil
}

func (r *Repository) GetOAuthAuthorizationByCodeHash(_ context.Context, codeHash string) (domain.OAuthAuthorization, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, authorization := range r.oauthAuthorizations {
		if authorization.CodeHash == codeHash {
			return cloneOAuthAuthorization(authorization), nil
		}
	}
	return domain.OAuthAuthorization{}, domain.ErrNotFound
}

func (r *Repository) GetInstallationAccessContextByTokenHash(_ context.Context, tokenHash string, now time.Time) (domain.InstallationAccessContext, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var token domain.OAuthAccessToken
	found := false
	for _, candidate := range r.oauthTokens {
		if candidate.TokenHash == tokenHash {
			token = candidate
			found = true
			break
		}
	}
	if !found || token.RevokedAt != nil || !token.ExpiresAt.After(now) {
		return domain.InstallationAccessContext{}, domain.ErrNotFound
	}
	installation, ok := r.installations[token.AppID][token.InstallationID]
	if !ok || installation.AppID != token.AppID {
		return domain.InstallationAccessContext{}, domain.ErrNotFound
	}
	app, ok := r.apps[token.AppID]
	if !ok || app.Status != domain.AppStatusActive {
		return domain.InstallationAccessContext{}, domain.ErrNotFound
	}
	if organization, ok := r.developerOrganizations[app.OrganizationID]; ok && organization.Status != domain.OrganizationStatusActive {
		return domain.InstallationAccessContext{}, domain.ErrNotFound
	}
	credential, ok := r.credentials[token.AppID][token.CredentialID]
	if !ok || credential.Status != domain.CredentialStatusActive || (credential.ExpiresAt != nil && !credential.ExpiresAt.After(now)) {
		return domain.InstallationAccessContext{}, domain.ErrNotFound
	}
	granted := make(map[string]struct{}, len(installation.GrantedScopes))
	for _, scope := range installation.GrantedScopes {
		granted[scope] = struct{}{}
	}
	scopes := make([]string, 0, len(token.Scopes))
	for _, scope := range token.Scopes {
		if _, ok := granted[scope]; ok {
			scopes = append(scopes, scope)
		}
	}
	sort.Strings(scopes)
	context := domain.InstallationAccessContext{
		OrganizationID: app.OrganizationID, AppID: app.ID, AppName: app.Name, AppSlug: app.Slug,
		InstallationID: installation.ID, MerchantID: installation.MerchantID, MerchantName: installation.MerchantName,
		MerchantDomain: installation.MerchantDomain, Environment: installation.Environment,
		InstallationStatus: installation.Status, InstalledVersionID: installation.InstalledVersionID,
		Scopes: scopes, TokenExpiresAt: token.ExpiresAt,
	}
	if context.MerchantDomain != nil {
		value := *context.MerchantDomain
		context.MerchantDomain = &value
	}
	return context, nil
}

func (r *Repository) ConsumeOAuthAuthorization(_ context.Context, authorizationID string, token domain.OAuthAccessToken, installation domain.AppInstallation, consumedAt time.Time) (domain.AppInstallation, error) {
	auditID, err := r.id()
	if err != nil {
		return domain.AppInstallation{}, fmt.Errorf("generate audit id: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	authorization, ok := r.oauthAuthorizations[authorizationID]
	if !ok || authorization.ConsumedAt != nil || !authorization.ExpiresAt.After(consumedAt) {
		return domain.AppInstallation{}, fmt.Errorf("%w: authorization code is no longer usable", domain.ErrInvalidGrant)
	}
	app, ok := r.apps[authorization.AppID]
	if !ok || app.ActiveVersionID == nil || *app.ActiveVersionID != authorization.InstalledVersionID {
		return domain.AppInstallation{}, fmt.Errorf("%w: active app version changed", domain.ErrInvalidGrant)
	}
	credential, ok := r.credentials[authorization.AppID][authorization.CredentialID]
	if !ok || credential.Status != domain.CredentialStatusActive || (credential.ExpiresAt != nil && !credential.ExpiresAt.After(consumedAt)) {
		return domain.AppInstallation{}, fmt.Errorf("%w: OAuth client is no longer active", domain.ErrInvalidGrant)
	}
	if r.installations[installation.AppID] == nil {
		r.installations[installation.AppID] = make(map[string]domain.AppInstallation)
	}
	for _, existing := range r.installations[installation.AppID] {
		if existing.MerchantID == installation.MerchantID && existing.Environment == installation.Environment && existing.Status != domain.InstallationStatusUninstalled {
			return domain.AppInstallation{}, fmt.Errorf("%w: app is already installed for this merchant and environment", domain.ErrConflict)
		}
	}
	consumed := consumedAt
	authorization.ConsumedAt = &consumed
	r.oauthAuthorizations[authorizationID] = cloneOAuthAuthorization(authorization)
	r.installations[installation.AppID][installation.ID] = cloneInstallation(installation)
	token.Scopes = append([]string{}, token.Scopes...)
	r.oauthTokens[token.ID] = token
	credential.LastUsedAt = &consumed
	r.credentials[authorization.AppID][authorization.CredentialID] = credential
	r.appendAuditLocked(auditID, authorization.OrganizationID, ports.MutationMeta{ActorID: authorization.ApprovedBy, Action: "oauth.authorization.exchanged"}, "app_installation", installation.ID, map[string]any{"authorizationId": authorization.ID, "merchantId": installation.MerchantID})
	return cloneInstallation(installation), nil
}

func cloneOAuthAuthorization(authorization domain.OAuthAuthorization) domain.OAuthAuthorization {
	authorization.GrantedScopes = append([]string{}, authorization.GrantedScopes...)
	if authorization.TestInstallRequestID != nil {
		value := *authorization.TestInstallRequestID
		authorization.TestInstallRequestID = &value
	}
	return authorization
}

func (r *Repository) revokeOAuthTokensLocked(installationID, credentialID string, revokedAt time.Time) {
	for id, token := range r.oauthTokens {
		if token.RevokedAt != nil {
			continue
		}
		if (installationID != "" && token.InstallationID == installationID) || (credentialID != "" && token.CredentialID == credentialID) {
			value := revokedAt
			token.RevokedAt = &value
			r.oauthTokens[id] = token
		}
	}
}
