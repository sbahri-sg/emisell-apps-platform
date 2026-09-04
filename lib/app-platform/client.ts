import type {
  ApiError,
  ApiResponse,
  AppListResponse,
  CreateAppRequest,
  CreateExtensionRequest,
  CreateCredentialRequest,
  CreateWebhookRequest,
  PublishWebhookEventRequest,
  PublishWebhookEventEnvelope,
  CreateInstallationRequest,
	CreateDevelopmentInstallRequest,
	DevelopmentInstallRequestListResponse,
	DevelopmentInstallRequestResponse,
  CreateVersionRequest,
  ListAppsQuery,
  ReleaseVersionRequest,
  ReplaceScopesRequest,
  ScopeListResponse,
  ScopeCatalogResponse,
  WebhookEventCatalogResponse,
  ExtensionListResponse,
  ExtensionResponse,
  CredentialListResponse,
  CredentialSecretEnvelope,
  UpdateWebhookRequest,
  WebhookListResponse,
  WebhookResponse,
  WebhookSecretEnvelope,
  WebhookDeliveryListResponse,
  InstallationListResponse,
  InstallationResponse,
  UpdateInstallationRequest,
  UpgradeInstallationRequest,
  AuthorizeOAuthRequest,
  OAuthAuthorizationResult,
  ExchangeOAuthTokenRequest,
  OAuthTokenResult,
  InstallationAccessContextResponse,
  MerchantProfileResponse,
  MerchantSessionResponse,
  MerchantOAuthConsentResponse,
  MerchantInstalledAppListResponse,
  SandboxMerchantLoginRequest,
  PreviewMerchantOAuthRequest,
  AuthorizeMerchantOAuthRequest,
  SessionResponse,
  OrganizationMembershipListResponse,
  DeveloperApplicationListResponse,
  DeveloperApplicationResponse,
  DeveloperOrganizationListResponse,
  DeveloperOrganizationResponse,
  CreateDeveloperApplicationRequest,
  ReviewDeveloperApplicationRequest,
  ApproveDeveloperApplicationRequest,
  RejectDeveloperApplicationRequest,
  DeveloperApprovalResponse,
  DeveloperInvitationResponse,
  InvitationAcceptanceResponse,
  UpdateAppRequest,
  UpdateExtensionRequest,
  VersionListResponse,
  VersionResponse,
  CatalogAppListResponse,
  CatalogCandidateListResponse,
  CatalogListingResponse,
  CatalogCategory,
} from "./contracts";
import type { App, AppVersion, OrganizationMembership } from "./domain";
import type { IntegrationReadiness } from "./integration";
import type { ContractId, DocumentationReference } from '../documentation/types';
import type { ExtensionCatalog } from './extension-catalog';
import type { DeveloperContractId, DeveloperGuide } from '../documentation/developer-types';
import type { AppPlan, AppSubscription, AppSubscriptionQuote, CreateAppPlan, InstallationBilling } from './billing';

const DEFAULT_GATEWAY_URL = "http://localhost:8081";
const DEFAULT_DEVELOPMENT_TOKEN = "emisell-local-dev-token";
const DEFAULT_DEVELOPMENT_ORGANIZATION_ID =
  "01995f72-0000-7000-8000-000000000001";

type ErrorEnvelope = { error?: ApiError };

export class AppPlatformApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly requestId?: string;
  readonly details?: Record<string, unknown>;

  constructor(
    message: string,
    options: {
      status: number;
      code?: string;
      requestId?: string;
      details?: Record<string, unknown>;
    },
  ) {
    super(message);
    this.name = "AppPlatformApiError";
    this.status = options.status;
    this.code = options.code ?? "request_failed";
    this.requestId = options.requestId;
    this.details = options.details;
  }
}

function createIdempotencyKey(operation: string) {
  const suffix =
    globalThis.crypto?.randomUUID?.() ??
    `${Date.now()}-${Math.random().toString(36).slice(2)}`;
  return `${operation}-${suffix}`;
}

function queryString(query: ListAppsQuery = {}) {
  const params = new URLSearchParams();
  if (query.cursor) params.set("cursor", query.cursor);
  if (query.limit) params.set("limit", String(query.limit));
  if (query.search) params.set("search", query.search);
  if (query.status) params.set("status", query.status);
  if (query.type) params.set("type", query.type);
  const value = params.toString();
  return value ? `?${value}` : "";
}

