package ports

import (
	"context"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
)

type MutationMeta struct {
	ActorID        string
	Action         string
	IdempotencyKey string
	// Catalog publication binds the service's validation to the locked app.
	ExpectedAppRevision     *int64
	ExpectedActiveVersionID *string
}

type AppFilter struct {
	Cursor string
	Limit  int
	Search string
	Status domain.AppStatus
}

type PageMeta struct {
	NextCursor *string `json:"nextCursor"`
	HasMore    bool    `json:"hasMore"`
}

type DeveloperApplicationFilter struct {
	Cursor string
	Limit  int
	Search string
	Status domain.DeveloperApplicationStatus
}

type DeveloperOrganizationFilter struct {
	Cursor string
	Limit  int
	Search string
	Status domain.OrganizationStatus
}

type CatalogFilter struct {
	Cursor   string
	Limit    int
	Search   string
	Category domain.CatalogCategory
	Featured *bool
	Status   domain.CatalogListingStatus
}

type DeveloperOrganizationRepository interface {
	ListDeveloperOrganizations(ctx context.Context, filter DeveloperOrganizationFilter) ([]domain.DeveloperOrganization, PageMeta, error)
	GetDeveloperOrganization(ctx context.Context, organizationID string) (domain.DeveloperOrganizationDetail, error)
}

type DeveloperProgramRepository interface {
	ListDeveloperApplications(ctx context.Context, platformOrgID string, filter DeveloperApplicationFilter) ([]domain.DeveloperApplication, PageMeta, error)
	CreateDeveloperApplication(ctx context.Context, application domain.DeveloperApplication, meta MutationMeta) (domain.DeveloperApplication, error)
	GetDeveloperApplication(ctx context.Context, platformOrgID, applicationID string) (domain.DeveloperApplication, error)
	StartDeveloperApplicationReview(ctx context.Context, platformOrgID, applicationID, actorID string, expectedRevision int64, notes *string) (domain.DeveloperApplication, error)
	ApproveDeveloperApplication(ctx context.Context, platformOrgID string, application domain.DeveloperApplication, organizationID, organizationName, organizationSlug string, entitlement domain.OrganizationEntitlement, invitation domain.DeveloperInvitation, expectedRevision int64, meta MutationMeta) (domain.DeveloperApplication, domain.DeveloperInvitation, error)
	RejectDeveloperApplication(ctx context.Context, platformOrgID, applicationID, actorID string, expectedRevision int64, notes string) (domain.DeveloperApplication, error)
	RotateDeveloperInvitation(ctx context.Context, platformOrgID, applicationID string, invitation domain.DeveloperInvitation, expectedRevision int64, meta MutationMeta) (domain.DeveloperApplication, domain.DeveloperInvitation, error)
	RevokeDeveloperInvitation(ctx context.Context, platformOrgID, invitationID, actorID string) (domain.DeveloperInvitation, error)
	AcceptDeveloperInvitation(ctx context.Context, tokenHash, actorID, actorEmail, displayName string, acceptedAt time.Time) (domain.DeveloperApplication, domain.OrganizationEntitlement, error)
}

type IdentityRepository interface {
	ListOrganizationMemberships(ctx context.Context, userID string) ([]domain.OrganizationMembership, error)
	CreateIdentitySession(ctx context.Context, session domain.IdentitySession) error
	GetIdentitySessionByTokenHash(ctx context.Context, tokenHash string, now time.Time, idleTTL time.Duration) (domain.IdentitySession, *domain.OrganizationMembership, error)
	SwitchIdentitySessionOrganization(ctx context.Context, sessionID, userID, organizationID string, now time.Time) (domain.IdentitySession, domain.OrganizationMembership, error)
	RevokeIdentitySession(ctx context.Context, tokenHash string, now time.Time) error
	CreateOIDCLoginState(ctx context.Context, state domain.OIDCLoginState) error
	ConsumeOIDCLoginState(ctx context.Context, stateHash string, now time.Time) (domain.OIDCLoginState, error)
	ProvisionOIDCUser(ctx context.Context, provider, subject, email, displayName string, userID string, now time.Time) (string, error)
}

