package domain

import "time"

type AppStatus string

const (
	AppStatusDraft    AppStatus = "draft"
	AppStatusActive   AppStatus = "active"
	AppStatusArchived AppStatus = "archived"
)

type AppReleaseStatus string

const (
	AppReleaseStatusDevelopment AppReleaseStatus = "development"
	AppReleaseStatusReleased    AppReleaseStatus = "released"
)

type Distribution string

const (
	DistributionPublic Distribution = "public"
	DistributionCustom Distribution = "custom"
)

type CatalogListingStatus string

const (
	CatalogListingStatusDraft     CatalogListingStatus = "draft"
	CatalogListingStatusPublished CatalogListingStatus = "published"
	CatalogListingStatusHidden    CatalogListingStatus = "hidden"
)

type CatalogCategory string

const (
	CatalogCategoryPayment    CatalogCategory = "payment"
	CatalogCategoryShipping   CatalogCategory = "shipping"
	CatalogCategoryERP        CatalogCategory = "erp"
	CatalogCategoryMarketing  CatalogCategory = "marketing"
	CatalogCategoryOperations CatalogCategory = "operations"
	CatalogCategoryCustom     CatalogCategory = "custom"
)

type VersionStatus string

const (
	VersionStatusDraft    VersionStatus = "draft"
	VersionStatusReleased VersionStatus = "released"
	VersionStatusActive   VersionStatus = "active"
)

type ExtensionType string

const (
	ExtensionTypePayment  ExtensionType = "payment"
	ExtensionTypeShipping ExtensionType = "shipping"
	ExtensionTypeCustom   ExtensionType = "custom"
)

type ExtensionStatus string

const (
	ExtensionStatusDraft    ExtensionStatus = "draft"
	ExtensionStatusActive   ExtensionStatus = "active"
	ExtensionStatusDisabled ExtensionStatus = "disabled"
)

type ScopeAccess string

const (
	ScopeAccessRequired ScopeAccess = "required"
	ScopeAccessOptional ScopeAccess = "optional"
)

type Environment string

const (
	EnvironmentSandbox    Environment = "sandbox"
	EnvironmentProduction Environment = "production"
)

type CredentialStatus string

const (
	CredentialStatusActive  CredentialStatus = "active"
	CredentialStatusRevoked CredentialStatus = "revoked"
	CredentialStatusExpired CredentialStatus = "expired"
)

type WebhookStatus string

const (
	WebhookStatusPending  WebhookStatus = "pending"
	WebhookStatusActive   WebhookStatus = "active"
	WebhookStatusPaused   WebhookStatus = "paused"
	WebhookStatusFailing  WebhookStatus = "failing"
	WebhookStatusDisabled WebhookStatus = "disabled"
)

type WebhookEventAvailability string

const (
	WebhookEventAvailabilityAvailable WebhookEventAvailability = "available"
	WebhookEventAvailabilityPlanned   WebhookEventAvailability = "planned"
)

type WebhookEventSource string

const (
	WebhookEventSourceAppPlatform    WebhookEventSource = "app_platform"
	WebhookEventSourceEmisellBackend WebhookEventSource = "emisell_backend"
	WebhookEventSourceTest           WebhookEventSource = "test"
)

type InstallationStatus string

const (
	InstallationStatusActive      InstallationStatus = "active"
	InstallationStatusSuspended   InstallationStatus = "suspended"
	InstallationStatusUninstalled InstallationStatus = "uninstalled"
)

type DevelopmentInstallRequestStatus string

const (
	DevelopmentInstallRequestStatusPending    DevelopmentInstallRequestStatus = "pending"
	DevelopmentInstallRequestStatusAuthorized DevelopmentInstallRequestStatus = "authorized"
	DevelopmentInstallRequestStatusCancelled  DevelopmentInstallRequestStatus = "cancelled"
	DevelopmentInstallRequestStatusExpired    DevelopmentInstallRequestStatus = "expired"
)

type WebhookDeliveryStatus string

const (
	WebhookDeliveryStatusPending   WebhookDeliveryStatus = "pending"
	WebhookDeliveryStatusDelivered WebhookDeliveryStatus = "delivered"
	WebhookDeliveryStatusFailed    WebhookDeliveryStatus = "failed"
)

type Role string

