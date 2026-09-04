"use client";

import { MerchantAppBilling } from './merchant-billing';

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";
import {
  ArrowRight,
  Boxes,
  Check,
  CheckCircle2,
  ChevronRight,
  ExternalLink,
  LayoutGrid,
  LoaderCircle,
  LockKeyhole,
  LogOut,
  Search,
  ShieldCheck,
  Sparkles,
  Store,
  Trash2,
} from "lucide-react";
import type {
  CatalogApp,
  CatalogCategory,
  MerchantInstalledApp,
  MerchantOAuthConsent,
  MerchantSession,
  PreviewMerchantOAuthRequest,
} from "../lib/app-platform/contracts";
import {
  AppPlatformApiError,
  apiErrorMessage,
  appPlatformClient,
} from "../lib/app-platform/client";
import { ConfirmDialog, InlineNotice } from "./components/ui";

const SCOPE_DESCRIPTIONS: Record<string, string> = {
  read_merchant: "View this merchant's installation identity",
  read_products: "View base product data (pilot; excludes variants)",
  write_products: "Create and update products and variants",
  read_orders: "View orders",
  write_orders: "Create and update orders",
  read_customers: "View customer profiles",
  write_customers: "Create and update customer profiles",
  read_inventory: "View inventory levels",
  write_inventory: "Adjust inventory quantities",
  read_fulfillments: "View fulfillment and shipment status",
  write_fulfillments: "Create fulfillment updates",
};

const DEFAULT_SANDBOX_MERCHANT_ID =
  process.env.NEXT_PUBLIC_SANDBOX_MERCHANT_ID ??
  "cmmerchantdemo000000000001";
const EMISELL_ID_PATTERN = /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/;

function oauthRequestKey(request: PreviewMerchantOAuthRequest) {
  return JSON.stringify([
    request.clientId,
    request.redirectUri,
    request.state,
    request.codeChallenge,
    request.requestedScopes,
	request.testInstallRequestId ?? "",
  ]);
}

function scopeDescription(scope: string) {
  return SCOPE_DESCRIPTIONS[scope] ?? "Use this app permission";
}

function initials(name: string) {
  return name
    .split(/\s+/)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase())
    .join("");
}

function unauthorized(error: unknown) {
  return error instanceof AppPlatformApiError && error.status === 401;
}

function environmentLabel(environment: MerchantSession["merchant"]["environment"]) {
  return environment === "production" ? "Production" : "Sandbox";
}

function MerchantBrand({ compact = false }: { compact?: boolean }) {
  return (
    <div className={`merchant-brand ${compact ? "compact" : ""}`}>
      <span>
        <Boxes size={compact ? 16 : 19} />
      </span>
      <div>
        <strong>Emisell</strong>
        <small>Merchant Apps</small>
      </div>
    </div>
  );
}

