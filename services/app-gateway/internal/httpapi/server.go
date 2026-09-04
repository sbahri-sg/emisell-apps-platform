package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"emisell-app-platform/services/app-gateway/internal/adapters/emisell"
	"emisell-app-platform/services/app-gateway/internal/application"
)

type Dependencies struct {
	AdminLogin             *application.AdminLoginService
	AdminAuthenticator     Authenticator
	AppBilling             *application.AppBillingService
	ExtensionConnections   *application.ExtensionConnectionService
	Products               *emisell.Products
	ShippingRates          *application.ShippingRateService
	Apps                   *application.AppService
	Versions               *application.VersionService
	Extensions             *application.ExtensionService
	Scopes                 *application.ScopeService
	Credentials            *application.CredentialService
	Webhooks               *application.WebhookService
	Installations          *application.InstallationService
	DevelopmentInstalls    *application.DevelopmentInstallService
	InstallationAccess     *application.InstallationAccessService
	OAuth                  *application.OAuthService
	DeveloperProgram       *application.DeveloperProgramService
	DeveloperOrganizations *application.DeveloperOrganizationService
	Identity               *application.IdentityService
	Merchants              *application.MerchantService
	Catalog                *application.CatalogService
	EmisellIntegration     *application.EmisellIntegrationService
	EmisellBackendAuth     EmisellBackendAuthenticator
	OIDC                   *application.OIDCService
	IdentityHTTP           IdentityHTTPOptions
	Authenticator          Authenticator
	Logger                 *slog.Logger
	AllowedOrigins         []string
	Readiness              func(context.Context) error
}