class AppPlatformClient {
  async listAppPlans(appId: string, signal?: AbortSignal) {
    return (await this.request<ApiResponse<AppPlan[]>>(`/v1/apps/${appId}/plans`, { signal })).data;
  }

  async createAppPlan(appId: string, input: CreateAppPlan, key: string) {
    return (await this.request<ApiResponse<AppPlan>>(`/v1/apps/${appId}/plans`, { method: 'POST', body: JSON.stringify(input), headers: { 'Idempotency-Key': key } })).data;
  }

  archiveAppPlan(appId: string, planId: string) {
    return this.request<void>(`/v1/apps/${appId}/plans/${planId}`, { method: 'DELETE' });
  }

  async getMerchantInstallationBilling(installationId: string, signal?: AbortSignal) {
    return (await this.merchantRequest<ApiResponse<InstallationBilling>>(`/v1/merchant/installations/${installationId}/billing`, { signal })).data;
  }

  async quoteAppSubscription(installationId: string, planId: string) {
    return (await this.merchantRequest<ApiResponse<AppSubscriptionQuote>>(`/v1/merchant/installations/${installationId}/billing/quotes`, { method: 'POST', body: JSON.stringify({ planId }) })).data;
  }

  async approveAppSubscription(installationId: string, quoteId: string) {
    return (await this.merchantRequest<ApiResponse<AppSubscription>>(`/v1/merchant/installations/${installationId}/billing/approve`, { method: 'POST', body: JSON.stringify({ quoteId, acceptRecurringCharge: true }) })).data;
  }

  cancelAppSubscription(installationId: string, subscriptionId: string) {
    return this.merchantRequest<void>(`/v1/merchant/installations/${installationId}/billing/subscriptions/${subscriptionId}`, { method: 'DELETE' });
  }
  private readonly baseUrl = (
    process.env.NEXT_PUBLIC_APP_GATEWAY_URL ?? DEFAULT_GATEWAY_URL
  ).replace(/\/$/, "");
  private readonly token =
    process.env.NEXT_PUBLIC_APP_GATEWAY_TOKEN ?? DEFAULT_DEVELOPMENT_TOKEN;
  private readonly organizationId =
    process.env.NEXT_PUBLIC_ORGANIZATION_ID ??
    DEFAULT_DEVELOPMENT_ORGANIZATION_ID;

  private csrfToken() {
    if (typeof document === "undefined") return "";
    const cookieName = `${process.env.NEXT_PUBLIC_AUTH_CSRF_COOKIE_NAME ?? "emisell_csrf"}=`;
    return (
      document.cookie
        .split(";")
        .map((part) => part.trim())
        .find((part) => part.startsWith(cookieName))
        ?.slice(cookieName.length) ?? ""
    );
  }

  private merchantCSRFToken() {
    if (typeof document === "undefined") return "";
    const cookieName = `${process.env.NEXT_PUBLIC_MERCHANT_CSRF_COOKIE_NAME ?? "emisell_merchant_csrf"}=`;
    return (
      document.cookie
        .split(";")
        .map((part) => part.trim())
        .find((part) => part.startsWith(cookieName))
        ?.slice(cookieName.length) ?? ""
    );
  }

  private async merchantRequest<T>(
    path: string,
    init: RequestInit = {},
  ): Promise<T> {
    const headers = new Headers(init.headers);
    headers.set("Accept", "application/json");
    if (init.body) headers.set("Content-Type", "application/json");
    if (init.method && !["GET", "HEAD", "OPTIONS"].includes(init.method)) {
      const csrfToken = this.merchantCSRFToken();
      if (csrfToken) headers.set("X-CSRF-Token", decodeURIComponent(csrfToken));
    }
    const response = await fetch(`${this.baseUrl}${path}`, {
      ...init,
      credentials: "include",
      headers,
    });
    if (!response.ok) {
      let envelope: ErrorEnvelope = {};
      try {
        envelope = (await response.json()) as ErrorEnvelope;
      } catch {
        // Preserve a useful HTTP fallback when an intermediary returns non-JSON.
      }
      throw new AppPlatformApiError(
        envelope.error?.message ??
          `Merchant API returned HTTP ${response.status}.`,
        {
          status: response.status,
          code: envelope.error?.code,
          requestId: envelope.error?.requestId,
          details: envelope.error?.details,
        },
      );
    }
    if (response.status === 204) return undefined as T;
    return response.json() as Promise<T>;
  }