function SandboxMerchantLogin({
  onAuthenticated,
  title = "Continue as a sandbox merchant",
}: {
  onAuthenticated: (session: MerchantSession) => void;
  title?: string;
}) {
  const [merchantId, setMerchantId] = useState(DEFAULT_SANDBOX_MERCHANT_ID);
  const [merchantName, setMerchantName] = useState("Demo Merchant");
  const [merchantDomain, setMerchantDomain] = useState("demo.example.com");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async () => {
    setLoading(true);
    setError(null);
    try {
      const session = await appPlatformClient.sandboxMerchantLogin({
        merchantId: merchantId.trim(),
        merchantName: merchantName.trim(),
        ...(merchantDomain.trim()
          ? { merchantDomain: merchantDomain.trim().toLowerCase() }
          : {}),
      });
      onAuthenticated(session);
    } catch (loginError) {
      setError(apiErrorMessage(loginError));
      setLoading(false);
    }
  };

  return (
    <section className="merchant-login-card">
      <MerchantBrand />
      <div className="merchant-login-copy">
        <p className="section-kicker">Sandbox merchant identity</p>
        <h1>{title}</h1>
        <p>
          This development-only sign-in creates an isolated, server-side
          merchant session. Production will use the selected Emisell merchant
          identity provider.
        </p>
      </div>
      <div className="merchant-login-fields">
        <label>
          Store name
          <input
            value={merchantName}
            onChange={(event) => setMerchantName(event.target.value)}
          />
        </label>
        <label>
          Store domain
          <input
            value={merchantDomain}
            onChange={(event) => setMerchantDomain(event.target.value)}
          />
        </label>
        <label>
          Sandbox merchant ID
          <input
            value={merchantId}
            onChange={(event) => setMerchantId(event.target.value)}
            autoComplete="off"
            spellCheck={false}
          />
        </label>
      </div>
      <button
        className="primary-button merchant-login-action"
        disabled={
          loading ||
          merchantName.trim().length < 2 ||
          !EMISELL_ID_PATTERN.test(merchantId.trim())
        }
        onClick={() => void submit()}
      >
        {loading ? (
          <LoaderCircle className="spin" size={16} />
        ) : (
          <ShieldCheck size={16} />
        )}
        {loading ? "Starting session…" : "Continue in sandbox"}
        {!loading ? <ArrowRight size={15} /> : null}
      </button>
      {error ? (
        <p className="merchant-form-error" role="alert">
          {error}
        </p>
      ) : null}
      <div className="merchant-security-note">
        <LockKeyhole size={14} />
        <span>
          The merchant session uses a separate HttpOnly cookie and cannot access
          the Developer or Admin Console.
        </span>
      </div>
    </section>
  );
}

const CATALOG_CATEGORIES: Array<{ value: "all" | CatalogCategory; label: string }> = [
  { value: "all", label: "All apps" },
  { value: "payment", label: "Payments" },
  { value: "shipping", label: "Shipping" },
  { value: "erp", label: "ERP" },
  { value: "marketing", label: "Marketing" },
  { value: "operations", label: "Operations" },
  { value: "custom", label: "Custom" },
];

