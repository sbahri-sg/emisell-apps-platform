/**
 * Canonical domain vocabulary for the Emisell App Platform control plane.
 * These types are persistence-agnostic and safe to share with API clients.
 */

export type EntityId = string;
export type EmisellMerchantId = string;
export type IsoDateTime = string;
export type SemVer = string;

export type Environment = 'sandbox' | 'production';
export type Distribution = 'public' | 'custom';
export type AppStatus = 'draft' | 'active' | 'archived';
export type AppReleaseStatus = 'development' | 'released';
export type VersionStatus = 'draft' | 'released' | 'active';
export type ExtensionType = 'payment' | 'shipping' | 'custom';
export type ExtensionStatus = 'draft' | 'active' | 'disabled';
export type CredentialStatus = 'active' | 'revoked' | 'expired';
export type WebhookStatus = 'pending' | 'active' | 'paused' | 'failing';
export type WebhookDeliveryStatus = 'pending' | 'delivered' | 'failed';
export type InstallationStatus = 'pending' | 'active' | 'suspended' | 'uninstalled';
export type OrganizationRole = 'owner' | 'admin' | 'developer' | 'analyst';
export type ScopeAccess = 'required' | 'optional';
export type DeveloperApplicationStatus = 'submitted' | 'under_review' | 'approved' | 'invited' | 'active' | 'rejected';
export type DeveloperAppType = 'payment' | 'shipping' | 'erp' | 'marketing' | 'custom';
export type DeveloperInvitationStatus = 'pending' | 'accepted' | 'revoked' | 'expired';

export interface Organization {
  id: EntityId;
  name: string;
  slug: string;
  createdAt: IsoDateTime;
  updatedAt: IsoDateTime;
}

export interface User {
  id: EntityId;
  email: string;
  displayName: string;
  createdAt: IsoDateTime;
}

export interface OrganizationMembership {
  id: EntityId;
  organizationId: EntityId;
  userId: EntityId;
  role: OrganizationRole;
  createdAt: IsoDateTime;
  updatedAt: IsoDateTime;
}

export interface App {
  id: EntityId;
  organizationId: EntityId;
  name: string;
  slug: string;
  description: string | null;
  distribution: Distribution;
  status: AppStatus;
  releaseStatus: AppReleaseStatus;
  appUrl: string | null;
  contactEmail: string | null;
  activeVersionId: EntityId | null;
  createdBy: EntityId;
  createdAt: IsoDateTime;
  updatedAt: IsoDateTime;
  revision: number;
}

export interface AppVersion {
  id: EntityId;
  appId: EntityId;
  version: SemVer;
  status: VersionStatus;
  releaseNote: string | null;
  snapshot: AppVersionSnapshot;
  createdBy: EntityId;
  releasedBy: EntityId | null;
  createdAt: IsoDateTime;
  releasedAt: IsoDateTime | null;
}

export interface AppVersionSnapshot {
  extensions: VersionedExtension[];
  scopes: VersionedScope[];
  webhookSubscriptions: VersionedWebhookSubscription[];
  redirectUrls: string[];
  configurationHash: string;
}

export interface AppExtension {
  id: EntityId;
  appId: EntityId;
  name: string;
  type: ExtensionType;
  status: ExtensionStatus;
  runtimeUrl: string | null;
  configuration: Record<string, unknown>;
  createdAt: IsoDateTime;
  updatedAt: IsoDateTime;
  revision: number;
}

export interface VersionedExtension {
  extensionId: EntityId;
  name: string;
  type: ExtensionType;
  runtimeUrl: string | null;
  configuration: Record<string, unknown>;
}

export interface ScopeDefinition {
  scope: string;
  name: string;
  description: string;
  access: "read" | "write";
  risk: "low" | "standard" | "sensitive" | "high";
  dataClassification: string;
  approval: "merchant_consent" | "merchant_consent_and_emisell_review";
  availability: "available" | "planned";
  resources: string[];
  endpoints: Array<{ method: string; path: string }>;
  requiredForWebhooks: string[];
}

export interface AppScope {
  appId: EntityId;
  scope: string;
  access: ScopeAccess;
  createdAt: IsoDateTime;
}

export interface VersionedScope {
  scope: string;
  access: ScopeAccess;
}

export interface AppCredential {
  id: EntityId;
  appId: EntityId;
  environment: Environment;
  clientId: string;
  secretFingerprint: string;
  status: CredentialStatus;
  lastUsedAt: IsoDateTime | null;
  expiresAt: IsoDateTime | null;
  createdBy: EntityId;
  createdAt: IsoDateTime;
  rotatedAt: IsoDateTime | null;
  revokedAt: IsoDateTime | null;
}

export interface WebhookSubscription {
  id: EntityId;
  appId: EntityId;
  event: string;
  endpointUrl: string;
  status: WebhookStatus;
  signingSecretFingerprint: string;
  createdAt: IsoDateTime;
  updatedAt: IsoDateTime;
  revision: number;
}

