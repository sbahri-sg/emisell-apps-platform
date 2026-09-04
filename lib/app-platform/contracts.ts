import type {
  App,
  AppCredential,
  AppExtension,
  AppVersion,
  Distribution,
  EntityId,
  EmisellMerchantId,
  Environment,
  ExtensionType,
  Installation,
	DevelopmentInstallRequest,
  InstallationStatus,
  ScopeAccess,
  WebhookSubscription,
  WebhookEventDefinition,
  WebhookDelivery,
  VersionedScope,
  ScopeDefinition,
  DeveloperApplication,
  DeveloperAppType,
  DeveloperInvitation,
  OrganizationEntitlement,
  SessionActor,
  OrganizationMembership,
  DeveloperOrganization,
  DeveloperOrganizationDetail,
} from "./domain";

export interface ApiError {
  code: string;
  message: string;
  requestId: string;
  details?: Record<string, unknown>;
}

export interface ApiResponse<T> {
  data: T;
  meta?: Record<string, unknown>;
}

export interface CursorPage<T> {
  data: T[];
  meta: {
    nextCursor: string | null;
    hasMore: boolean;
  };
}

export interface ListAppsQuery {
  cursor?: string;
  limit?: number;
  search?: string;
  status?: string;
  type?: string;
}

export interface CreateAppRequest {
  name: string;
  description?: string;
  distribution: Distribution;
  appUrl?: string;
  contactEmail?: string;
}

export interface UpdateAppRequest {
  name?: string;
  description?: string | null;
  appUrl?: string | null;
  contactEmail?: string | null;
  distribution?: Distribution;
  revision: number;
}

export interface CreateVersionRequest {
  version: string;
  releaseNote?: string;
}

export interface ReleaseVersionRequest {
  expectedActiveVersionId?: EntityId | null;
}

export interface CreateExtensionRequest {
  name: string;
  type: ExtensionType;
  runtimeUrl?: string;
  configuration?: Record<string, unknown>;
}

export interface UpdateExtensionRequest {
  name?: string;
  runtimeUrl?: string | null;
  configuration?: Record<string, unknown>;
  revision: number;
}

export interface ReplaceScopesRequest {
  scopes: Array<{ scope: string; access: ScopeAccess }>;
}

export interface CreateCredentialRequest {
  environment: Environment;
  expiresAt?: string;
}

export interface CredentialSecretResponse {
  credential: AppCredential;
  clientSecret: string;
}

export interface WebhookSecretResponse {
  subscription: WebhookSubscription;
  signingSecret: string;
}

export interface CreateWebhookRequest {
  event: string;
  endpointUrl: string;
}

export interface UpdateWebhookRequest {
  endpointUrl?: string;
  status?: "active" | "paused";
  revision: number;
}

export interface PublishWebhookEventRequest {
  event: string;
  payload: Record<string, unknown>;
}

export interface PublishWebhookEventResponse {
  event: {
    id: EntityId;
    appId: EntityId;
    event: string;
    source: "test";
    merchantId?: EmisellMerchantId;
    installationId?: EntityId;
    createdAt: string;
  };
  deliveries: WebhookDelivery[];
}

export interface CreateInstallationRequest {
  merchantId: EmisellMerchantId;
  merchantName: string;
  merchantDomain?: string;
  environment: Environment;
  grantedScopes?: string[];
}

export interface CreateDevelopmentInstallRequest {
  merchantId: EmisellMerchantId;
}

export interface UpdateInstallationRequest {
  status: "active" | "suspended";
  revision: number;
}

export interface UpgradeInstallationRequest {
  targetVersionId: EntityId;
  expectedInstalledVersionId: EntityId;
  revision: number;
}

export interface AuthorizeOAuthRequest {
  clientId: string;
  redirectUri: string;
  state: string;
  codeChallenge: string;
  merchantId: EmisellMerchantId;
  merchantName: string;
  merchantDomain?: string;
  environment: "sandbox";
  grantedScopes: string[];
}