export function MerchantStoreView() {
  const [session, setSession] = useState<MerchantSession | null>(null);
  const [apps, setApps] = useState<CatalogApp[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [category, setCategory] = useState<"all" | CatalogCategory>("all");

  const loadCatalog = useCallback(async (currentSession: MerchantSession) => {
    setSession(currentSession);
    setLoading(true);
    setError(null);
    try {
      const response = await appPlatformClient.listCatalogApps();
      setApps(response.data);
    } catch (loadError) {
      if (unauthorized(loadError)) setSession(null);
      setError(apiErrorMessage(loadError));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    appPlatformClient
      .getMerchantSession(controller.signal)
      .then((currentSession) => loadCatalog(currentSession))
      .catch((sessionError) => {
        if (sessionError instanceof DOMException) return;
        if (!unauthorized(sessionError)) setError(apiErrorMessage(sessionError));
        setLoading(false);
      });
    return () => controller.abort();
  }, [loadCatalog]);

  const visibleApps = useMemo(() => {
    const query = search.trim().toLowerCase();
    return apps.filter((app) => {
      if (category !== "all" && app.category !== category) return false;
      if (!query) return true;
      return `${app.name} ${app.developerName} ${app.description ?? ""}`
        .toLowerCase()
        .includes(query);
    });
  }, [apps, category, search]);

  const logout = async () => {
    setError(null);
    try {
      await appPlatformClient.logoutMerchantSession();
      setSession(null);
      setApps([]);
    } catch (logoutError) {
      setError(apiErrorMessage(logoutError));
    }
  };

  if (!session && !loading) {
    return (
      <main className="merchant-auth-page">
        <SandboxMerchantLogin
          title="Sign in to browse Emisell App Store"
          onAuthenticated={loadCatalog}
        />
      </main>
    );
  }

  return (
    <main className="merchant-apps-page merchant-store-page">
      <header className="merchant-apps-topbar">
        <MerchantBrand compact />
        <nav className="merchant-portal-nav" aria-label="Merchant apps">
          <a className="active" href="/merchant/app-store">App Store</a>
          <a href="/merchant/apps">Connected apps</a>
        </nav>
        <div>
          {session ? (
            <div className="merchant-session-pill">
              <span>{initials(session.merchant.name)}</span>
              <div>
                <strong>{session.merchant.name}</strong>
                <small>{session.merchant.domain ?? session.merchant.merchantId}</small>
              </div>
            </div>
          ) : null}
          {session ? (
            <button className="secondary-button" onClick={() => void logout()}>
              <LogOut size={13} /> Sign out
            </button>
          ) : null}
        </div>
      </header>
      <div className="merchant-store-shell">
        <section className="merchant-store-hero">
          <div>
            <p className="section-kicker">Emisell App Store · {session ? environmentLabel(session.merchant.environment) : "Merchant"}</p>
            <h1>Apps built to extend your business</h1>
            <p>Discover integrations reviewed and published by Emisell. You will always review permissions before an app is installed.</p>
          </div>
          <span><Sparkles size={22} /></span>
        </section>
        <section className="merchant-store-toolbar">
          <label>
            <Search size={14} />
            <input
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Search apps or developers"
              aria-label="Search App Store"
            />
          </label>
          <div className="merchant-category-tabs">
            {CATALOG_CATEGORIES.map((item) => (
              <button
                type="button"
                key={item.value}
                className={category === item.value ? "active" : ""}
                onClick={() => setCategory(item.value)}
              >
                {item.label}
              </button>
            ))}
          </div>
        </section>
        {error ? <InlineNotice tone="warning" title="App Store unavailable">{error}</InlineNotice> : null}
        {loading ? (
          <section className="merchant-apps-loading"><LoaderCircle className="spin" size={18} />Loading reviewed apps…</section>
        ) : visibleApps.length ? (
          <section className="merchant-catalog-grid">
            {visibleApps.map((app) => (
              <article key={app.appId} className={app.featured ? "featured" : ""}>
                <header>
                  <span className="merchant-catalog-icon">{initials(app.name)}</span>
                  {app.featured ? <small><Sparkles size={11} /> Featured</small> : null}
                </header>
                <div>
                  <p className="merchant-catalog-category">{app.category}</p>
                  <h2>{app.name}</h2>
                  <p className="merchant-catalog-developer">by {app.developerName}</p>
                  <p className="merchant-catalog-description">{app.description ?? "A reviewed integration for Emisell merchants."}</p>
                </div>
                <footer>
                  <div>
                    <small>Version {app.version}</small>
                    <span>{app.requiredScopes.length} required permission{app.requiredScopes.length === 1 ? "" : "s"}</span>
                  </div>
                  <a className="primary-button" href={app.launchUrl}>
                    View & install <ArrowRight size={13} />
                  </a>
                </footer>
              </article>
            ))}
          </section>
        ) : (
          <section className="merchant-apps-empty">
            <span><LayoutGrid size={22} /></span>
            <h2>{apps.length ? "No matching apps" : "No apps published yet"}</h2>
            <p>{apps.length ? "Try another search or category." : "Apps appear here only after Emisell reviews and publishes an active version."}</p>
          </section>
        )}
      </div>
    </main>
  );
}

export function MerchantInstallView() {
  const searchParams = useSearchParams();
  const oauthRequest = useMemo<PreviewMerchantOAuthRequest | null>(() => {
    const clientId = searchParams.get("client_id")?.trim() ?? "";
    const redirectUri = searchParams.get("redirect_uri")?.trim() ?? "";
    const state = searchParams.get("state")?.trim() ?? "";
    const codeChallenge = searchParams.get("code_challenge")?.trim() ?? "";
    const method = searchParams.get("code_challenge_method")?.trim() ?? "";
    if (
      !clientId ||
      !redirectUri ||
      !state ||
      !codeChallenge ||
      method !== "S256"
    )
      return null;
    return {
      clientId,
      redirectUri,
      state,
      codeChallenge,
      requestedScopes: (searchParams.get("scope") ?? "")
        .split(/\s+/)
        .map((scope) => scope.trim())
        .filter(Boolean),
		...(searchParams.get("test_install_request")?.trim()
		  ? { testInstallRequestId: searchParams.get("test_install_request")!.trim() }
		  : {}),
    };
  }, [searchParams]);
  const [session, setSession] = useState<MerchantSession | null>(null);
  const [validatedConsent, setValidatedConsent] = useState<{
    request: PreviewMerchantOAuthRequest;
    consent: MerchantOAuthConsent;
  } | null>(null);
  const [optionalScopes, setOptionalScopes] = useState<string[]>([]);
  const [loading, setLoading] = useState(Boolean(oauthRequest));
  const [approving, setApproving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const validationSequence = useRef(0);
  const consent =
    validatedConsent &&
    oauthRequest &&
    oauthRequestKey(validatedConsent.request) === oauthRequestKey(oauthRequest)
      ? validatedConsent.consent
      : null;

  const loadConsent = useCallback(
    async (currentSession: MerchantSession) => {
      if (!oauthRequest) return;
      const sequence = ++validationSequence.current;
      setSession(currentSession);
      setValidatedConsent(null);
      setLoading(true);
      setError(null);
      try {
        const preview =
          await appPlatformClient.previewMerchantOAuth(oauthRequest);
        if (sequence !== validationSequence.current) return;
        setValidatedConsent({ request: oauthRequest, consent: preview });
        setOptionalScopes(preview.optionalScopes.map((scope) => scope.scope));
      } catch (previewError) {
        if (sequence !== validationSequence.current) return;
        if (unauthorized(previewError)) setSession(null);
        setError(apiErrorMessage(previewError));
      } finally {
        if (sequence === validationSequence.current) setLoading(false);
      }
    },
    [oauthRequest],
  );

  useEffect(() => {
    if (!oauthRequest) return;
    const controller = new AbortController();
    appPlatformClient
      .getMerchantSession(controller.signal)
      .then((currentSession) => loadConsent(currentSession))
      .catch((sessionError) => {
        if (sessionError instanceof DOMException) return;
        if (!unauthorized(sessionError))
          setError(apiErrorMessage(sessionError));
        setLoading(false);
      });
    return () => {
      controller.abort();
      validationSequence.current += 1;
    };
  }, [loadConsent, oauthRequest]);

  const toggleOptional = (scope: string) => {
    setOptionalScopes((current) =>
      current.includes(scope)
        ? current.filter((item) => item !== scope)
        : [...current, scope],
    );
  };

  const approve = async () => {
    if (!validatedConsent || !consent) return;
    setApproving(true);
    setError(null);
    try {
      const authorization = await appPlatformClient.authorizeMerchantOAuth({
        ...validatedConsent.request,
        grantedScopes: [
          ...consent.requiredScopes.map((scope) => scope.scope),
          ...optionalScopes,
        ],
      });
      window.location.assign(authorization.redirectTo);
    } catch (approvalError) {
      setError(apiErrorMessage(approvalError));
      setApproving(false);
    }
  };

  const deny = () => {
    if (!validatedConsent || !consent) return;
    const callback = new URL(consent.redirectUri);
    callback.searchParams.set("error", "access_denied");
    callback.searchParams.set("state", validatedConsent.request.state);
    window.location.assign(callback.toString());
  };

  if (!oauthRequest) {
    return (
      <main className="merchant-auth-page">
        <section className="merchant-invalid-card">
          <MerchantBrand />
          <span className="merchant-invalid-icon">
            <LockKeyhole size={22} />
          </span>
          <p className="section-kicker">Invalid install request</p>
          <h1>This app link is incomplete</h1>
          <p>
            Start from an app-generated install URL containing a client ID,
            exact callback, state, PKCE S256 challenge, and requested scopes.
          </p>
          <a className="secondary-button" href="/merchant/apps">
            View connected apps
            <ChevronRight size={14} />
          </a>
        </section>
      </main>
    );
  }

  if (!session && !loading) {
    return (
      <main className="merchant-auth-page">
        <SandboxMerchantLogin onAuthenticated={loadConsent} />
      </main>
    );
  }

  return (
    <main className="merchant-consent-page">
      <header className="merchant-consent-topbar">
        <MerchantBrand compact />
        {session ? (
          <div className="merchant-session-pill">
            <span>{initials(session.merchant.name)}</span>
            <div>
              <strong>{session.merchant.name}</strong>
              <small>{environmentLabel(session.merchant.environment)} merchant</small>
            </div>
          </div>
        ) : null}
      </header>
      <div className="merchant-consent-shell">
        {loading ? (
          <section className="merchant-consent-loading">
            <LoaderCircle className="spin" size={20} />
            Validating app, callback, version, and permissions…
          </section>
        ) : consent ? (
          <>
            <section className="merchant-consent-main">
              {consent.developmentInstall ? (
                <InlineNotice title="Development installation">
                  This app is not published in the App Store yet. The request is
                  limited to this Merchant ID and still uses the normal OAuth
                  consent and PKCE protections.
                </InlineNotice>
              ) : null}
              <div className="merchant-app-heading">
                <span>{initials(consent.app.name)}</span>
                <div>
                  <p className="section-kicker">App installation</p>
                  <h1>{consent.app.name} wants to connect</h1>
                  <p>
                    {consent.app.description ??
                      "This app is requesting access to your merchant workspace."}
                  </p>
                </div>
              </div>
              <div className="merchant-consent-target">
                <Store size={16} />
                <span>
                  Installing for <strong>{consent.merchant.name}</strong>
                  <small>
                    {consent.merchant.domain ?? consent.merchant.merchantId}
                  </small>
                </span>
                <b>{consent.developmentInstall ? "Development" : "Released"}</b>
              </div>
              <section className="merchant-permission-section">
                <header>
                  <div>
                    <h2>Required access</h2>
                    <p>The app cannot operate without these permissions.</p>
                  </div>
                  <span>{consent.requiredScopes.length}</span>
                </header>
                <div className="merchant-permission-list">
                  {consent.requiredScopes.length ? (
                    consent.requiredScopes.map((scope) => (
                      <div key={scope.scope}>
                        <span className="merchant-permission-check">
                          <Check size={12} />
                        </span>
                        <span>
                          <strong>{scopeDescription(scope.scope)}</strong>
                          <code>{scope.scope}</code>
                        </span>
                        <small>Required</small>
                      </div>
                    ))
                  ) : (
                    <p className="merchant-permission-empty">
                      No required scopes.
                    </p>
                  )}
                </div>
              </section>
              {consent.optionalScopes.length ? (
                <section className="merchant-permission-section optional">
                  <header>
                    <div>
                      <h2>Optional access</h2>
                      <p>Choose which additional permissions to grant.</p>
                    </div>
                    <span>{optionalScopes.length} enabled</span>
                  </header>
                  <div className="merchant-permission-list">
                    {consent.optionalScopes.map((scope) => {
                      const selected = optionalScopes.includes(scope.scope);
                      return (
                        <button
                          type="button"
                          className={selected ? "is-selected" : ""}
                          onClick={() => toggleOptional(scope.scope)}
                          key={scope.scope}
                        >
                          <span className="merchant-permission-check">
                            {selected ? <Check size={12} /> : null}
                          </span>
                          <span>
                            <strong>{scopeDescription(scope.scope)}</strong>
                            <code>{scope.scope}</code>
                          </span>
                          <small>{selected ? "Enabled" : "Not granted"}</small>
                        </button>
                      );
                    })}
                  </div>
                </section>
              ) : null}
              {error ? (
                <InlineNotice tone="warning" title="Cannot approve this app">
                  {error}
                </InlineNotice>
              ) : null}
              <div className="merchant-consent-actions">
                <button
                  className="secondary-button"
                  disabled={approving}
                  onClick={deny}
                >
                  Cancel
                </button>
                <button
                  className="primary-button"
                  disabled={approving}
                  onClick={() => void approve()}
                >
                  {approving ? (
                    <LoaderCircle className="spin" size={15} />
                  ) : (
                    <ShieldCheck size={15} />
                  )}
                  {approving ? "Authorizing…" : "Install app"}
                </button>
              </div>
            </section>
            <aside className="merchant-consent-aside">
              <p className="section-kicker">Security details</p>
              <h2>Before you install</h2>
              <dl>
                <div>
                  <dt>App version</dt>
                  <dd>v{consent.version.version}</dd>
                </div>
                <div>
                  <dt>App status</dt>
                  <dd>{consent.developmentInstall ? "Development" : "Released"}</dd>
                </div>
                <div>
                  <dt>Callback host</dt>
                  <dd>{new URL(consent.redirectUri).host}</dd>
                </div>
              </dl>
              <div className="merchant-aside-note">
                <LockKeyhole size={15} />
                <p>
                  The merchant identity comes from your server-side session.
                  This app cannot replace it in the authorization request.
                </p>
              </div>
              <a href="/merchant/apps">
                Review connected apps
                <ChevronRight size={13} />
              </a>
            </aside>
          </>
        ) : (
          <section className="merchant-consent-error">
            <LockKeyhole size={22} />
            <h1>This installation cannot continue</h1>
            <p>{error ?? "The app request could not be validated."}</p>
            <button
              className="secondary-button"
              onClick={() => window.location.reload()}
            >
              Try again
            </button>
          </section>
        )}
      </div>
    </main>
  );
}

export function MerchantAppsView() {
	const [billingInstallation, setBillingInstallation] = useState<string | null>(null);
  const [session, setSession] = useState<MerchantSession | null>(null);
  const [apps, setApps] = useState<MerchantInstalledApp[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [removeTarget, setRemoveTarget] = useState<MerchantInstalledApp | null>(
    null,
  );
  const [removing, setRemoving] = useState(false);

  const loadApps = useCallback(async (currentSession: MerchantSession) => {
    setSession(currentSession);
    setLoading(true);
    setError(null);
    try {
      setApps(await appPlatformClient.listMerchantInstalledApps());
    } catch (loadError) {
      if (unauthorized(loadError)) {
        setSession(null);
        setApps([]);
      }
      setError(apiErrorMessage(loadError));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    appPlatformClient
      .getMerchantSession(controller.signal)
      .then((currentSession) => loadApps(currentSession))
      .catch((sessionError) => {
        if (sessionError instanceof DOMException) return;
        if (!unauthorized(sessionError))
          setError(apiErrorMessage(sessionError));
        setLoading(false);
      });
    return () => controller.abort();
  }, [loadApps]);

  const logout = async () => {
    setError(null);
    try {
      await appPlatformClient.logoutMerchantSession();
      setSession(null);
      setApps([]);
    } catch (logoutError) {
      setError(apiErrorMessage(logoutError));
    }
  };

  const uninstall = async () => {
    if (!removeTarget) return;
    setRemoving(true);
    setError(null);
    try {
      await appPlatformClient.uninstallMerchantInstallation(
        removeTarget.installationId,
      );
      setApps((current) =>
        current.filter(
          (item) => item.installationId !== removeTarget.installationId,
        ),
      );
      setRemoveTarget(null);
    } catch (uninstallError) {
      setError(apiErrorMessage(uninstallError));
    } finally {
      setRemoving(false);
    }
  };

  if (!session && !loading) {
    return (
      <main className="merchant-auth-page">
        <SandboxMerchantLogin
          title="Sign in to manage connected apps"
          onAuthenticated={loadApps}
        />
      </main>
    );
  }

  return (
    <main className="merchant-apps-page">
      <header className="merchant-apps-topbar">
        <MerchantBrand compact />
        <nav className="merchant-portal-nav" aria-label="Merchant apps">
          <a href="/merchant/app-store">App Store</a>
          <a className="active" href="/merchant/apps">Connected apps</a>
        </nav>
        <div>
          {session ? (
            <div className="merchant-session-pill">
              <span>{initials(session.merchant.name)}</span>
              <div>
                <strong>{session.merchant.name}</strong>
                <small>{session.merchant.domain ?? `${environmentLabel(session.merchant.environment)} merchant`}</small>
              </div>
            </div>
          ) : null}
          {session ? (
            <button className="secondary-button" onClick={() => void logout()}>
              <LogOut size={13} />
              Sign out
            </button>
          ) : null}
        </div>
      </header>
      <div className="merchant-apps-shell">
        <section className="merchant-apps-heading">
          <div>
            <p className="section-kicker">{session ? environmentLabel(session.merchant.environment) : "Merchant"} workspace</p>
            <h1>Connected apps</h1>
            <p>
              Review installed versions and permissions, or revoke an app from
              this merchant.
            </p>
          </div>
          <div className="merchant-app-summary">
            <strong>{apps.length}</strong>
            <span>connected apps</span>
          </div>
        </section>
        {error ? (
          <InlineNotice tone="warning" title="Merchant apps unavailable">
            {error}
          </InlineNotice>
        ) : null}
        {loading ? (
          <section className="merchant-apps-loading">
            <LoaderCircle className="spin" size={18} />
            Reading connected apps…
          </section>
        ) : apps.length ? (
          <section className="merchant-connected-list">
            {apps.map((app) => (
              <article key={app.installationId}>
                <div className="merchant-connected-app-icon">
                  {initials(app.appName)}
                </div>
                <div className="merchant-connected-app-copy">
                  <header>
                    <div>
                      <h2>{app.appName}</h2>
                      <p>
                        Version {app.version} · Installed{" "}
                        {new Date(app.installedAt).toLocaleDateString()}
                      </p>
                    </div>
                    <span className={`status ${app.status}`}>
                      <i />
                      {app.status}
                    </span>
                  </header>
                  <p>
                    {app.appDescription ??
                      "Connected to this merchant."}
                  </p>
                  <div className="merchant-connected-scopes">
                    {app.grantedScopes.length ? (
                      app.grantedScopes.map((scope) => (
                        <code key={scope}>{scope}</code>
                      ))
                    ) : (
                      <small>No scopes granted</small>
                    )}
                  </div>
                </div>
                <div className="merchant-connected-actions">
                  <button className="secondary-button" aria-expanded={billingInstallation === app.installationId} onClick={() => setBillingInstallation(billingInstallation === app.installationId ? null : app.installationId)}>Plan & billing</button>
                  {app.appUrl ? (
                    <a
                      className="icon-button"
                      href={app.appUrl}
                      target="_blank"
                      rel="noreferrer"
                      aria-label={`Open ${app.appName}`}
                    >
                      <ExternalLink size={14} />
                    </a>
                  ) : null}
                  <button
                    className="icon-button danger-icon-button"
                    aria-label={`Uninstall ${app.appName}`}
                    onClick={() => setRemoveTarget(app)}
                  >
                    <Trash2 size={14} />
                  </button>
                </div>
                {billingInstallation === app.installationId && <MerchantAppBilling key={app.installationId} installationId={app.installationId} appName={app.appName} />}
              </article>
            ))}
          </section>
        ) : (
          <section className="merchant-apps-empty">
            <span>
              <CheckCircle2 size={22} />
            </span>
            <h2>No apps connected</h2>
            <p>
              Browse the Emisell App Store to review and connect an approved app.
            </p>
            <a className="secondary-button" href="/merchant/app-store">Browse App Store <ChevronRight size={13} /></a>
          </section>
        )}
      </div>
      <ConfirmDialog
        open={Boolean(removeTarget)}
        eyebrow="Merchant revocation"
        title={`Uninstall ${removeTarget?.appName ?? "this app"}?`}
        description="The app's access tokens will be revoked and subscription renewal cancelled. Already-issued invoices remain payable. Historical installation and audit records remain available to Emisell operators."
        confirmLabel="Uninstall app"
        tone="danger"
        loading={removing}
        onClose={() => {
          if (!removing) setRemoveTarget(null);
        }}
        onConfirm={() => void uninstall()}
      />
    </main>
  );
}
