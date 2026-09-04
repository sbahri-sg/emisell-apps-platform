package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"emisell-app-platform/services/app-gateway/internal/adapters/memory"
	postgresadapter "emisell-app-platform/services/app-gateway/internal/adapters/postgres"
	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/config"
	"emisell-app-platform/services/app-gateway/internal/database"
	"emisell-app-platform/services/app-gateway/internal/httpapi"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/ports"
	"emisell-app-platform/services/app-gateway/internal/security"
)

// shippingRateRepository joins the control-plane and merchant projections
// without widening the shared repository interfaces used by other services.
type shippingRateRepository struct {
	ports.Repository
	ports.MerchantRepository
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	configuration, err := config.Load()
	if err != nil {
		logger.Error("load configuration", "error", err)
		os.Exit(1)
	}

	now := func() time.Time { return time.Now().UTC() }
	products, err := config.LoadProducts(configuration.Environment)
	if err != nil {
		logger.Error("initialize resource integration", "error", err)
		os.Exit(1)
	}
	configuredShippingRateClient, err := config.LoadShippingRates(configuration.Environment)
	if err != nil {
		logger.Error("initialize API Kurir rate client", "error", err)
		os.Exit(1)
	}
	var shippingRateClient application.ShippingRateClient
	if configuredShippingRateClient != nil {
		shippingRateClient = configuredShippingRateClient
	}
	pilot, localHTTP, err := config.LoadDevelopmentPilot(configuration.Environment, products != nil)
	if err != nil {
		logger.Error("initialize local pilot", "error", err)
		os.Exit(1)
	}
	var repository ports.Repository
	var developerRepository ports.DeveloperProgramRepository
	var developerOrganizationRepository ports.DeveloperOrganizationRepository
	var identityRepository ports.IdentityRepository
	var merchantRepository ports.MerchantRepository
	var catalogRepository ports.CatalogRepository
	var merchantSessionGrantRepository ports.MerchantSessionGrantRepository
	var extensionConnectionRepository ports.ExtensionConnectionRepository
	var billingRepository ports.AppBillingRepository
	var adminLoginRepository ports.AdminLoginRepository
	var readiness func(context.Context) error
	if configuration.RepositoryDriver == "memory" {
		memoryRepository := memory.NewRepository(ids.NewUUIDv7, now)
		repository = memoryRepository
		developerRepository = memoryRepository
		developerOrganizationRepository = memoryRepository
		identityRepository = memoryRepository
		merchantRepository = memoryRepository
		catalogRepository = memoryRepository
		merchantSessionGrantRepository = memoryRepository
		extensionConnectionRepository = memoryRepository
		billingRepository = memoryRepository
		readiness = func(context.Context) error { return nil }
		logger.Warn("using non-durable memory repository")
	} else {
		connectContext, cancel := context.WithTimeout(context.Background(), configuration.Database.ConnectTimeout)
		pool, err := database.Open(connectContext, database.Options{
			URL:             configuration.Database.URL,
			MaxConnections:  configuration.Database.MaxConnections,
			MinConnections:  configuration.Database.MinConnections,
			ConnectTimeout:  configuration.Database.ConnectTimeout,
			ApplicationName: "emisell-app-gateway",
		})
		cancel()
		if err != nil {
			logger.Error("open database", "error", err)
			os.Exit(1)
		}
		defer pool.Close()
		if configuration.Environment == "development" {
			bootstrapContext, bootstrapCancel := context.WithTimeout(context.Background(), configuration.Database.ConnectTimeout)
			if err := database.BootstrapDevelopmentIdentity(
				bootstrapContext,
				pool,
				configuration.DevelopmentOrgID,
				configuration.DevelopmentUserID,
				string(configuration.DevelopmentRole),
				configuration.DevelopmentEmail,
				configuration.DevelopmentDisplayName,
			); err != nil {
				bootstrapCancel()
				logger.Error("bootstrap development identity", "error", err)
				os.Exit(1)
			}
			bootstrapCancel()
		}
		postgresRepository := postgresadapter.NewRepository(pool, ids.NewUUIDv7, now)
		repository = postgresRepository
		developerRepository = postgresRepository
		developerOrganizationRepository = postgresRepository
		identityRepository = postgresRepository
		merchantRepository = postgresRepository
		catalogRepository = postgresRepository
		merchantSessionGrantRepository = postgresRepository
		extensionConnectionRepository = postgresRepository
		billingRepository = postgresRepository
		adminLoginRepository = postgresRepository
		readiness = pool.Ping
	}
	appService := application.NewAppService(repository, ids.NewUUIDv7, now, localHTTP)
	var billing *application.AppBillingService
	if configuration.AppBillingEnabled {
		billing = application.NewAppBillingService(billingRepository, ids.NewUUIDv7, now, configuration.AppBillingLiveEnabled)
	}
	versionService := application.NewVersionService(repository, application.CurrentConfigurationSnapshotBuilder{Repository: repository}, ids.NewUUIDv7, now)
	extensionService := application.NewExtensionService(repository, ids.NewUUIDv7, now)
	scopeService := application.NewScopeService(repository)
	secretBox, err := security.NewSecretBox(configuration.SecretEncryptionKey)
	if err != nil {
		logger.Error("initialize secret encryption", "error", err)
		os.Exit(1)
	}
	credentialService := application.NewCredentialService(repository, secretBox, configuration.SecretKeyVersion, ids.NewUUIDv7, now)
	var extensionConnections *application.ExtensionConnectionService
	if configuration.ExtensionConnectionsEnabled {
		extensionConnections = application.NewExtensionConnectionService(extensionConnectionRepository, secretBox, configuration.SecretKeyVersion, ids.NewUUIDv7, now, pilot)
	}
	webhookService := application.NewWebhookService(repository, secretBox, configuration.SecretKeyVersion, ids.NewUUIDv7, now)
	installationService := application.NewInstallationService(repository, ids.NewUUIDv7, now)
	shippingRateService := application.NewShippingRateService(shippingRateRepository{Repository: repository, MerchantRepository: merchantRepository}, shippingRateClient)
	developmentInstallService := application.NewDevelopmentInstallService(repository, merchantRepository, ids.NewUUIDv7, now, 7*24*time.Hour, pilot)
	installationAccessService := application.NewInstallationAccessService(repository, now)
	oauthService := application.NewOAuthService(repository, secretBox, ids.NewUUIDv7, now, configuration.OAuthCodeTTL, configuration.OAuthAccessTokenTTL, pilot)
	developerProgramService := application.NewDeveloperProgramService(developerRepository, ids.NewUUIDv7, now, configuration.DeveloperInvitationTTL)
	developerOrganizationService := application.NewDeveloperOrganizationService(developerOrganizationRepository)
	identityService := application.NewIdentityService(identityRepository, ids.NewUUIDv7, now, configuration.Identity.SessionTTL, configuration.Identity.IdleTTL)
	var adminLoginService *application.AdminLoginService
	if adminLoginRepository != nil {
		adminLoginService, err = application.NewAdminLoginService(adminLoginRepository, ids.NewUUIDv7, now)
		if err != nil {
			logger.Error("initialize admin login")
			os.Exit(1)
		}
	}
	merchantService := application.NewMerchantService(merchantRepository, repository, identityRepository, identityService, ids.NewUUIDv7, now)
	catalogService := application.NewCatalogService(catalogRepository, repository, now)
	emisellIntegrationService := application.NewEmisellIntegrationService(
		merchantSessionGrantRepository, merchantRepository, identityRepository, identityService,
		ids.NewUUIDv7, now, configuration.EmisellBackend.GrantTTL, configuration.EmisellBackend.PublicGatewayURL,
	)
	var oidcService *application.OIDCService
	if configuration.OIDC.Enabled {
		oidcService, err = application.NewOIDCService(identityRepository, identityService, secretBox, ids.NewUUIDv7, now, application.OIDCOptions{
			Issuer: configuration.OIDC.Issuer, AuthorizationEndpoint: configuration.OIDC.AuthorizationEndpoint,
			TokenEndpoint: configuration.OIDC.TokenEndpoint, ClientID: configuration.OIDC.ClientID, ClientSecret: configuration.OIDC.ClientSecret,
			RedirectURL: configuration.OIDC.RedirectURL, KeyID: configuration.OIDC.KeyID, PublicKeyPEM: configuration.OIDC.PublicKeyPEM,
			Scopes: configuration.OIDC.Scopes, ClockSkew: configuration.OIDC.ClockSkew, LoginTTL: configuration.OIDC.LoginTTL,
		})
		if err != nil {
			logger.Error("initialize OIDC service", "error", err)
			os.Exit(1)
		}
	}
	webhookDispatcher := application.NewWebhookDispatcher(
		repository, secretBox, application.NewSafeWebhookHTTPClient(configuration.Webhook.RequestTimeout),
		ids.NewUUIDv7, now, logger, application.WebhookDispatcherOptions{
			BatchSize: configuration.Webhook.BatchSize, MaxAttempts: configuration.Webhook.MaxAttempts,
			BaseRetry: configuration.Webhook.BaseRetry, MaxRetry: configuration.Webhook.MaxRetry,
		},
	)
	var bearerAuthenticator httpapi.Authenticator
	if configuration.Environment == "development" {
		bearerAuthenticator = httpapi.DevelopmentAuthenticator{
			BearerToken: configuration.DevelopmentBearerToken, DefaultUserID: configuration.DevelopmentUserID,
			DefaultRole: configuration.DevelopmentRole, DefaultEmail: configuration.DevelopmentEmail,
			DefaultDisplayName: configuration.DevelopmentDisplayName, PlatformOperator: configuration.DevelopmentPlatformOperator,
		}
	} else {
		bearerAuthenticator, err = httpapi.NewJWTAuthenticator(
			configuration.JWT.Issuer, configuration.JWT.Audience, configuration.JWT.KeyID,
			configuration.JWT.PublicKeyPEM, configuration.JWT.ClockSkew,
		)
		if err != nil {
			logger.Error("initialize production authenticator", "error", err)
			os.Exit(1)
		}
	}
	authenticator := httpapi.CompositeAuthenticator{
		Session: httpapi.SessionAuthenticator{Identity: identityService, CookieName: configuration.Identity.SessionCookie},
		Bearer:  bearerAuthenticator,
	}
	var emisellBackendAuthenticator httpapi.EmisellBackendAuthenticator
	if configuration.Environment == "development" {
		emisellBackendAuthenticator = httpapi.DevelopmentEmisellBackendAuthenticator{BearerToken: configuration.EmisellBackend.DevelopmentToken}
	} else {
		emisellBackendAuthenticator, err = httpapi.NewJWTEmisellBackendAuthenticator(
			configuration.EmisellBackend.JWT.Issuer, configuration.EmisellBackend.JWT.Audience,
			configuration.EmisellBackend.JWT.KeyID, configuration.EmisellBackend.JWT.PublicKeyPEM,
			configuration.EmisellBackend.JWT.ClockSkew,
		)
		if err != nil {
			logger.Error("initialize Emisell Backend authenticator", "error", err)
			os.Exit(1)
		}
	}
	identityHTTP := httpapi.IdentityHTTPOptions{
		FrontendURL: configuration.Identity.FrontendURL, SessionCookieName: configuration.Identity.SessionCookie,
		CSRFCookieName: configuration.Identity.CSRFCookie, CookieSecure: configuration.Identity.CookieSecure,
		MerchantSessionCookieName: configuration.Identity.MerchantSessionCookie,
		MerchantCSRFCookieName:    configuration.Identity.MerchantCSRFCookie,
		AllowedOrigins:            configuration.AllowedOrigins,
	}
	if configuration.Environment == "development" {
		identityHTTP.Development = &httpapi.DevelopmentSessionIdentity{
			UserID: configuration.DevelopmentUserID, Email: configuration.DevelopmentEmail, DisplayName: configuration.DevelopmentDisplayName,
			PreferredOrganizationID: configuration.DevelopmentOrgID, PlatformOperator: configuration.DevelopmentPlatformOperator,
		}
	}
	handler := httpapi.NewServer(httpapi.Dependencies{
		AdminLogin:             adminLoginService,
		AppBilling:             billing,
		ExtensionConnections:   extensionConnections,
		Products:               products,
		ShippingRates:          shippingRateService,
		Apps:                   appService,
		Versions:               versionService,
		Extensions:             extensionService,
		Scopes:                 scopeService,
		Credentials:            credentialService,
		Webhooks:               webhookService,
		Installations:          installationService,
		DevelopmentInstalls:    developmentInstallService,
		InstallationAccess:     installationAccessService,
		OAuth:                  oauthService,
		DeveloperProgram:       developerProgramService,
		DeveloperOrganizations: developerOrganizationService,
		Identity:               identityService,
		Merchants:              merchantService,
		Catalog:                catalogService,
		EmisellIntegration:     emisellIntegrationService,
		EmisellBackendAuth:     emisellBackendAuthenticator,
		OIDC:                   oidcService,
		IdentityHTTP:           identityHTTP,
		Authenticator:          authenticator,
		Logger:                 logger,
		AllowedOrigins:         configuration.AllowedOrigins,
		Readiness:              readiness,
	})
	workerContext, stopWorker := context.WithCancel(context.Background())
	defer stopWorker()
	go webhookDispatcher.Run(workerContext, configuration.Webhook.PollInterval)

	server := &http.Server{
		Addr:              configuration.HTTPAddress,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	go func() {
		logger.Info("app gateway listening", "address", configuration.HTTPAddress, "environment", configuration.Environment, "repository", configuration.RepositoryDriver)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("serve app gateway", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	stopWorker()

	ctx, cancel := context.WithTimeout(context.Background(), configuration.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("shutdown app gateway", "error", err)
		os.Exit(1)
	}
	logger.Info("app gateway stopped")
}