type MerchantRepository interface {
	UpsertMerchantIdentity(ctx context.Context, identity domain.MerchantIdentity) (domain.MerchantIdentity, error)
	GetMerchantIdentity(ctx context.Context, userID, merchantID string, environment domain.Environment) (domain.MerchantIdentity, error)
	ResolveMerchantIdentity(ctx context.Context, merchantID string) (domain.MerchantIdentity, error)
	ListMerchantInstalledApps(ctx context.Context, merchantID string, environment domain.Environment) ([]domain.MerchantInstalledApp, error)
	GetMerchantInstalledApp(ctx context.Context, merchantID, installationID string, environment domain.Environment) (domain.MerchantInstalledApp, error)
}

type CatalogRepository interface {
	ListCatalogApps(ctx context.Context, filter CatalogFilter) ([]domain.CatalogApp, PageMeta, error)
	GetCatalogApp(ctx context.Context, appID string) (domain.CatalogApp, error)
	ListCatalogCandidates(ctx context.Context, filter CatalogFilter) ([]domain.CatalogCandidate, PageMeta, error)
	GetCatalogListing(ctx context.Context, organizationID, appID string) (domain.AppCatalogListing, error)
	UpsertCatalogListing(ctx context.Context, listing domain.AppCatalogListing, expectedRevision int64, meta MutationMeta) (domain.AppCatalogListing, error)
}

type MerchantSessionGrantRepository interface {
	CreateMerchantSessionGrant(ctx context.Context, grant domain.MerchantSessionGrant) error
	ConsumeMerchantSessionGrant(ctx context.Context, codeHash string, now time.Time) (domain.MerchantSessionGrant, error)
}