  private async request<T>(path: string, init: RequestInit = {}, baseUrl = this.baseUrl): Promise<T> {
    const adminRequest = path.startsWith('/auth/admin/') || path.startsWith('/v1/internal/') || path.startsWith('/admin/docs/');
    const csrfToken = adminRequest && typeof document !== 'undefined'
      ? document.cookie.split(';').map((part) => part.trim()).find((part) => part.startsWith('emisell_admin_csrf='))?.slice('emisell_admin_csrf='.length) ?? ''
      : adminRequest ? '' : this.csrfToken();
    const headers = new Headers(init.headers);
    headers.set("Accept", "application/json");
    if (init.body) headers.set("Content-Type", "application/json");
    if (csrfToken) {
      if (init.method && !["GET", "HEAD"].includes(init.method))
        headers.set("X-CSRF-Token", decodeURIComponent(csrfToken));
    } else if (!adminRequest) {
      headers.set("Authorization", `Bearer ${this.token}`);
      headers.set("X-Organization-Id", this.organizationId);
    }
    const response = await fetch(`${baseUrl}${path}`, {
      ...init,
      credentials: "include",
      headers,
    });

    if (!response.ok) {
      if (adminRequest && response.status === 401 && typeof window !== 'undefined' && window.location.pathname !== '/admin/login' && window.location.pathname.startsWith('/admin')) {
        window.location.replace(`/admin/login?reason=expired&next=${encodeURIComponent(window.location.pathname + window.location.search)}`);
      }
      let envelope: ErrorEnvelope = {};
      try {
        envelope = (await response.json()) as ErrorEnvelope;
      } catch {
        // Preserve a useful HTTP fallback when an intermediary returns non-JSON.
      }
      throw new AppPlatformApiError(
        envelope.error?.message ??
          `App Gateway returned HTTP ${response.status}.`,
        {
          status: response.status,
          code: envelope.error?.code,
          requestId: envelope.error?.requestId,
          details: envelope.error?.details,
        },
      );
    }

    if (response.status === 204) return undefined as T;
    return response.json() as Promise<T>;
  }

  async adminLogin(email: string, password: string) {
    await this.request('/auth/admin/login', { method: 'POST', body: JSON.stringify({ email, password }) });
  }

  async getAdminSession(signal?: AbortSignal) {
    return (await this.request<SessionResponse>('/auth/admin/session', { signal, cache: 'no-store' })).data;
  }

  async adminLogout() {
    await this.request<void>('/auth/admin/logout', { method: 'POST' });
  }

  async getSession(signal?: AbortSignal) {
    const response = await this.request<SessionResponse>("/v1/session", {
      signal,
    });
    return response.data;
  }

  async getAdminDocumentation(contract: ContractId, signal?: AbortSignal) {
    return this.request<DocumentationReference>(`/admin/docs/content?contract=${contract}`, { signal, cache: 'no-store' }, '');
  }

  getDeveloperGuide(signal?: AbortSignal) {
    return this.request<DeveloperGuide>('/docs/content', { signal, cache: 'no-store' }, '');
  }

  async getExtensionCatalog(signal?: AbortSignal) {
    return (await this.request<ApiResponse<ExtensionCatalog>>('/v1/extension-catalog', { signal, cache: 'no-store' })).data;
  }

  getDeveloperReference(contract: DeveloperContractId, signal?: AbortSignal) {
    return this.request<DocumentationReference>(`/docs/content?contract=${contract}&format=reference`, { signal, cache: 'no-store' }, '');
  }

  getDeveloperDocumentationDownload(contract: DeveloperContractId, format: 'openapi' | 'postman') {
    return this.request<unknown>(`/docs/content?contract=${contract}&format=${format}`, { cache: 'no-store' }, '');
  }

  async getAdminDocumentationDownload(contract: ContractId, format: 'openapi' | 'postman') {
    return this.request<unknown>(`/admin/docs/content?contract=${contract}&format=${format}`, { cache: 'no-store' }, '');
  }

  async developmentLogin() {
    await this.request<ApiResponse<{ status: string }>>(
      "/auth/development-login",
      { method: "POST" },
    );
  }