export interface WebhookEventDefinition {
  event: string;
  name: string;
  description: string;
  source: "app_platform" | "emisell_backend";
  availability: "available" | "planned";
  requiredScope: string | null;
  apiVersion: string;
}

export interface VersionedWebhookSubscription {
  subscriptionId: EntityId;
  event: string;
  endpointUrl: string;
}

export interface WebhookDelivery {
  id: EntityId;
  subscriptionId: EntityId;
  eventId: EntityId;
	 event: string;
  attempt: number;
	 status: WebhookDeliveryStatus;
  responseStatus: number | null;
  responseTimeMs: number | null;
	 errorCode: string | null;
  nextAttemptAt: IsoDateTime | null;
	 completedAt: IsoDateTime | null;
	 attemptedAt: IsoDateTime;
}

export interface Installation {
  id: EntityId;
  appId: EntityId;
  merchantId: EmisellMerchantId;
  merchantName: string;
  merchantDomain: string | null;
  environment: Environment;
  status: InstallationStatus;
  installedVersionId: EntityId;
  grantedScopes: string[];
  installedBy: EntityId;
  installedAt: IsoDateTime;
  createdAt: IsoDateTime;
  updatedAt: IsoDateTime;
  uninstalledAt: IsoDateTime | null;
  revision: number;
}

export interface DevelopmentInstallRequest {
  id: EntityId;
  appId: EntityId;
  merchantId: EmisellMerchantId;
  merchantName: string;
  merchantDomain: string | null;
  versionId: EntityId;
  status: "pending" | "authorized" | "cancelled" | "expired";
  launchUrl: string;
  requestedBy: EntityId;
  createdAt: IsoDateTime;
  updatedAt: IsoDateTime;
  expiresAt: IsoDateTime;
  authorizedAt: IsoDateTime | null;
  authorizationId: EntityId | null;
}

export interface AuditEvent {
  id: EntityId;
  organizationId: EntityId;
  actorId: EntityId;
  action: string;
  resourceType: string;
  resourceId: EntityId;
  metadata: Record<string, unknown>;
  createdAt: IsoDateTime;
}

export interface SessionActor {
  userId: EntityId;
  organizationId: EntityId | '';
  role: OrganizationRole | '';
  email: string;
  displayName: string;
  platformOperator: boolean;
  authenticationMethod: 'session' | 'bearer';
  sessionExpiresAt: IsoDateTime | null;
  activeOrganization: OrganizationMembership | null;
}

export interface OrganizationMembership {
  organizationId: EntityId;
  name: string;
  slug: string;
  status: 'active' | 'suspended';
  role: OrganizationRole;
}

export interface DeveloperInvitation {
  id: EntityId;
  applicationId: EntityId;
  organizationId: EntityId;
  email: string;
  role: 'owner';
  status: DeveloperInvitationStatus;
  createdBy: EntityId;
  acceptedBy: EntityId | null;
  createdAt: IsoDateTime;
  expiresAt: IsoDateTime;
  acceptedAt: IsoDateTime | null;
  revokedAt: IsoDateTime | null;
}

export interface DeveloperApplication {
  id: EntityId;
  companyName: string;
  companyDomain: string;
  contactName: string;
  contactEmail: string;
  requestedAppName: string;
  appType: DeveloperAppType;
  useCase: string;
  requestedScopes: string[];
  status: DeveloperApplicationStatus;
  reviewNotes: string | null;
  organizationId: EntityId | null;
  submittedBy: EntityId;
  reviewedBy: EntityId | null;
  reviewedAt: IsoDateTime | null;
  createdAt: IsoDateTime;
  updatedAt: IsoDateTime;
  revision: number;
  currentInvitation?: DeveloperInvitation;
}

export interface OrganizationEntitlement {
  organizationId: EntityId;
  sandboxAccess: boolean;
  productionAccess: boolean;
  maxApps: number;
  maxWebhooks: number;
  createdAt: IsoDateTime;
  updatedAt: IsoDateTime;
}

export type OrganizationStatus = 'active' | 'suspended';

export interface DeveloperOrganization {
  id: EntityId;
  name: string;
  slug: string;
  status: OrganizationStatus;
  entitlement: OrganizationEntitlement;
  appCount: number;
  membershipCount: number;
  createdAt: IsoDateTime;
  updatedAt: IsoDateTime;
}

export interface DeveloperOrganizationMember {
  userId: EntityId;
  email: string;
  displayName: string;
  role: OrganizationRole;
  createdAt: IsoDateTime;
}

export interface DeveloperOrganizationDetail extends DeveloperOrganization {
  memberships: DeveloperOrganizationMember[];
}

export const roleCapabilities: Record<OrganizationRole, readonly string[]> = {
  owner: ['organization.manage', 'team.manage', 'app.manage', 'release.manage', 'credential.manage', 'analytics.read'],
  admin: ['team.manage', 'app.manage', 'release.manage', 'credential.manage', 'analytics.read'],
  developer: ['app.read', 'app.write', 'release.create', 'analytics.read'],
  analyst: ['app.read', 'analytics.read'],
};