type Repository interface {
	GetOrganizationEntitlement(ctx context.Context, organizationID string) (domain.OrganizationEntitlement, error)

	ListApps(ctx context.Context, organizationID string, filter AppFilter) ([]domain.App, PageMeta, error)
	CreateApp(ctx context.Context, app domain.App, meta MutationMeta) (domain.App, error)
	GetApp(ctx context.Context, organizationID, appID string) (domain.App, error)
	UpdateApp(ctx context.Context, app domain.App, expectedRevision int64, meta MutationMeta) (domain.App, error)
	ArchiveApp(ctx context.Context, organizationID, appID string, meta MutationMeta) error

	ListVersions(ctx context.Context, organizationID, appID string, filter AppFilter) ([]domain.AppVersion, PageMeta, error)
	CreateVersion(ctx context.Context, organizationID string, version domain.AppVersion, meta MutationMeta) (domain.AppVersion, error)
	GetVersion(ctx context.Context, organizationID, appID, versionID string) (domain.AppVersion, error)
	ActivateVersion(ctx context.Context, organizationID, appID, versionID string, expectedStatus domain.VersionStatus, expectedActiveVersionID *string, meta MutationMeta) (domain.AppVersion, error)

	ListExtensions(ctx context.Context, organizationID, appID string) ([]domain.AppExtension, error)
	CreateExtension(ctx context.Context, organizationID string, extension domain.AppExtension, meta MutationMeta) (domain.AppExtension, error)
	GetExtension(ctx context.Context, organizationID, appID, extensionID string) (domain.AppExtension, error)
	UpdateExtension(ctx context.Context, organizationID string, extension domain.AppExtension, expectedRevision int64, meta MutationMeta) (domain.AppExtension, error)
	DisableExtension(ctx context.Context, organizationID, appID, extensionID string, meta MutationMeta) error

	ListScopes(ctx context.Context, organizationID, appID string) ([]domain.AppScope, error)
	ReplaceScopes(ctx context.Context, organizationID, appID string, scopes []domain.AppScope, meta MutationMeta) ([]domain.AppScope, error)

	ListCredentials(ctx context.Context, organizationID, appID string) ([]domain.AppCredential, error)
	CreateCredential(ctx context.Context, organizationID string, credential domain.AppCredential, meta MutationMeta) (domain.AppCredential, error)
	RotateCredential(ctx context.Context, organizationID, appID, credentialID string, ciphertext []byte, fingerprint string, keyVersion int, meta MutationMeta) (domain.AppCredential, error)
	RevokeCredential(ctx context.Context, organizationID, appID, credentialID string, meta MutationMeta) error

	ListWebhooks(ctx context.Context, organizationID, appID string, filter AppFilter) ([]domain.WebhookSubscription, PageMeta, error)
	ListWebhookEventDefinitions(ctx context.Context) ([]domain.WebhookEventDefinition, error)
	GetWebhookEventDefinition(ctx context.Context, event string) (domain.WebhookEventDefinition, error)
	CreateWebhook(ctx context.Context, organizationID string, subscription domain.WebhookSubscription, meta MutationMeta) (domain.WebhookSubscription, error)
	GetWebhook(ctx context.Context, organizationID, appID, webhookID string) (domain.WebhookSubscription, error)
	UpdateWebhook(ctx context.Context, organizationID string, subscription domain.WebhookSubscription, expectedRevision int64, meta MutationMeta) (domain.WebhookSubscription, error)
	DisableWebhook(ctx context.Context, organizationID, appID, webhookID string, meta MutationMeta) error

	ListInstallations(ctx context.Context, organizationID, appID string, filter AppFilter) ([]domain.AppInstallation, PageMeta, error)
	CreateInstallation(ctx context.Context, organizationID string, installation domain.AppInstallation, expectedActiveVersionID string, meta MutationMeta) (domain.AppInstallation, error)
	GetInstallation(ctx context.Context, organizationID, appID, installationID string) (domain.AppInstallation, error)
	UpdateInstallation(ctx context.Context, organizationID string, installation domain.AppInstallation, expectedRevision int64, meta MutationMeta) (domain.AppInstallation, error)
	UpgradeInstallation(ctx context.Context, organizationID, appID, installationID, targetVersionID, expectedInstalledVersionID string, grantedScopes []string, expectedRevision int64, meta MutationMeta) (domain.AppInstallation, error)
	UninstallInstallation(ctx context.Context, organizationID, appID, installationID string, meta MutationMeta) error

	ListDevelopmentInstallRequests(ctx context.Context, organizationID, appID string) ([]domain.DevelopmentInstallRequest, error)
	CreateDevelopmentInstallRequest(ctx context.Context, organizationID string, request domain.DevelopmentInstallRequest, meta MutationMeta) (domain.DevelopmentInstallRequest, error)
	GetDevelopmentInstallRequest(ctx context.Context, organizationID, appID, requestID string) (domain.DevelopmentInstallRequest, error)

	GetOAuthClient(ctx context.Context, clientID string) (domain.OAuthClient, error)
	CreateOAuthAuthorization(ctx context.Context, authorization domain.OAuthAuthorization, meta MutationMeta) (domain.OAuthAuthorization, error)
	GetOAuthAuthorizationByCodeHash(ctx context.Context, codeHash string) (domain.OAuthAuthorization, error)
	ConsumeOAuthAuthorization(ctx context.Context, authorizationID string, token domain.OAuthAccessToken, installation domain.AppInstallation, consumedAt time.Time) (domain.AppInstallation, error)
	GetInstallationAccessContextByTokenHash(ctx context.Context, tokenHash string, now time.Time) (domain.InstallationAccessContext, error)

	EnqueueWebhookEvent(ctx context.Context, organizationID string, event domain.WebhookEvent, subscriptionIDs []string, meta MutationMeta) ([]domain.WebhookDelivery, error)
	ListWebhookDeliveries(ctx context.Context, organizationID, appID, webhookID string, filter AppFilter) ([]domain.WebhookDelivery, PageMeta, error)
	ClaimWebhookDeliveries(ctx context.Context, limit int, now, leaseExpiredBefore time.Time) ([]domain.WebhookDelivery, error)
	CompleteWebhookDelivery(ctx context.Context, delivery domain.WebhookDelivery, retry *domain.WebhookDelivery) error

	ListAuditEvents(ctx context.Context, organizationID string) ([]domain.AuditEvent, error)
}