  async sandboxMerchantLogin(input: SandboxMerchantLoginRequest) {
    const response = await this.merchantRequest<MerchantSessionResponse>(
      "/auth/sandbox-merchant-login",
      { method: "POST", body: JSON.stringify(input) },
    );
    return response.data;
  }

  async getMerchantSession(signal?: AbortSignal) {
    const response = await this.merchantRequest<MerchantSessionResponse>(
      "/v1/merchant/session",
      { signal },
    );
    return response.data;
  }

  async logoutMerchantSession() {
    await this.merchantRequest<void>("/v1/merchant/session/logout", {
      method: "POST",
    });
  }

  async previewMerchantOAuth(input: PreviewMerchantOAuthRequest) {
    const response = await this.merchantRequest<MerchantOAuthConsentResponse>(
      "/v1/merchant/oauth/preview",
      { method: "POST", body: JSON.stringify(input) },
    );
    return response.data;
  }

  async authorizeMerchantOAuth(input: AuthorizeMerchantOAuthRequest) {
    const response = await this.merchantRequest<
      ApiResponse<OAuthAuthorizationResult>
    >("/v1/merchant/oauth/authorize", {
      method: "POST",
      body: JSON.stringify(input),
    });
    return response.data;
  }

  async listMerchantInstalledApps(signal?: AbortSignal) {
    const response =
      await this.merchantRequest<MerchantInstalledAppListResponse>(
        "/v1/merchant/installations",
        { signal },
      );
    return response.data;
  }

  async listCatalogApps(
    query: { search?: string; category?: CatalogCategory; featured?: boolean } = {},
    signal?: AbortSignal,
  ) {
    const params = new URLSearchParams({ limit: "100" });
    if (query.search) params.set("search", query.search);
    if (query.category) params.set("category", query.category);
    if (query.featured !== undefined)
      params.set("featured", String(query.featured));
    return this.merchantRequest<CatalogAppListResponse>(
      `/v1/catalog/apps?${params}`,
      { signal },
    );
  }

  listCatalogCandidates(
    query: { search?: string; status?: string; cursor?: string } = {},
    signal?: AbortSignal,
  ) {
    const params = new URLSearchParams({ limit: "100" });
    if (query.search) params.set("search", query.search);
    if (query.status) params.set("status", query.status);
    if (query.cursor) params.set("cursor", query.cursor);
    return this.request<CatalogCandidateListResponse>(
      `/v1/internal/catalog/apps?${params}`,
      { signal },
    );
  }

  async updateCatalogListing(
    organizationId: string,
    appId: string,
    input: {
      category: CatalogCategory;
      status: "draft" | "published" | "hidden";
      featured: boolean;
      revision: number;
      expectedAppRevision?: number;
      expectedActiveVersionId?: string;
    },
  ) {
    const response = await this.request<CatalogListingResponse>(
      `/v1/internal/organizations/${organizationId}/apps/${appId}/catalog-listing`,
      {
        method: "PUT",
        body: JSON.stringify(input),
      },
    );
    return response.data;
  }

  async getIntegrationReadiness(appId: string, organizationId?: string, signal?: AbortSignal) {
    const path = organizationId
      ? `/v1/internal/organizations/${encodeURIComponent(organizationId)}/apps/${encodeURIComponent(appId)}/integration-readiness`
      : `/v1/apps/${encodeURIComponent(appId)}/integration-readiness`;
    const response = await this.request<ApiResponse<IntegrationReadiness>>(path, { signal });
    return response.data;
  }

  async uninstallMerchantInstallation(installationId: string) {
    await this.merchantRequest<void>(
      `/v1/merchant/installations/${installationId}`,
      {
        method: "DELETE",
        headers: {
          "Idempotency-Key": createIdempotencyKey("merchant-uninstall"),
        },
      },
    );
  }

  async listOrganizations(signal?: AbortSignal) {
    const response = await this.request<OrganizationMembershipListResponse>(
      "/v1/session/organizations",
      { signal },
    );
    return response.data;
  }

  async switchOrganization(organizationId: string) {
    const response = await this.request<ApiResponse<OrganizationMembership>>(
      "/v1/session/organization",
      {
        method: "POST",
        body: JSON.stringify({ organizationId }),
      },
    );
    return response.data;
  }

  async logout() {
    await this.request<void>("/v1/session/logout", { method: "POST" });
  }