export interface OAuthAuthorizationResult {
  authorization: {
    id: EntityId;
    appId: EntityId;
    clientId: string;
    redirectUri: string;
    merchantId: EmisellMerchantId;
    merchantName: string;
    merchantDomain: string | null;
    environment: "sandbox";
    installedVersionId: EntityId;
    grantedScopes: string[];
    approvedBy: EntityId;
    createdAt: string;
    expiresAt: string;
    consumedAt: string | null;
  };
  code: string;
  redirectTo: string;
}

export interface ExchangeOAuthTokenRequest {
  clientId: string;
  clientSecret: string;
  code: string;
  redirectUri: string;
  codeVerifier: string;
}

export interface OAuthTokenResult {
  access_token: string;
  token_type: "Bearer";
  expires_in: number;
  scope: string;
  installation: Installation;
}

export interface InstallationAccessContext {
  organizationId: EntityId;
  appId: EntityId;
  appName: string;
  appSlug: string;
  installationId: EntityId;
  merchantId: EmisellMerchantId;
  merchantName: string;
  merchantDomain: string | null;
  environment: Environment;
  installationStatus: InstallationStatus;
  installedVersionId: EntityId;
  scopes: string[];
  tokenExpiresAt: string;
}

export interface MerchantProfile {
  id: EmisellMerchantId;
  name: string;
  domain: string | null;
  environment: Environment;
  installationId: EntityId;
}

export interface MerchantIdentity {
  merchantId: EmisellMerchantId;
  name: string;
  domain: string | null;
  environment: Environment;
  createdAt: string;
  updatedAt: string;
}

export interface MerchantSession {
  merchant: MerchantIdentity;
  sessionExpiresAt: string;
  authenticationMethod: "merchant_session";
}

export type CatalogCategory =
  | "payment"
  | "shipping"
  | "erp"
  | "marketing"
  | "operations"
  | "custom";

export interface CatalogApp {
  appId: EntityId;
  slug: string;
  name: string;
  description: string | null;
  developerName: string;
  category: CatalogCategory;
  featured: boolean;
  launchUrl: string;
  activeVersionId: EntityId;
  version: string;
  extensionTypes: ExtensionType[];
  requiredScopes: VersionedScope[];
  optionalScopes: VersionedScope[];
  publishedAt: string;
  listingUpdatedAt: string;
}

export interface AppCatalogListing {
  appId: EntityId;
  organizationId: EntityId;
  category: CatalogCategory;
  status: "draft" | "published" | "hidden";
  featured: boolean;
  publishedBy: EntityId | null;
  publishedAt: string | null;
  updatedAt: string;
  revision: number;
}

export interface CatalogCandidate {
  organizationId: EntityId;
  developerName: string;
  app: App;
  activeVersion: AppVersion | null;
  listing: AppCatalogListing | null;
  eligible: boolean;
  blockingReason: string | null;
}

export interface SandboxMerchantLoginRequest {
  merchantId: EmisellMerchantId;
  merchantName: string;
  merchantDomain?: string;
}

export interface PreviewMerchantOAuthRequest {
  clientId: string;
  redirectUri: string;
  state: string;
  codeChallenge: string;
  requestedScopes: string[];
  testInstallRequestId?: EntityId;
}

export interface AuthorizeMerchantOAuthRequest extends PreviewMerchantOAuthRequest {
  grantedScopes: string[];
}

export interface MerchantOAuthConsent {
  app: {
    id: EntityId;
    name: string;
    description: string | null;
    appUrl: string | null;
  };
  version: { id: EntityId; version: string };
  merchant: MerchantIdentity;
  redirectUri: string;
  requiredScopes: VersionedScope[];
  optionalScopes: VersionedScope[];
  requestedScopes: string[];
  developmentInstall: boolean;
}

export interface MerchantInstalledApp {
  installationId: EntityId;
  appId: EntityId;
  appName: string;
  appDescription: string | null;
  appUrl: string | null;
  version: string;
  status: "active" | "suspended";
  grantedScopes: string[];
  installedAt: string;
  updatedAt: string;
}