const (
	RoleOwner     Role = "owner"
	RoleAdmin     Role = "admin"
	RoleDeveloper Role = "developer"
	RoleAnalyst   Role = "analyst"
)

type DeveloperApplicationStatus string

const (
	DeveloperApplicationStatusSubmitted   DeveloperApplicationStatus = "submitted"
	DeveloperApplicationStatusUnderReview DeveloperApplicationStatus = "under_review"
	DeveloperApplicationStatusApproved    DeveloperApplicationStatus = "approved"
	DeveloperApplicationStatusInvited     DeveloperApplicationStatus = "invited"
	DeveloperApplicationStatusActive      DeveloperApplicationStatus = "active"
	DeveloperApplicationStatusRejected    DeveloperApplicationStatus = "rejected"
)

type DeveloperAppType string

const (
	DeveloperAppTypePayment   DeveloperAppType = "payment"
	DeveloperAppTypeShipping  DeveloperAppType = "shipping"
	DeveloperAppTypeERP       DeveloperAppType = "erp"
	DeveloperAppTypeMarketing DeveloperAppType = "marketing"
	DeveloperAppTypeCustom    DeveloperAppType = "custom"
)

type DeveloperInvitationStatus string

const (
	DeveloperInvitationStatusPending  DeveloperInvitationStatus = "pending"
	DeveloperInvitationStatusAccepted DeveloperInvitationStatus = "accepted"
	DeveloperInvitationStatusRevoked  DeveloperInvitationStatus = "revoked"
	DeveloperInvitationStatusExpired  DeveloperInvitationStatus = "expired"
)

type DeveloperApplication struct {
	ID                string                     `json:"id"`
	PlatformOrgID     string                     `json:"-"`
	CompanyName       string                     `json:"companyName"`
	CompanyDomain     string                     `json:"companyDomain"`
	ContactName       string                     `json:"contactName"`
	ContactEmail      string                     `json:"contactEmail"`
	RequestedAppName  string                     `json:"requestedAppName"`
	AppType           DeveloperAppType           `json:"appType"`
	UseCase           string                     `json:"useCase"`
	RequestedScopes   []string                   `json:"requestedScopes"`
	Status            DeveloperApplicationStatus `json:"status"`
	ReviewNotes       *string                    `json:"reviewNotes"`
	OrganizationID    *string                    `json:"organizationId"`
	SubmittedBy       string                     `json:"submittedBy"`
	ReviewedBy        *string                    `json:"reviewedBy"`
	ReviewedAt        *time.Time                 `json:"reviewedAt"`
	CreatedAt         time.Time                  `json:"createdAt"`
	UpdatedAt         time.Time                  `json:"updatedAt"`
	Revision          int64                      `json:"revision"`
	CurrentInvitation *DeveloperInvitation       `json:"currentInvitation,omitempty"`
}

type DeveloperInvitation struct {
	ID             string                    `json:"id"`
	ApplicationID  string                    `json:"applicationId"`
	OrganizationID string                    `json:"organizationId"`
	Email          string                    `json:"email"`
	Role           Role                      `json:"role"`
	TokenHash      string                    `json:"-"`
	Status         DeveloperInvitationStatus `json:"status"`
	CreatedBy      string                    `json:"createdBy"`
	AcceptedBy     *string                   `json:"acceptedBy"`
	CreatedAt      time.Time                 `json:"createdAt"`
	ExpiresAt      time.Time                 `json:"expiresAt"`
	AcceptedAt     *time.Time                `json:"acceptedAt"`
	RevokedAt      *time.Time                `json:"revokedAt"`
}