  loginURL(returnTo = "/overview") {
    return `${this.baseUrl}/auth/login?return_to=${encodeURIComponent(returnTo)}`;
  }

  listDeveloperApplications(
    query: { search?: string; status?: string } = {},
    signal?: AbortSignal,
  ) {
    const params = new URLSearchParams({ limit: "100" });
    if (query.search) params.set("search", query.search);
    if (query.status) params.set("status", query.status);
    return this.request<DeveloperApplicationListResponse>(
      `/v1/internal/developer-applications?${params}`,
      { signal },
    );
  }

  listDeveloperOrganizations(
    query: { search?: string; status?: string } = {},
    signal?: AbortSignal,
  ) {
    const params = new URLSearchParams({ limit: "100" });
    if (query.search) params.set("search", query.search);
    if (query.status) params.set("status", query.status);
    return this.request<DeveloperOrganizationListResponse>(
      `/v1/internal/organizations?${params}`,
      { signal },
    );
  }

  async getDeveloperOrganization(organizationId: string, signal?: AbortSignal) {
    const response = await this.request<DeveloperOrganizationResponse>(
      `/v1/internal/organizations/${organizationId}`,
      { signal },
    );
    return response.data;
  }

  async createDeveloperApplication(input: CreateDeveloperApplicationRequest) {
    const response = await this.request<DeveloperApplicationResponse>(
      "/v1/internal/developer-applications",
      {
        method: "POST",
        body: JSON.stringify(input),
      },
    );
    return response.data;
  }

  async reviewDeveloperApplication(
    applicationId: string,
    input: ReviewDeveloperApplicationRequest,
  ) {
    const response = await this.request<DeveloperApplicationResponse>(
      `/v1/internal/developer-applications/${applicationId}/review`,
      {
        method: "POST",
        body: JSON.stringify(input),
      },
    );
    return response.data;
  }

  async approveDeveloperApplication(
    applicationId: string,
    input: ApproveDeveloperApplicationRequest,
  ) {
    const response = await this.request<DeveloperApprovalResponse>(
      `/v1/internal/developer-applications/${applicationId}/approve`,
      {
        method: "POST",
        body: JSON.stringify(input),
      },
    );
    return response.data;
  }

  async rejectDeveloperApplication(
    applicationId: string,
    input: RejectDeveloperApplicationRequest,
  ) {
    const response = await this.request<DeveloperApplicationResponse>(
      `/v1/internal/developer-applications/${applicationId}/reject`,
      {
        method: "POST",
        body: JSON.stringify(input),
      },
    );
    return response.data;
  }

  async rotateDeveloperInvitation(applicationId: string, revision: number) {
    const response = await this.request<DeveloperApprovalResponse>(
      `/v1/internal/developer-applications/${applicationId}/invitations`,
      {
        method: "POST",
        body: JSON.stringify({ revision }),
      },
    );
    return response.data;
  }

  async revokeDeveloperInvitation(invitationId: string) {
    const response = await this.request<DeveloperInvitationResponse>(
      `/v1/internal/developer-invitations/${invitationId}/revoke`,
      {
        method: "POST",
      },
    );
    return response.data;
  }

  async acceptDeveloperInvitation(token: string) {
    const response = await this.request<InvitationAcceptanceResponse>(
      "/v1/developer-invitations/accept",
      {
        method: "POST",
        body: JSON.stringify({ token }),
      },
    );
    return response.data;
  }

  listApps(query: ListAppsQuery = {}, signal?: AbortSignal) {
    return this.request<AppListResponse>(`/v1/apps${queryString(query)}`, {
      signal,
    });
  }

  async listAllApps(signal?: AbortSignal) {
    const apps: App[] = [];
    let cursor: string | undefined;
    do {
      const response = await this.listApps({ cursor, limit: 100 }, signal);
      apps.push(...response.data);
      cursor = response.meta.nextCursor ?? undefined;
    } while (cursor);
    return apps;
  }

  async createApp(input: CreateAppRequest) {
    const response = await this.request<ApiResponse<App>>("/v1/apps", {
      method: "POST",
      body: JSON.stringify(input),
      headers: { "Idempotency-Key": createIdempotencyKey("create-app") },
    });
    return response.data;
  }

  async getApp(appId: string, signal?: AbortSignal) {
    const response = await this.request<ApiResponse<App>>(`/v1/apps/${appId}`, {
      signal,
    });
    return response.data;
  }

