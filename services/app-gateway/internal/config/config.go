package config

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
)

type Config struct {
	Environment                 string
	HTTPAddress                 string
	ShutdownTimeout             time.Duration
	AllowedOrigins              []string
	RepositoryDriver            string
	Database                    DatabaseConfig
	DevelopmentBearerToken      string
	DevelopmentOrgID            string
	DevelopmentUserID           string
	DevelopmentRole             domain.Role
	DevelopmentEmail            string
	DevelopmentDisplayName      string
	DevelopmentPlatformOperator bool
	SecretEncryptionKey         []byte
	SecretKeyVersion            int
	ExtensionConnectionsEnabled bool
	AppBillingEnabled           bool
	AppBillingLiveEnabled       bool
	JWT                         JWTConfig
	EmisellBackend              EmisellBackendConfig
	Identity                    IdentityConfig
	OIDC                        OIDCConfig
	OAuthCodeTTL                time.Duration
	OAuthAccessTokenTTL         time.Duration
	DeveloperInvitationTTL      time.Duration
	Webhook                     WebhookConfig
}

type EmisellBackendConfig struct {
	PublicGatewayURL string
	GrantTTL         time.Duration
	DevelopmentToken string
	JWT              JWTConfig
}

type IdentityConfig struct {
	FrontendURL           string
	SessionCookie         string
	CSRFCookie            string
	MerchantSessionCookie string
	MerchantCSRFCookie    string
	SessionTTL            time.Duration
	IdleTTL               time.Duration
	CookieSecure          bool
}

type OIDCConfig struct {
	Enabled               bool
	Issuer                string
	AuthorizationEndpoint string
	TokenEndpoint         string
	ClientID              string
	ClientSecret          string
	RedirectURL           string
	KeyID                 string
	PublicKeyPEM          []byte
	Scopes                []string
	ClockSkew             time.Duration
	LoginTTL              time.Duration
}

type WebhookConfig struct {
	PollInterval   time.Duration
	RequestTimeout time.Duration
	BatchSize      int
	MaxAttempts    int
	BaseRetry      time.Duration
	MaxRetry       time.Duration
}

type JWTConfig struct {
	Issuer       string
	Audience     string
	KeyID        string
	PublicKeyPEM []byte
	ClockSkew    time.Duration
}

type DatabaseConfig struct {
	URL            string
	MaxConnections int32
	MinConnections int32
	ConnectTimeout time.Duration
}