type OrganizationEntitlement struct {
	OrganizationID   string    `json:"organizationId"`
	SandboxAccess    bool      `json:"sandboxAccess"`
	ProductionAccess bool      `json:"productionAccess"`
	MaxApps          int       `json:"maxApps"`
	MaxWebhooks      int       `json:"maxWebhooks"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type OrganizationStatus string

const (
	OrganizationStatusActive    OrganizationStatus = "active"
	OrganizationStatusSuspended OrganizationStatus = "suspended"
)

type DeveloperOrganization struct {
	ID              string                  `json:"id"`
	Name            string                  `json:"name"`
	Slug            string                  `json:"slug"`
	Status          OrganizationStatus      `json:"status"`
	Entitlement     OrganizationEntitlement `json:"entitlement"`
	AppCount        int                     `json:"appCount"`
	MembershipCount int                     `json:"membershipCount"`
	CreatedAt       time.Time               `json:"createdAt"`
	UpdatedAt       time.Time               `json:"updatedAt"`
}

type DeveloperOrganizationMember struct {
	UserID      string    `json:"userId"`
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
	Role        Role      `json:"role"`
	CreatedAt   time.Time `json:"createdAt"`
}

type DeveloperOrganizationDetail struct {
	DeveloperOrganization
	Memberships []DeveloperOrganizationMember `json:"memberships"`
}

type OrganizationMembership struct {
	OrganizationID string `json:"organizationId"`
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	Status         string `json:"status"`
	Role           Role   `json:"role"`
}

type IdentitySession struct {
	ID                  string       `json:"-"`
	TokenHash           string       `json:"-"`
	CSRFTokenHash       string       `json:"-"`
	UserID              string       `json:"userId"`
	ActiveOrgID         *string      `json:"activeOrganizationId"`
	MerchantID          *string      `json:"-"`
	MerchantEnvironment *Environment `json:"-"`
	Email               string       `json:"email"`
	DisplayName         string       `json:"displayName"`
	PlatformOperator    bool         `json:"platformOperator"`
	CreatedAt           time.Time    `json:"createdAt"`
	LastSeenAt          time.Time    `json:"lastSeenAt"`
	ExpiresAt           time.Time    `json:"expiresAt"`
	IdleExpiresAt       time.Time    `json:"idleExpiresAt"`
	RevokedAt           *time.Time   `json:"-"`
}

type MerchantIdentity struct {
	MerchantID  string      `json:"merchantId"`
	UserID      string      `json:"-"`
	Name        string      `json:"name"`
	Domain      *string     `json:"domain"`
	Environment Environment `json:"environment"`
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
}

type AppCatalogListing struct {
	AppID          string               `json:"appId"`
	OrganizationID string               `json:"organizationId"`
	Category       CatalogCategory      `json:"category"`
	Status         CatalogListingStatus `json:"status"`
	Featured       bool                 `json:"featured"`
	PublishedBy    *string              `json:"publishedBy"`
	PublishedAt    *time.Time           `json:"publishedAt"`
	UpdatedAt      time.Time            `json:"updatedAt"`
	Revision       int64                `json:"revision"`
}

type CatalogApp struct {
	AppID            string          `json:"appId"`
	Slug             string          `json:"slug"`
	Name             string          `json:"name"`
	Description      *string         `json:"description"`
	DeveloperName    string          `json:"developerName"`
	Category         CatalogCategory `json:"category"`
	Featured         bool            `json:"featured"`
	LaunchURL        string          `json:"launchUrl"`
	ActiveVersionID  string          `json:"activeVersionId"`
	Version          string          `json:"version"`
	ExtensionTypes   []ExtensionType `json:"extensionTypes"`
	RequiredScopes   []SnapshotScope `json:"requiredScopes"`
	OptionalScopes   []SnapshotScope `json:"optionalScopes"`
	PublishedAt      time.Time       `json:"publishedAt"`
	ListingUpdatedAt time.Time       `json:"listingUpdatedAt"`
}

type CatalogCandidate struct {
	OrganizationID string             `json:"organizationId"`
	DeveloperName  string             `json:"developerName"`
	App            App                `json:"app"`
	ActiveVersion  *AppVersion        `json:"activeVersion"`
	Listing        *AppCatalogListing `json:"listing"`
	Eligible       bool               `json:"eligible"`
	BlockingReason *string            `json:"blockingReason"`
}

type MerchantSessionGrant struct {
	ID           string      `json:"-"`
	CodeHash     string      `json:"-"`
	SourceJTI    string      `json:"-"`
	Subject      string      `json:"-"`
	Email        string      `json:"-"`
	DisplayName  string      `json:"-"`
	MerchantID   string      `json:"-"`
	MerchantName string      `json:"-"`
	Domain       *string     `json:"-"`
	Environment  Environment `json:"-"`
	Permissions  []string    `json:"-"`
	ReturnTo     string      `json:"-"`
	CreatedAt    time.Time   `json:"-"`
	ExpiresAt    time.Time   `json:"expiresAt"`
	ConsumedAt   *time.Time  `json:"-"`
}

type OIDCLoginState struct {
	ID                     string     `json:"-"`
	StateHash              string     `json:"-"`
	NonceHash              string     `json:"-"`
	CodeVerifierCiphertext []byte     `json:"-"`
	ReturnTo               string     `json:"-"`
	CreatedAt              time.Time  `json:"-"`
	ExpiresAt              time.Time  `json:"-"`
	ConsumedAt             *time.Time `json:"-"`
}

type App struct {
	ID              string           `json:"id"`
	OrganizationID  string           `json:"organizationId"`
	Name            string           `json:"name"`
	Slug            string           `json:"slug"`
	Description     *string          `json:"description"`
	Distribution    Distribution     `json:"distribution"`
	Status          AppStatus        `json:"status"`
	ReleaseStatus   AppReleaseStatus `json:"releaseStatus"`
	AppURL          *string          `json:"appUrl"`
	ContactEmail    *string          `json:"contactEmail"`
	ActiveVersionID *string          `json:"activeVersionId"`
	CreatedBy       string           `json:"createdBy"`
	CreatedAt       time.Time        `json:"createdAt"`
	UpdatedAt       time.Time        `json:"updatedAt"`
	Revision        int64            `json:"revision"`
}

type VersionSnapshot struct {
	Extensions           []SnapshotExtension `json:"extensions"`
	Scopes               []SnapshotScope     `json:"scopes"`
	WebhookSubscriptions []SnapshotWebhook   `json:"webhookSubscriptions"`
	RedirectURLs         []string            `json:"redirectUrls"`
	ConfigurationHash    string              `json:"configurationHash"`
}

type SnapshotExtension struct {
	ExtensionID   string         `json:"extensionId"`
	Name          string         `json:"name"`
	Type          string         `json:"type"`
	RuntimeURL    *string        `json:"runtimeUrl"`
	Configuration map[string]any `json:"configuration"`
}

type SnapshotScope struct {
	Scope  string `json:"scope"`
	Access string `json:"access"`
}

type SnapshotWebhook struct {
	SubscriptionID string `json:"subscriptionId"`
	Event          string `json:"event"`
	EndpointURL    string `json:"endpointUrl"`
}

type AppVersion struct {
	ID          string          `json:"id"`
	AppID       string          `json:"appId"`
	Version     string          `json:"version"`
	Status      VersionStatus   `json:"status"`
	ReleaseNote *string         `json:"releaseNote"`
	Snapshot    VersionSnapshot `json:"snapshot"`
	CreatedBy   string          `json:"createdBy"`
	ReleasedBy  *string         `json:"releasedBy"`
	CreatedAt   time.Time       `json:"createdAt"`
	ReleasedAt  *time.Time      `json:"releasedAt"`
}

type AppExtension struct {
	ID            string                 `json:"id"`
	AppID         string                 `json:"appId"`
	Name          string                 `json:"name"`
	Type          ExtensionType          `json:"type"`
	Status        ExtensionStatus        `json:"status"`
	RuntimeURL    *string                `json:"runtimeUrl"`
	Configuration map[string]interface{} `json:"configuration"`
	CreatedAt     time.Time              `json:"createdAt"`
	UpdatedAt     time.Time              `json:"updatedAt"`
	Revision      int64                  `json:"revision"`
}

type AppScope struct {
	AppID  string      `json:"-"`
	Scope  string      `json:"scope"`
	Access ScopeAccess `json:"access"`
}

type ScopeAvailability string

const (
	ScopeAvailabilityAvailable ScopeAvailability = "available"
	ScopeAvailabilityPlanned   ScopeAvailability = "planned"
)

type ScopeRisk string

const (
	ScopeRiskLow       ScopeRisk = "low"
	ScopeRiskStandard  ScopeRisk = "standard"
	ScopeRiskSensitive ScopeRisk = "sensitive"
	ScopeRiskHigh      ScopeRisk = "high"
)

type ScopeEndpoint struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

type ScopeDefinition struct {
	Scope               string            `json:"scope"`
	Name                string            `json:"name"`
	Description         string            `json:"description"`
	Access              string            `json:"access"`
	Risk                ScopeRisk         `json:"risk"`
	DataClassification  string            `json:"dataClassification"`
	Approval            string            `json:"approval"`
	Availability        ScopeAvailability `json:"availability"`
	Resources           []string          `json:"resources"`
	Endpoints           []ScopeEndpoint   `json:"endpoints"`
	RequiredForWebhooks []string          `json:"requiredForWebhooks"`
}

type AppCredential struct {
	ID                   string           `json:"id"`
	AppID                string           `json:"appId"`
	Environment          Environment      `json:"environment"`
	ClientID             string           `json:"clientId"`
	SecretFingerprint    string           `json:"secretFingerprint"`
	Status               CredentialStatus `json:"status"`
	LastUsedAt           *time.Time       `json:"lastUsedAt"`
	ExpiresAt            *time.Time       `json:"expiresAt"`
	CreatedBy            string           `json:"createdBy"`
	CreatedAt            time.Time        `json:"createdAt"`
	RotatedAt            *time.Time       `json:"rotatedAt"`
	RevokedAt            *time.Time       `json:"revokedAt"`
	SecretCiphertext     []byte           `json:"-"`
	EncryptionKeyVersion int              `json:"-"`
}

type WebhookSubscription struct {
	ID                       string        `json:"id"`
	AppID                    string        `json:"appId"`
	Event                    string        `json:"event"`
	EndpointURL              string        `json:"endpointUrl"`
	Status                   WebhookStatus `json:"status"`
	SigningSecretFingerprint string        `json:"signingSecretFingerprint"`
	CreatedAt                time.Time     `json:"createdAt"`
	UpdatedAt                time.Time     `json:"updatedAt"`
	Revision                 int64         `json:"revision"`
	SigningSecretCiphertext  []byte        `json:"-"`
	EncryptionKeyVersion     int           `json:"-"`
}

type WebhookEventDefinition struct {
	Event         string                   `json:"event"`
	Name          string                   `json:"name"`
	Description   string                   `json:"description"`
	Source        WebhookEventSource       `json:"source"`
	Availability  WebhookEventAvailability `json:"availability"`
	RequiredScope *string                  `json:"requiredScope"`
	APIVersion    string                   `json:"apiVersion"`
}

type AppInstallation struct {
	ID                 string             `json:"id"`
	AppID              string             `json:"appId"`
	MerchantID         string             `json:"merchantId"`
	MerchantName       string             `json:"merchantName"`
	MerchantDomain     *string            `json:"merchantDomain"`
	Environment        Environment        `json:"environment"`
	Status             InstallationStatus `json:"status"`
	InstalledVersionID string             `json:"installedVersionId"`
	GrantedScopes      []string           `json:"grantedScopes"`
	InstalledBy        string             `json:"installedBy"`
	InstalledAt        time.Time          `json:"installedAt"`
	CreatedAt          time.Time          `json:"createdAt"`
	UpdatedAt          time.Time          `json:"updatedAt"`
	UninstalledAt      *time.Time         `json:"uninstalledAt"`
	Revision           int64              `json:"revision"`
}

type DevelopmentInstallRequest struct {
	ID              string                          `json:"id"`
	AppID           string                          `json:"appId"`
	MerchantID      string                          `json:"merchantId"`
	MerchantName    string                          `json:"merchantName"`
	MerchantDomain  *string                         `json:"merchantDomain"`
	Environment     Environment                     `json:"-"`
	VersionID       string                          `json:"versionId"`
	Status          DevelopmentInstallRequestStatus `json:"status"`
	LaunchURL       string                          `json:"launchUrl"`
	RequestedBy     string                          `json:"requestedBy"`
	CreatedAt       time.Time                       `json:"createdAt"`
	UpdatedAt       time.Time                       `json:"updatedAt"`
	ExpiresAt       time.Time                       `json:"expiresAt"`
	AuthorizedAt    *time.Time                      `json:"authorizedAt"`
	AuthorizationID *string                         `json:"authorizationId"`
}

type OAuthClient struct {
	OrganizationID string        `json:"-"`
	Credential     AppCredential `json:"-"`
}

type OAuthAuthorization struct {
	ID                   string      `json:"id"`
	OrganizationID       string      `json:"-"`
	AppID                string      `json:"appId"`
	CredentialID         string      `json:"-"`
	ClientID             string      `json:"clientId"`
	CodeHash             string      `json:"-"`
	RedirectURI          string      `json:"redirectUri"`
	CodeChallenge        string      `json:"-"`
	MerchantID           string      `json:"merchantId"`
	MerchantName         string      `json:"merchantName"`
	MerchantDomain       *string     `json:"merchantDomain"`
	Environment          Environment `json:"environment"`
	InstalledVersionID   string      `json:"installedVersionId"`
	GrantedScopes        []string    `json:"grantedScopes"`
	ApprovedBy           string      `json:"approvedBy"`
	CreatedAt            time.Time   `json:"createdAt"`
	ExpiresAt            time.Time   `json:"expiresAt"`
	ConsumedAt           *time.Time  `json:"consumedAt"`
	TestInstallRequestID *string     `json:"-"`
}

type OAuthAccessToken struct {
	ID             string     `json:"id"`
	AppID          string     `json:"appId"`
	InstallationID string     `json:"installationId"`
	CredentialID   string     `json:"-"`
	TokenHash      string     `json:"-"`
	Scopes         []string   `json:"scopes"`
	CreatedAt      time.Time  `json:"createdAt"`
	ExpiresAt      time.Time  `json:"expiresAt"`
	RevokedAt      *time.Time `json:"revokedAt"`
}

type InstallationAccessContext struct {
	OrganizationID     string             `json:"organizationId"`
	AppID              string             `json:"appId"`
	AppName            string             `json:"appName"`
	AppSlug            string             `json:"appSlug"`
	InstallationID     string             `json:"installationId"`
	MerchantID         string             `json:"merchantId"`
	MerchantName       string             `json:"merchantName"`
	MerchantDomain     *string            `json:"merchantDomain"`
	Environment        Environment        `json:"environment"`
	InstallationStatus InstallationStatus `json:"installationStatus"`
	InstalledVersionID string             `json:"installedVersionId"`
	Scopes             []string           `json:"scopes"`
	TokenExpiresAt     time.Time          `json:"tokenExpiresAt"`
}

type MerchantProfile struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	Domain         *string     `json:"domain"`
	Environment    Environment `json:"environment"`
	InstallationID string      `json:"installationId"`
}

type MerchantInstalledApp struct {
	OrganizationID string             `json:"-"`
	InstallationID string             `json:"installationId"`
	AppID          string             `json:"appId"`
	AppName        string             `json:"appName"`
	AppDescription *string            `json:"appDescription"`
	AppURL         *string            `json:"appUrl"`
	Version        string             `json:"version"`
	Status         InstallationStatus `json:"status"`
	GrantedScopes  []string           `json:"grantedScopes"`
	InstalledAt    time.Time          `json:"installedAt"`
	UpdatedAt      time.Time          `json:"updatedAt"`
}

type WebhookEvent struct {
	ID             string             `json:"id"`
	AppID          string             `json:"appId"`
	Event          string             `json:"event"`
	Source         WebhookEventSource `json:"source"`
	MerchantID     *string            `json:"merchantId,omitempty"`
	InstallationID *string            `json:"installationId,omitempty"`
	Payload        map[string]any     `json:"-"`
	CreatedAt      time.Time          `json:"createdAt"`
}

type WebhookDelivery struct {
	ID                string                `json:"id"`
	SubscriptionID    string                `json:"subscriptionId"`
	EventID           string                `json:"eventId"`
	Event             string                `json:"event"`
	EventSource       WebhookEventSource    `json:"-"`
	MerchantID        *string               `json:"-"`
	InstallationID    *string               `json:"-"`
	EventCreatedAt    time.Time             `json:"-"`
	Attempt           int                   `json:"attempt"`
	Status            WebhookDeliveryStatus `json:"status"`
	ResponseStatus    *int                  `json:"responseStatus"`
	ResponseTimeMS    *int64                `json:"responseTimeMs"`
	ErrorCode         *string               `json:"errorCode"`
	AttemptedAt       time.Time             `json:"attemptedAt"`
	NextAttemptAt     *time.Time            `json:"nextAttemptAt"`
	CompletedAt       *time.Time            `json:"completedAt"`
	EndpointURL       string                `json:"-"`
	Payload           map[string]any        `json:"-"`
	SigningCiphertext []byte                `json:"-"`
	EncryptionVersion int                   `json:"-"`
}

type AuditEvent struct {
	ID             string         `json:"id"`
	OrganizationID string         `json:"organizationId"`
	ActorID        string         `json:"actorId"`
	Action         string         `json:"action"`
	ResourceType   string         `json:"resourceType"`
	ResourceID     string         `json:"resourceId"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"createdAt"`
}