  async updateApp(appId: string, input: UpdateAppRequest) {
    const response = await this.request<ApiResponse<App>>(`/v1/apps/${appId}`, {
      method: "PATCH",
      body: JSON.stringify(input),
    });
    return response.data;
  }

  archiveApp(appId: string) {
    return this.request<void>(`/v1/apps/${appId}`, {
      method: "DELETE",
      headers: { "Idempotency-Key": createIdempotencyKey("archive-app") },
    });
  }

  listVersions(appId: string, signal?: AbortSignal) {
    return this.request<VersionListResponse>(
      `/v1/apps/${appId}/versions?limit=100`,
      { signal },
    );
  }

  async getVersion(appId: string, versionId: string, signal?: AbortSignal) {
    const response = await this.request<VersionResponse>(
      `/v1/apps/${appId}/versions/${versionId}`,
      { signal },
    );
    return response.data;
  }

  async createVersion(appId: string, input: CreateVersionRequest) {
    const response = await this.request<VersionResponse>(
      `/v1/apps/${appId}/versions`,
      {
        method: "POST",
        body: JSON.stringify(input),
        headers: { "Idempotency-Key": createIdempotencyKey("create-version") },
      },
    );
    return response.data;
  }

  async releaseVersion(
    appId: string,
    versionId: string,
    input: ReleaseVersionRequest,
  ) {
    const response = await this.request<VersionResponse>(
      `/v1/apps/${appId}/versions/${versionId}/release`,
      {
        method: "POST",
        body: JSON.stringify(input),
        headers: { "Idempotency-Key": createIdempotencyKey("release-version") },
      },
    );
    return response.data;
  }

  async rollbackVersion(appId: string, versionId: string) {
    const response = await this.request<ApiResponse<AppVersion>>(
      `/v1/apps/${appId}/versions/${versionId}/rollback`,
      {
        method: "POST",
        headers: {
          "Idempotency-Key": createIdempotencyKey("rollback-version"),
        },
      },
    );
    return response.data;
  }

  listExtensions(appId: string, signal?: AbortSignal) {
    return this.request<ExtensionListResponse>(`/v1/apps/${appId}/extensions`, {
      signal,
    });
  }

  async createExtension(appId: string, input: CreateExtensionRequest) {
    const response = await this.request<ExtensionResponse>(
      `/v1/apps/${appId}/extensions`,
      {
        method: "POST",
        body: JSON.stringify(input),
        headers: {
          "Idempotency-Key": createIdempotencyKey("create-extension"),
        },
      },
    );
    return response.data;
  }

  async updateExtension(
    appId: string,
    extensionId: string,
    input: UpdateExtensionRequest,
  ) {
    const response = await this.request<ExtensionResponse>(
      `/v1/apps/${appId}/extensions/${extensionId}`,
      {
        method: "PATCH",
        body: JSON.stringify(input),
      },
    );
    return response.data;
  }

  disableExtension(appId: string, extensionId: string) {
    return this.request<void>(`/v1/apps/${appId}/extensions/${extensionId}`, {
      method: "DELETE",
    });
  }

  listScopes(appId: string, signal?: AbortSignal) {
    return this.request<ScopeListResponse>(`/v1/apps/${appId}/scopes`, {
      signal,
    });
  }

  listScopeCatalog(signal?: AbortSignal) {
    return this.request<ScopeCatalogResponse>("/v1/scope-catalog", { signal });
  }

  listWebhookEventCatalog(signal?: AbortSignal) {
    return this.request<WebhookEventCatalogResponse>(
      "/v1/webhook-event-catalog",
      { signal },
    );
  }

  async replaceScopes(appId: string, input: ReplaceScopesRequest) {
    const response = await this.request<ScopeListResponse>(
      `/v1/apps/${appId}/scopes`,
      {
        method: "PUT",
        body: JSON.stringify(input),
      },
    );
    return response.data;
  }

  listCredentials(appId: string, signal?: AbortSignal) {
    return this.request<CredentialListResponse>(
      `/v1/apps/${appId}/credentials`,
      { signal },
    );
  }

  async createCredential(appId: string, input: CreateCredentialRequest) {
    const response = await this.request<CredentialSecretEnvelope>(
      `/v1/apps/${appId}/credentials`,
      {
        method: "POST",
        body: JSON.stringify(input),
        headers: {
          "Idempotency-Key": createIdempotencyKey("create-credential"),
        },
      },
    );
    return response.data;
  }