func NewServer(dependencies Dependencies) http.Handler {
	adminAuthenticator := dependencies.AdminAuthenticator
	if adminAuthenticator == nil {
		adminAuthenticator = AdminSessionAuthenticator{Service: dependencies.AdminLogin}
	}
	handlers := &handlers{apps: dependencies.Apps, versions: dependencies.Versions, extensions: dependencies.Extensions, scopes: dependencies.Scopes, credentials: dependencies.Credentials, webhooks: dependencies.Webhooks, installations: dependencies.Installations, developmentInstalls: dependencies.DevelopmentInstalls, installationAccess: dependencies.InstallationAccess, oauth: dependencies.OAuth, developerProgram: dependencies.DeveloperProgram, developerOrganizations: dependencies.DeveloperOrganizations, identity: dependencies.Identity, merchants: dependencies.Merchants, catalog: dependencies.Catalog, emisellIntegration: dependencies.EmisellIntegration, emisellBackendAuthenticator: dependencies.EmisellBackendAuth, shippingRates: dependencies.ShippingRates, oidc: dependencies.OIDC, identityHTTP: dependencies.IdentityHTTP}
	handlers.products = dependencies.Products
	handlers.extensionConnections = dependencies.ExtensionConnections
	handlers.appBilling = dependencies.AppBilling
	api := http.NewServeMux()
	api.HandleFunc("GET /v1/session", handlers.getSession)
	api.HandleFunc("GET /v1/session/organizations", handlers.listSessionOrganizations)
	api.HandleFunc("POST /v1/session/organization", handlers.switchSessionOrganization)
	api.HandleFunc("POST /v1/session/logout", handlers.logout)
	api.HandleFunc("GET /v1/apps", handlers.listApps)
	api.HandleFunc("POST /v1/apps", handlers.createApp)
	api.HandleFunc("GET /v1/apps/{appId}", handlers.getApp)
	api.HandleFunc("GET /v1/apps/{appId}/integration-readiness", handlers.getAppIntegrationReadiness)
	api.HandleFunc("GET /v1/apps/{appId}/plans", handlers.listAppPlans)
	api.HandleFunc("POST /v1/apps/{appId}/plans", handlers.createAppPlan)
	api.HandleFunc("DELETE /v1/apps/{appId}/plans/{planId}", handlers.archiveAppPlan)
	api.HandleFunc("PATCH /v1/apps/{appId}", handlers.updateApp)
	api.HandleFunc("DELETE /v1/apps/{appId}", handlers.archiveApp)
	api.HandleFunc("GET /v1/apps/{appId}/versions", handlers.listVersions)
	api.HandleFunc("POST /v1/apps/{appId}/versions", handlers.createVersion)
	api.HandleFunc("GET /v1/apps/{appId}/versions/{versionId}", handlers.getVersion)
	api.HandleFunc("POST /v1/apps/{appId}/versions/{versionId}/release", handlers.releaseVersion)
	api.HandleFunc("POST /v1/apps/{appId}/versions/{versionId}/rollback", handlers.rollbackVersion)
	api.HandleFunc("GET /v1/apps/{appId}/extensions", handlers.listExtensions)
	api.HandleFunc("POST /v1/apps/{appId}/extensions", handlers.createExtension)
	api.HandleFunc("PATCH /v1/apps/{appId}/extensions/{extensionId}", handlers.updateExtension)
	api.HandleFunc("DELETE /v1/apps/{appId}/extensions/{extensionId}", handlers.disableExtension)
	api.HandleFunc("GET /v1/apps/{appId}/scopes", handlers.listScopes)
	api.HandleFunc("PUT /v1/apps/{appId}/scopes", handlers.replaceScopes)
	api.HandleFunc("GET /v1/apps/{appId}/credentials", handlers.listCredentials)
	api.HandleFunc("POST /v1/apps/{appId}/credentials", handlers.createCredential)
	api.HandleFunc("POST /v1/apps/{appId}/credentials/{credentialId}/rotate", handlers.rotateCredential)
	api.HandleFunc("DELETE /v1/apps/{appId}/credentials/{credentialId}", handlers.revokeCredential)
	api.HandleFunc("GET /v1/apps/{appId}/webhooks", handlers.listWebhooks)
	api.HandleFunc("POST /v1/apps/{appId}/webhooks", handlers.createWebhook)
	api.HandleFunc("PATCH /v1/apps/{appId}/webhooks/{webhookId}", handlers.updateWebhook)
	api.HandleFunc("DELETE /v1/apps/{appId}/webhooks/{webhookId}", handlers.disableWebhook)
	api.HandleFunc("GET /v1/apps/{appId}/webhooks/{webhookId}/deliveries", handlers.listWebhookDeliveries)
	api.HandleFunc("POST /v1/apps/{appId}/webhook-events", handlers.publishWebhookEvent)
	api.HandleFunc("GET /v1/apps/{appId}/installations", handlers.listInstallations)
	api.HandleFunc("POST /v1/apps/{appId}/installations", handlers.createInstallation)
	api.HandleFunc("GET /v1/apps/{appId}/installations/{installationId}", handlers.getInstallation)
	api.HandleFunc("PATCH /v1/apps/{appId}/installations/{installationId}", handlers.updateInstallation)
	api.HandleFunc("POST /v1/apps/{appId}/installations/{installationId}/upgrade", handlers.upgradeInstallation)
	api.HandleFunc("DELETE /v1/apps/{appId}/installations/{installationId}", handlers.uninstallInstallation)
	api.HandleFunc("GET /v1/apps/{appId}/test-install-requests", handlers.listDevelopmentInstallRequests)
	api.HandleFunc("POST /v1/apps/{appId}/test-install-requests", handlers.createDevelopmentInstallRequest)
	api.HandleFunc("POST /v1/oauth/authorizations", handlers.authorizeOAuth)
	api.HandleFunc("GET /v1/internal/developer-applications", handlers.listDeveloperApplications)
	api.HandleFunc("GET /v1/internal/organizations", handlers.listDeveloperOrganizations)
	api.HandleFunc("GET /v1/internal/organizations/{organizationId}", handlers.getDeveloperOrganization)
	api.HandleFunc("GET /v1/internal/organizations/{organizationId}/apps/{appId}/installations/{installationId}/extensions/{extensionId}/connection", handlers.getExtensionConnection)
	api.HandleFunc("PUT /v1/internal/organizations/{organizationId}/apps/{appId}/installations/{installationId}/extensions/{extensionId}/connection", handlers.provisionExtensionConnection)
	api.HandleFunc("DELETE /v1/internal/organizations/{organizationId}/apps/{appId}/installations/{installationId}/extensions/{extensionId}/connection", handlers.revokeExtensionConnection)
	api.HandleFunc("POST /v1/internal/organizations/{organizationId}/apps/{appId}/installations/{installationId}/extensions/{extensionId}/connection/rotate", handlers.rotateExtensionConnection)
	api.HandleFunc("GET /v1/internal/catalog/apps", handlers.listCatalogCandidates)
	api.HandleFunc("GET /v1/internal/organizations/{organizationId}/apps/{appId}/integration-readiness", handlers.getInternalIntegrationReadiness)
	api.HandleFunc("GET /v1/internal/organizations/{organizationId}/apps/{appId}/catalog-listing", handlers.getCatalogListing)
	api.HandleFunc("PUT /v1/internal/organizations/{organizationId}/apps/{appId}/catalog-listing", handlers.updateCatalogListing)
	api.HandleFunc("POST /v1/internal/developer-applications", handlers.createDeveloperApplication)
	api.HandleFunc("GET /v1/internal/developer-applications/{applicationId}", handlers.getDeveloperApplication)
	api.HandleFunc("POST /v1/internal/developer-applications/{applicationId}/review", handlers.reviewDeveloperApplication)
	api.HandleFunc("POST /v1/internal/developer-applications/{applicationId}/approve", handlers.approveDeveloperApplication)
	api.HandleFunc("POST /v1/internal/developer-applications/{applicationId}/reject", handlers.rejectDeveloperApplication)
	api.HandleFunc("POST /v1/internal/developer-applications/{applicationId}/invitations", handlers.rotateDeveloperInvitation)
	api.HandleFunc("POST /v1/internal/developer-invitations/{invitationId}/revoke", handlers.revokeDeveloperInvitation)
	api.HandleFunc("POST /v1/developer-invitations/accept", handlers.acceptDeveloperInvitation)

	handlers.adminLoginService = dependencies.AdminLogin
	root := http.NewServeMux()
	root.HandleFunc("POST /auth/admin/login", handlers.adminLogin)
	root.HandleFunc("GET /auth/admin/session", authenticationMiddleware(adminAuthenticator, http.HandlerFunc(handlers.getSession)).ServeHTTP)
	root.HandleFunc("POST /auth/admin/logout", authenticationMiddleware(adminAuthenticator, http.HandlerFunc(handlers.adminLogout)).ServeHTTP)
	// A separate cookie-backed trust boundary; no developer bearer fallback.
	root.Handle("/v1/internal/", authenticationMiddleware(adminAuthenticator, api))
	root.HandleFunc("GET /healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
	})
	root.HandleFunc("GET /readyz", func(writer http.ResponseWriter, request *http.Request) {
		if dependencies.Readiness != nil {
			ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
			defer cancel()
			if err := dependencies.Readiness(ctx); err != nil {
				writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
				return
			}
		}
		writeJSON(writer, http.StatusOK, map[string]string{"status": "ready"})
	})
	root.HandleFunc("POST /oauth/token", handlers.exchangeOAuthToken)
	root.HandleFunc("POST /v1/runtime/extension-credentials/resolve", handlers.resolveExtensionCredential)
	root.HandleFunc("GET /v1/scope-catalog", handlers.listScopeCatalog)
	root.HandleFunc("GET /v1/extension-catalog", handlers.listExtensionCatalog)
	root.HandleFunc("GET /v1/webhook-event-catalog", handlers.listWebhookEventCatalog)
	root.HandleFunc("GET /v1/installation-context", handlers.getInstallationContext)
	root.HandleFunc("GET /v1/installation-billing", handlers.getInstallationBilling)
	root.HandleFunc("GET /v1/merchant/installations/{installationId}/billing", handlers.getMerchantInstallationBilling)
	root.HandleFunc("POST /v1/merchant/installations/{installationId}/billing/quotes", handlers.quoteAppSubscription)
	root.HandleFunc("POST /v1/merchant/installations/{installationId}/billing/approve", handlers.approveAppSubscription)
	root.HandleFunc("DELETE /v1/merchant/installations/{installationId}/billing/subscriptions/{subscriptionId}", handlers.cancelAppSubscription)
	root.HandleFunc("PUT /v1/integrations/emisell/billing/account", handlers.syncAppBillingAccount)
	root.HandleFunc("POST /v1/integrations/emisell/billing/invoices", handlers.issueAppBillingInvoice)
	root.HandleFunc("POST /v1/integrations/emisell/billing/invoices/{invoiceId}/payment", handlers.recordAppInvoicePayment)
	root.HandleFunc("GET /v1/merchant/profile", handlers.getMerchantProfile)
	root.HandleFunc("GET /v1/products", handlers.getProducts)
	root.HandleFunc("GET /v1/products/{productId}", handlers.getProducts)
	root.HandleFunc("GET /v1/catalog/apps", handlers.listCatalogApps)
	root.HandleFunc("GET /v1/catalog/apps/{appId}", handlers.getCatalogApp)
	root.HandleFunc("POST /v1/integrations/emisell/merchant-session-grants", handlers.createEmisellMerchantSessionGrant)
	root.HandleFunc("POST /v1/integrations/emisell/shipping/rates/calculate", handlers.calculateEmisellShippingRates)
	root.HandleFunc("GET /auth/emisell-merchant/exchange", handlers.exchangeEmisellMerchantSessionGrant)
	root.HandleFunc("POST /auth/sandbox-merchant-login", handlers.sandboxMerchantLogin)
	root.HandleFunc("GET /v1/merchant/session", handlers.getMerchantSession)
	root.HandleFunc("POST /v1/merchant/session/logout", handlers.logoutMerchantSession)
	root.HandleFunc("POST /v1/merchant/oauth/preview", handlers.previewMerchantOAuth)
	root.HandleFunc("POST /v1/merchant/oauth/authorize", handlers.authorizeMerchantOAuth)
	root.HandleFunc("GET /v1/merchant/installations", handlers.listMerchantInstallations)
	root.HandleFunc("DELETE /v1/merchant/installations/{installationId}", handlers.uninstallMerchantInstallation)
	root.HandleFunc("GET /auth/login", handlers.startOIDCLogin)
	root.HandleFunc("GET /auth/callback", handlers.completeOIDCLogin)
	root.HandleFunc("POST /auth/development-login", handlers.developmentLogin)
	root.Handle("/v1/", authenticationMiddleware(dependencies.Authenticator, api))

	var handler http.Handler = root
	handler = corsMiddleware(dependencies.AllowedOrigins, handler)
	handler = securityHeadersMiddleware(handler)
	handler = loggingMiddleware(dependencies.Logger, handler)
	handler = recoveryMiddleware(dependencies.Logger, handler)
	handler = requestIDMiddleware(handler)
	return handler
}

type handlers struct {
	adminLoginService           *application.AdminLoginService
	appBilling                  *application.AppBillingService
	extensionConnections        *application.ExtensionConnectionService
	extensionCredentialRate     productRateLimiter
	products                    *emisell.Products
	shippingRates               *application.ShippingRateService
	productRate                 productRateLimiter
	apps                        *application.AppService
	versions                    *application.VersionService
	extensions                  *application.ExtensionService
	scopes                      *application.ScopeService
	credentials                 *application.CredentialService
	webhooks                    *application.WebhookService
	installations               *application.InstallationService
	developmentInstalls         *application.DevelopmentInstallService
	installationAccess          *application.InstallationAccessService
	oauth                       *application.OAuthService
	developerProgram            *application.DeveloperProgramService
	developerOrganizations      *application.DeveloperOrganizationService
	identity                    *application.IdentityService
	merchants                   *application.MerchantService
	catalog                     *application.CatalogService
	emisellIntegration          *application.EmisellIntegrationService
	emisellBackendAuthenticator EmisellBackendAuthenticator
	oidc                        *application.OIDCService
	identityHTTP                IdentityHTTPOptions
}