func Load() (Config, error) {
	database, err := LoadDatabase()
	if err != nil {
		return Config{}, err
	}
	shutdownTimeout, err := time.ParseDuration(value("SHUTDOWN_TIMEOUT", "10s"))
	if err != nil {
		return Config{}, fmt.Errorf("parse SHUTDOWN_TIMEOUT: %w", err)
	}
	environment := value("APP_ENV", "development")
	if environment != "development" && environment != "production" {
		return Config{}, fmt.Errorf("APP_ENV must be development or production")
	}
	appBillingEnabled, err := strconv.ParseBool(value("APP_BILLING_ENABLED", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("APP_BILLING_ENABLED must be true or false")
	}
	appBillingLiveEnabled, err := strconv.ParseBool(value("APP_BILLING_LIVE_ENABLED", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("APP_BILLING_LIVE_ENABLED must be true or false")
	}
	if appBillingLiveEnabled && (!appBillingEnabled || environment != "production") {
		return Config{}, fmt.Errorf("live app billing requires APP_BILLING_ENABLED and APP_ENV=production")
	}
	role := domain.Role(value("DEVELOPMENT_ROLE", string(domain.RoleOwner)))
	if role != domain.RoleOwner && role != domain.RoleAdmin && role != domain.RoleDeveloper && role != domain.RoleAnalyst {
		return Config{}, fmt.Errorf("invalid DEVELOPMENT_ROLE %q", role)
	}
	secretKeyEncoded := strings.TrimSpace(os.Getenv("SECRET_ENCRYPTION_KEY_BASE64"))
	if secretKeyEncoded == "" && environment == "development" {
		secretKeyEncoded = "r8PQPmYOmajyNIx3Gn4bEISAPze1pvrCs8vB/WOjjjc="
	}
	secretKey, err := base64.StdEncoding.DecodeString(secretKeyEncoded)
	if err != nil || len(secretKey) != 32 {
		return Config{}, fmt.Errorf("SECRET_ENCRYPTION_KEY_BASE64 must decode to exactly 32 bytes")
	}
	secretKeyVersion, err := strconv.Atoi(value("SECRET_ENCRYPTION_KEY_VERSION", "1"))
	if err != nil || secretKeyVersion < 1 {
		return Config{}, fmt.Errorf("SECRET_ENCRYPTION_KEY_VERSION must be a positive integer")
	}
	oauthCodeTTL, err := boundedDuration("OAUTH_CODE_TTL", "5m", time.Minute, 10*time.Minute)
	if err != nil {
		return Config{}, err
	}
	oauthAccessTokenTTL, err := boundedDuration("OAUTH_ACCESS_TOKEN_TTL", "1h", 5*time.Minute, 24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	developerInvitationTTL, err := boundedDuration("DEVELOPER_INVITATION_TTL", "48h", time.Hour, 7*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	sessionTTL, err := boundedDuration("AUTH_SESSION_TTL", "12h", time.Hour, 30*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	sessionIdleTTL, err := boundedDuration("AUTH_SESSION_IDLE_TTL", "2h", 15*time.Minute, sessionTTL)
	if err != nil {
		return Config{}, err
	}
	oidcEnabled, err := strconv.ParseBool(value("AUTH_OIDC_ENABLED", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("AUTH_OIDC_ENABLED must be true or false")
	}
	extensionConnectionsEnabled, err := strconv.ParseBool(value("EXTENSION_CONNECTIONS_ENABLED", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("EXTENSION_CONNECTIONS_ENABLED must be true or false")
	}
	if extensionConnectionsEnabled && environment == "production" && secretKeyEncoded == "r8PQPmYOmajyNIx3Gn4bEISAPze1pvrCs8vB/WOjjjc=" {
		return Config{}, fmt.Errorf("managed extensions cannot use the public development encryption key in production")
	}
	oidcClockSkew, err := boundedDuration("AUTH_OIDC_CLOCK_SKEW", "30s", 0, 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	oidcLoginTTL, err := boundedDuration("AUTH_OIDC_LOGIN_TTL", "5m", time.Minute, 10*time.Minute)
	if err != nil {
		return Config{}, err
	}
	oidcConfig := OIDCConfig{Enabled: oidcEnabled, ClockSkew: oidcClockSkew, LoginTTL: oidcLoginTTL}
	if oidcEnabled {
		publicKey, err := base64.StdEncoding.DecodeString(strings.TrimSpace(os.Getenv("AUTH_OIDC_PUBLIC_KEY_BASE64")))
		if err != nil || len(publicKey) == 0 {
			return Config{}, fmt.Errorf("AUTH_OIDC_PUBLIC_KEY_BASE64 must contain a base64-encoded RSA public key when OIDC is enabled")
		}
		oidcConfig = OIDCConfig{
			Enabled: true, Issuer: strings.TrimSpace(os.Getenv("AUTH_OIDC_ISSUER")),
			AuthorizationEndpoint: strings.TrimSpace(os.Getenv("AUTH_OIDC_AUTHORIZATION_ENDPOINT")),
			TokenEndpoint:         strings.TrimSpace(os.Getenv("AUTH_OIDC_TOKEN_ENDPOINT")), ClientID: strings.TrimSpace(os.Getenv("AUTH_OIDC_CLIENT_ID")),
			ClientSecret: strings.TrimSpace(os.Getenv("AUTH_OIDC_CLIENT_SECRET")), RedirectURL: strings.TrimSpace(os.Getenv("AUTH_OIDC_REDIRECT_URL")),
			KeyID: strings.TrimSpace(os.Getenv("AUTH_OIDC_KEY_ID")), PublicKeyPEM: publicKey,
			Scopes: split(value("AUTH_OIDC_SCOPES", "openid,profile,email")), ClockSkew: oidcClockSkew, LoginTTL: oidcLoginTTL,
		}
		if oidcConfig.Issuer == "" || oidcConfig.AuthorizationEndpoint == "" || oidcConfig.TokenEndpoint == "" || oidcConfig.ClientID == "" || oidcConfig.RedirectURL == "" || oidcConfig.KeyID == "" {
			return Config{}, fmt.Errorf("OIDC issuer, endpoints, client id, redirect URL, and key id are required when OIDC is enabled")
		}
	}
	developmentPlatformOperator, err := strconv.ParseBool(value("DEVELOPMENT_PLATFORM_OPERATOR", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("DEVELOPMENT_PLATFORM_OPERATOR must be true or false")
	}
	webhookPoll, err := boundedDuration("WEBHOOK_POLL_INTERVAL", "1s", 100*time.Millisecond, time.Minute)
	if err != nil {
		return Config{}, err
	}
	webhookTimeout, err := boundedDuration("WEBHOOK_REQUEST_TIMEOUT", "8s", time.Second, 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	webhookBaseRetry, err := boundedDuration("WEBHOOK_BASE_RETRY", "5s", time.Second, 10*time.Minute)
	if err != nil {
		return Config{}, err
	}
	webhookMaxRetry, err := boundedDuration("WEBHOOK_MAX_RETRY", "15m", webhookBaseRetry, 24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	webhookBatchSize, err := strconv.Atoi(value("WEBHOOK_BATCH_SIZE", "25"))
	if err != nil || webhookBatchSize < 1 || webhookBatchSize > 100 {
		return Config{}, fmt.Errorf("WEBHOOK_BATCH_SIZE must be between 1 and 100")
	}
	webhookMaxAttempts, err := strconv.Atoi(value("WEBHOOK_MAX_ATTEMPTS", "5"))
	if err != nil || webhookMaxAttempts < 1 || webhookMaxAttempts > 10 {
		return Config{}, fmt.Errorf("WEBHOOK_MAX_ATTEMPTS must be between 1 and 10")
	}
	jwtConfig := JWTConfig{}
	if environment == "production" {
		clockSkew, err := time.ParseDuration(value("AUTH_JWT_CLOCK_SKEW", "30s"))
		if err != nil || clockSkew < 0 || clockSkew > 5*time.Minute {
			return Config{}, fmt.Errorf("AUTH_JWT_CLOCK_SKEW must be between 0s and 5m")
		}
		publicKey, err := base64.StdEncoding.DecodeString(strings.TrimSpace(os.Getenv("AUTH_JWT_PUBLIC_KEY_BASE64")))
		if err != nil || len(publicKey) == 0 {
			return Config{}, fmt.Errorf("AUTH_JWT_PUBLIC_KEY_BASE64 must contain a base64-encoded RSA public key in production")
		}
		jwtConfig = JWTConfig{
			Issuer: strings.TrimSpace(os.Getenv("AUTH_JWT_ISSUER")), Audience: strings.TrimSpace(os.Getenv("AUTH_JWT_AUDIENCE")),
			KeyID: strings.TrimSpace(os.Getenv("AUTH_JWT_KEY_ID")), PublicKeyPEM: publicKey, ClockSkew: clockSkew,
		}
		if jwtConfig.Issuer == "" || jwtConfig.Audience == "" || jwtConfig.KeyID == "" {
			return Config{}, fmt.Errorf("AUTH_JWT_ISSUER, AUTH_JWT_AUDIENCE, and AUTH_JWT_KEY_ID are required in production")
		}
	}
	emisellGrantTTL, err := boundedDuration("EMISELL_BACKEND_SESSION_GRANT_TTL", "2m", 30*time.Second, 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	emisellJWTConfig := JWTConfig{}
	if environment == "production" {
		clockSkew, err := boundedDuration("EMISELL_BACKEND_JWT_CLOCK_SKEW", "30s", 0, 5*time.Minute)
		if err != nil {
			return Config{}, err
		}
		publicKey, err := base64.StdEncoding.DecodeString(strings.TrimSpace(os.Getenv("EMISELL_BACKEND_JWT_PUBLIC_KEY_BASE64")))
		if err != nil || len(publicKey) == 0 {
			return Config{}, fmt.Errorf("EMISELL_BACKEND_JWT_PUBLIC_KEY_BASE64 must contain a base64-encoded RSA public key in production")
		}
		emisellJWTConfig = JWTConfig{
			Issuer: strings.TrimSpace(os.Getenv("EMISELL_BACKEND_JWT_ISSUER")), Audience: strings.TrimSpace(os.Getenv("EMISELL_BACKEND_JWT_AUDIENCE")),
			KeyID: strings.TrimSpace(os.Getenv("EMISELL_BACKEND_JWT_KEY_ID")), PublicKeyPEM: publicKey, ClockSkew: clockSkew,
		}
		if emisellJWTConfig.Issuer == "" || emisellJWTConfig.Audience == "" || emisellJWTConfig.KeyID == "" {
			return Config{}, fmt.Errorf("EMISELL_BACKEND_JWT_ISSUER, EMISELL_BACKEND_JWT_AUDIENCE, and EMISELL_BACKEND_JWT_KEY_ID are required in production")
		}
	}
	configuration := Config{
		AppBillingEnabled:           appBillingEnabled,
		AppBillingLiveEnabled:       appBillingLiveEnabled,
		ExtensionConnectionsEnabled: extensionConnectionsEnabled,
		Environment:                 environment,
		HTTPAddress:                 value("HTTP_ADDRESS", ":8080"),
		ShutdownTimeout:             shutdownTimeout,
		AllowedOrigins:              split(value("CORS_ALLOWED_ORIGINS", "http://localhost:3003")),
		RepositoryDriver:            value("REPOSITORY_DRIVER", "postgres"),
		Database:                    database,
		DevelopmentBearerToken:      value("DEVELOPMENT_BEARER_TOKEN", "emisell-local-dev-token"),
		DevelopmentOrgID:            value("DEVELOPMENT_ORGANIZATION_ID", "01995f72-0000-7000-8000-000000000001"),
		DevelopmentUserID:           value("DEVELOPMENT_USER_ID", "01995f72-0000-7000-8000-000000000002"),
		DevelopmentRole:             role,
		DevelopmentEmail:            strings.ToLower(value("DEVELOPMENT_EMAIL", "developer@local.emisell.test")),
		DevelopmentDisplayName:      value("DEVELOPMENT_DISPLAY_NAME", "Local Developer"),
		DevelopmentPlatformOperator: developmentPlatformOperator,
		SecretEncryptionKey:         secretKey,
		SecretKeyVersion:            secretKeyVersion,
		JWT:                         jwtConfig,
		EmisellBackend: EmisellBackendConfig{
			PublicGatewayURL: value("PUBLIC_GATEWAY_URL", "http://localhost:8081"),
			GrantTTL:         emisellGrantTTL, DevelopmentToken: value("EMISELL_BACKEND_DEVELOPMENT_TOKEN", "emisell-backend-local-token"),
			JWT: emisellJWTConfig,
		},
		Identity: IdentityConfig{
			FrontendURL: value("FRONTEND_URL", "http://localhost:3003"), SessionCookie: value("AUTH_SESSION_COOKIE_NAME", "emisell_session"),
			CSRFCookie:            value("AUTH_CSRF_COOKIE_NAME", "emisell_csrf"),
			MerchantSessionCookie: value("MERCHANT_SESSION_COOKIE_NAME", "emisell_merchant_session"),
			MerchantCSRFCookie:    value("MERCHANT_CSRF_COOKIE_NAME", "emisell_merchant_csrf"), SessionTTL: sessionTTL, IdleTTL: sessionIdleTTL,
			CookieSecure: environment == "production",
		},
		OIDC:                   oidcConfig,
		OAuthCodeTTL:           oauthCodeTTL,
		OAuthAccessTokenTTL:    oauthAccessTokenTTL,
		DeveloperInvitationTTL: developerInvitationTTL,
		Webhook: WebhookConfig{
			PollInterval: webhookPoll, RequestTimeout: webhookTimeout, BatchSize: webhookBatchSize,
			MaxAttempts: webhookMaxAttempts, BaseRetry: webhookBaseRetry, MaxRetry: webhookMaxRetry,
		},
	}
	if configuration.RepositoryDriver != "postgres" && configuration.RepositoryDriver != "memory" {
		return Config{}, fmt.Errorf("invalid REPOSITORY_DRIVER %q", configuration.RepositoryDriver)
	}
	if configuration.Environment == "production" && configuration.RepositoryDriver != "postgres" {
		return Config{}, fmt.Errorf("production requires REPOSITORY_DRIVER=postgres")
	}
	frontendURL, err := url.Parse(configuration.Identity.FrontendURL)
	if err != nil || frontendURL.Host == "" || (frontendURL.Scheme != "http" && frontendURL.Scheme != "https") || frontendURL.Path != "" {
		return Config{}, fmt.Errorf("FRONTEND_URL must be an absolute origin without a path")
	}
	if configuration.Environment == "production" && frontendURL.Scheme != "https" {
		return Config{}, fmt.Errorf("production FRONTEND_URL must use HTTPS")
	}
	publicGatewayURL, err := url.Parse(configuration.EmisellBackend.PublicGatewayURL)
	if err != nil || publicGatewayURL.Host == "" || (publicGatewayURL.Scheme != "http" && publicGatewayURL.Scheme != "https") || publicGatewayURL.Path != "" {
		return Config{}, fmt.Errorf("PUBLIC_GATEWAY_URL must be an absolute origin without a path")
	}
	if configuration.Environment == "production" && publicGatewayURL.Scheme != "https" {
		return Config{}, fmt.Errorf("production PUBLIC_GATEWAY_URL must use HTTPS")
	}
	if configuration.OIDC.Enabled {
		for name, rawURL := range map[string]string{
			"AUTH_OIDC_ISSUER": configuration.OIDC.Issuer, "AUTH_OIDC_AUTHORIZATION_ENDPOINT": configuration.OIDC.AuthorizationEndpoint,
			"AUTH_OIDC_TOKEN_ENDPOINT": configuration.OIDC.TokenEndpoint, "AUTH_OIDC_REDIRECT_URL": configuration.OIDC.RedirectURL,
		} {
			parsed, err := url.Parse(rawURL)
			if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
				return Config{}, fmt.Errorf("%s must be an absolute URL", name)
			}
			if configuration.Environment == "production" && parsed.Scheme != "https" {
				return Config{}, fmt.Errorf("production %s must use HTTPS", name)
			}
		}
	}
	for _, cookieName := range []string{
		configuration.Identity.SessionCookie, configuration.Identity.CSRFCookie,
		configuration.Identity.MerchantSessionCookie, configuration.Identity.MerchantCSRFCookie,
	} {
		if !cookieNamePattern.MatchString(cookieName) {
			return Config{}, fmt.Errorf("authentication cookie names may contain only letters, digits, underscore, and hyphen")
		}
		if cookieName == "emisell_admin_session" || cookieName == "emisell_admin_csrf" {
			return Config{}, fmt.Errorf("admin cookie names are reserved and cannot be reused by developer or merchant sessions")
		}
	}
	return configuration, nil
}

var cookieNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func boundedDuration(key, fallback string, minimum, maximum time.Duration) (time.Duration, error) {
	parsed, err := time.ParseDuration(value(key, fallback))
	if err != nil || parsed < minimum || parsed > maximum {
		return 0, fmt.Errorf("%s must be between %s and %s", key, minimum, maximum)
	}
	return parsed, nil
}

func LoadDatabase() (DatabaseConfig, error) {
	connectTimeout, err := time.ParseDuration(value("DATABASE_CONNECT_TIMEOUT", "5s"))
	if err != nil {
		return DatabaseConfig{}, fmt.Errorf("parse DATABASE_CONNECT_TIMEOUT: %w", err)
	}
	maxConnections, err := parseInt32("DATABASE_MAX_CONNECTIONS", 10)
	if err != nil {
		return DatabaseConfig{}, err
	}
	minConnections, err := parseInt32("DATABASE_MIN_CONNECTIONS", 1)
	if err != nil {
		return DatabaseConfig{}, err
	}
	if maxConnections < 1 || minConnections < 0 || minConnections > maxConnections {
		return DatabaseConfig{}, fmt.Errorf("database connection limits must satisfy 0 <= min <= max")
	}
	return DatabaseConfig{
		URL:            value("DATABASE_URL", "postgres://emisell:emisell-dev@localhost:5432/emisell_app_platform?sslmode=disable"),
		MaxConnections: maxConnections,
		MinConnections: minConnections,
		ConnectTimeout: connectTimeout,
	}, nil
}

func value(key, fallback string) string {
	if current := strings.TrimSpace(os.Getenv(key)); current != "" {
		return current
	}
	return fallback
}

func split(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func parseInt32(key string, fallback int32) (int32, error) {
	raw := value(key, strconv.FormatInt(int64(fallback), 10))
	parsed, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return int32(parsed), nil
}