export interface CreateDeveloperApplicationRequest {
  companyName: string;
  companyDomain: string;
  contactName: string;
  contactEmail: string;
  requestedAppName: string;
  appType: DeveloperAppType;
  useCase: string;
  requestedScopes: string[];
}

export interface ReviewDeveloperApplicationRequest {
  revision: number;
  notes?: string;
}

export interface ApproveDeveloperApplicationRequest {
  revision: number;
  maxApps?: number;
  maxWebhooks?: number;
}

export interface RejectDeveloperApplicationRequest {
  revision: number;
  notes: string;
}

export interface DeveloperApprovalResult {
  application: DeveloperApplication;
  invitation: DeveloperInvitation;
  invitationToken: string;
}

export interface InvitationAcceptanceResult {
  application: DeveloperApplication;
  entitlement: OrganizationEntitlement;
}

export type AppResponse = ApiResponse<App>;
export type AppListResponse = CursorPage<App>;
export type VersionResponse = ApiResponse<AppVersion>;
export type VersionListResponse = CursorPage<AppVersion>;
export type ExtensionResponse = ApiResponse<AppExtension>;
export type ExtensionListResponse = ApiResponse<AppExtension[]>;
export type ScopeListResponse = ApiResponse<VersionedScope[]>;
export type ScopeCatalogResponse = ApiResponse<ScopeDefinition[]>;
export type WebhookEventCatalogResponse = ApiResponse<WebhookEventDefinition[]>;
export type CredentialListResponse = ApiResponse<AppCredential[]>;
export type CredentialSecretEnvelope = ApiResponse<CredentialSecretResponse>;
export type WebhookResponse = ApiResponse<WebhookSubscription>;
export type WebhookListResponse = CursorPage<WebhookSubscription>;
export type WebhookSecretEnvelope = ApiResponse<WebhookSecretResponse>;
export type WebhookDeliveryListResponse = CursorPage<WebhookDelivery>;
export type PublishWebhookEventEnvelope =
  ApiResponse<PublishWebhookEventResponse>;
export type InstallationListResponse = CursorPage<Installation>;
export type InstallationResponse = ApiResponse<Installation>;
export type DevelopmentInstallRequestListResponse = ApiResponse<
  DevelopmentInstallRequest[]
>;
export type DevelopmentInstallRequestResponse =
  ApiResponse<DevelopmentInstallRequest>;
export type InstallationAccessContextResponse =
  ApiResponse<InstallationAccessContext>;
export type MerchantProfileResponse = ApiResponse<MerchantProfile>;
export type MerchantSessionResponse = ApiResponse<MerchantSession>;
export type MerchantOAuthConsentResponse = ApiResponse<MerchantOAuthConsent>;
export type MerchantInstalledAppListResponse = ApiResponse<
  MerchantInstalledApp[]
>;
export type CatalogAppListResponse = CursorPage<CatalogApp>;
export type CatalogAppResponse = ApiResponse<CatalogApp>;
export type CatalogCandidateListResponse = CursorPage<CatalogCandidate>;
export type CatalogListingResponse = ApiResponse<AppCatalogListing>;
export type SessionResponse = ApiResponse<SessionActor>;
export type OrganizationMembershipListResponse = ApiResponse<
  OrganizationMembership[]
>;
export type DeveloperApplicationResponse = ApiResponse<DeveloperApplication>;
export type DeveloperApplicationListResponse = CursorPage<DeveloperApplication>;
export type DeveloperOrganizationListResponse =
  CursorPage<DeveloperOrganization>;
export type DeveloperOrganizationResponse =
  ApiResponse<DeveloperOrganizationDetail>;
export type DeveloperApprovalResponse = ApiResponse<DeveloperApprovalResult>;
export type DeveloperInvitationResponse = ApiResponse<DeveloperInvitation>;
export type InvitationAcceptanceResponse =
  ApiResponse<InvitationAcceptanceResult>;