  async rotateCredential(appId: string, credentialId: string) {
    const response = await this.request<CredentialSecretEnvelope>(
      `/v1/apps/${appId}/credentials/${credentialId}/rotate`,
      {
        method: "POST",
        headers: {
          "Idempotency-Key": createIdempotencyKey("rotate-credential"),
        },
      },
    );
    return response.data;
  }

  revokeCredential(appId: string, credentialId: string) {
    return this.request<void>(`/v1/apps/${appId}/credentials/${credentialId}`, {
      method: "DELETE",
      headers: { "Idempotency-Key": createIdempotencyKey("revoke-credential") },
    });
  }

  listWebhooks(appId: string, signal?: AbortSignal) {
    return this.request<WebhookListResponse>(
      `/v1/apps/${appId}/webhooks?limit=100`,
      { signal },
    );
  }

  async createWebhook(appId: string, input: CreateWebhookRequest) {
    const response = await this.request<WebhookSecretEnvelope>(
      `/v1/apps/${appId}/webhooks`,
      {
        method: "POST",
        body: JSON.stringify(input),
        headers: { "Idempotency-Key": createIdempotencyKey("create-webhook") },
      },
    );
    return response.data;
  }

  async updateWebhook(
    appId: string,
    webhookId: string,
    input: UpdateWebhookRequest,
  ) {
    const response = await this.request<WebhookResponse>(
      `/v1/apps/${appId}/webhooks/${webhookId}`,
      {
        method: "PATCH",
        body: JSON.stringify(input),
      },
    );
    return response.data;
  }

  deleteWebhook(appId: string, webhookId: string) {
    return this.request<void>(`/v1/apps/${appId}/webhooks/${webhookId}`, {
      method: "DELETE",
    });
  }

  listWebhookDeliveries(
    appId: string,
    webhookId: string,
    signal?: AbortSignal,
  ) {
    return this.request<WebhookDeliveryListResponse>(
      `/v1/apps/${appId}/webhooks/${webhookId}/deliveries?limit=100`,
      { signal },
    );
  }

  async publishWebhookEvent(appId: string, input: PublishWebhookEventRequest) {
    const response = await this.request<PublishWebhookEventEnvelope>(
      `/v1/apps/${appId}/webhook-events`,
      {
        method: "POST",
        body: JSON.stringify(input),
        headers: {
          "Idempotency-Key": createIdempotencyKey("publish-webhook-event"),
        },
      },
    );
    return response.data;
  }

  listInstallations(appId: string, signal?: AbortSignal) {
    return this.request<InstallationListResponse>(
      `/v1/apps/${appId}/installations?limit=100`,
      { signal },
    );
  }

  async createInstallation(appId: string, input: CreateInstallationRequest) {
    const response = await this.request<InstallationResponse>(
      `/v1/apps/${appId}/installations`,
      {
        method: "POST",
        body: JSON.stringify(input),
        headers: {
          "Idempotency-Key": createIdempotencyKey("create-installation"),
        },
      },
    );
    return response.data;
  }

  listDevelopmentInstallRequests(appId: string, signal?: AbortSignal) {
    return this.request<DevelopmentInstallRequestListResponse>(
      `/v1/apps/${appId}/test-install-requests`,
      { signal },
    );
  }

  async createDevelopmentInstallRequest(
    appId: string,
    input: CreateDevelopmentInstallRequest,
  ) {
    const response = await this.request<DevelopmentInstallRequestResponse>(
      `/v1/apps/${appId}/test-install-requests`,
      {
        method: "POST",
        body: JSON.stringify(input),
        headers: {
          "Idempotency-Key": createIdempotencyKey("create-test-install"),
        },
      },
    );
    return response.data;
  }

  async updateInstallation(
    appId: string,
    installationId: string,
    input: UpdateInstallationRequest,
  ) {
    const response = await this.request<InstallationResponse>(
      `/v1/apps/${appId}/installations/${installationId}`,
      {
        method: "PATCH",
        body: JSON.stringify(input),
      },
    );
    return response.data;
  }

  async upgradeInstallation(
    appId: string,
    installationId: string,
    input: UpgradeInstallationRequest,
  ) {
    const response = await this.request<InstallationResponse>(
      `/v1/apps/${appId}/installations/${installationId}/upgrade`,
      {
        method: "POST",
        body: JSON.stringify(input),
        headers: {
          "Idempotency-Key": createIdempotencyKey("upgrade-installation"),
        },
      },
    );
    return response.data;
  }

  async authorizeOAuth(input: AuthorizeOAuthRequest) {
    const response = await this.request<ApiResponse<OAuthAuthorizationResult>>(
      "/v1/oauth/authorizations",
      {
        method: "POST",
        body: JSON.stringify(input),
      },
    );
    return response.data;
  }

  async exchangeOAuthToken(input: ExchangeOAuthTokenRequest) {
    const body = new URLSearchParams({
      grant_type: "authorization_code",
      code: input.code,
      redirect_uri: input.redirectUri,
      code_verifier: input.codeVerifier,
    });
    const response = await fetch(`${this.baseUrl}/oauth/token`, {
      method: "POST",
      credentials: "omit",
      headers: {
        Accept: "application/json",
        Authorization: `Basic ${globalThis.btoa(`${input.clientId}:${input.clientSecret}`)}`,
        "Content-Type": "application/x-www-form-urlencoded",
      },
      body,
    });
    if (!response.ok) {
      let payload: {
        error?: string;
        error_description?: string;
      } = {};
      try {
        payload = (await response.json()) as typeof payload;
      } catch {
        // Preserve a safe fallback when the OAuth endpoint returns non-JSON.
      }
      throw new AppPlatformApiError(
        payload.error_description ??
          `OAuth token exchange returned HTTP ${response.status}.`,
        { status: response.status, code: payload.error },
      );
    }
    return response.json() as Promise<OAuthTokenResult>;
  }

  async getInstallationContext(accessToken: string) {
    const response = await fetch(`${this.baseUrl}/v1/installation-context`, {
      method: "GET",
      credentials: "omit",
      headers: {
        Accept: "application/json",
        Authorization: `Bearer ${accessToken}`,
      },
    });
    if (!response.ok) {
      let envelope: ErrorEnvelope = {};
      try {
        envelope = (await response.json()) as ErrorEnvelope;
      } catch {
        // Preserve a useful HTTP fallback when an intermediary returns non-JSON.
      }
      throw new AppPlatformApiError(
        envelope.error?.message ??
          `Installation authentication returned HTTP ${response.status}.`,
        {
          status: response.status,
          code: envelope.error?.code,
          requestId: envelope.error?.requestId,
          details: envelope.error?.details,
        },
      );
    }
    const envelope =
      (await response.json()) as InstallationAccessContextResponse;
    return envelope.data;
  }

  async getMerchantProfile(accessToken: string) {
    const response = await fetch(`${this.baseUrl}/v1/merchant/profile`, {
      method: "GET",
      credentials: "omit",
      headers: {
        Accept: "application/json",
        Authorization: `Bearer ${accessToken}`,
      },
    });
    if (!response.ok) {
      let envelope: ErrorEnvelope = {};
      try {
        envelope = (await response.json()) as ErrorEnvelope;
      } catch {
        // Preserve a useful HTTP fallback when an intermediary returns non-JSON.
      }
      throw new AppPlatformApiError(
        envelope.error?.message ??
          `Merchant profile request returned HTTP ${response.status}.`,
        {
          status: response.status,
          code: envelope.error?.code,
          requestId: envelope.error?.requestId,
          details: envelope.error?.details,
        },
      );
    }
    const envelope = (await response.json()) as MerchantProfileResponse;
    return envelope.data;
  }

  uninstallInstallation(appId: string, installationId: string) {
    return this.request<void>(
      `/v1/apps/${appId}/installations/${installationId}`,
      {
        method: "DELETE",
        headers: {
          "Idempotency-Key": createIdempotencyKey("uninstall-installation"),
        },
      },
    );
  }
}

export const appPlatformClient = new AppPlatformClient();

export function apiErrorMessage(error: unknown) {
  if (error instanceof AppPlatformApiError) {
    return error.requestId
      ? `${error.message} (Request ${error.requestId})`
      : error.message;
  }
  if (error instanceof TypeError)
    return "Cannot reach App Gateway. Check that the local services are running.";
  return error instanceof Error
    ? error.message
    : "An unexpected error occurred.";
}
