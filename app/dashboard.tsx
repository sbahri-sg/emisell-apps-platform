"use client";

import { useEffect, useMemo, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { IntegrationReadinessView } from "./integration-readiness";
import { ExtensionCatalogPanel, useExtensionCatalog } from "./extension-catalog";
import {
  AlertCircle,
  AppWindow,
  ArrowLeft,
  ArrowUpRight,
  BellRing,
  Boxes,
  Building2,
  Check,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Clock3,
  Code2,
  Copy,
  CreditCard,
  Eye,
  EyeOff,
  FileCode2,
  KeyRound,
  LayoutDashboard,
  LoaderCircle,
  LogOut,
  Menu,
  PackageCheck,
  PauseCircle,
  Pencil,
  Play,
  Plus,
  RefreshCw,
  Rocket,
  Search,
  Settings,
  ShieldCheck,
  Store,
  Trash2,
  Truck,
  Webhook,
  X,
} from "lucide-react";
import {
  ConfirmDialog,
  InlineNotice,
  Panel,
  StatusBadge,
  Toast,
} from "./components/ui";
import type {
  App,
  AppCredential,
  AppExtension,
  AppVersion,
	DevelopmentInstallRequest,
  Environment,
  ExtensionType,
  Installation,
  OrganizationMembership,
  ScopeAccess,
  ScopeDefinition,
  SessionActor,
  VersionedScope,
  VersionedWebhookSubscription,
  WebhookDelivery,
  WebhookEventDefinition,
  WebhookSubscription,
} from "../lib/app-platform/domain";
import { apiErrorMessage, appPlatformClient } from "../lib/app-platform/client";
import { useAppPlatform } from "./app-platform-store";
import { AppPlansView } from "./app-plans";
import { DeveloperDocumentation } from "./docs/developer-documentation";

type Section = "Overview" | "Apps" | "Installations" | "Documentation";
type DetailTab =
  "Home" | "Integration" | "Versions" | "Extensions" | "API access" | "Webhooks" | "Plans & pricing" | "Settings";

const navItems: {
  label: Section;
  icon: typeof LayoutDashboard;
  count?: boolean;
}[] = [
  { label: "Overview", icon: LayoutDashboard },
  { label: "Apps", icon: AppWindow, count: true },
  { label: "Installations", icon: Building2 },
  { label: "Documentation", icon: FileCode2 },
];

const detailTabs: { label: DetailTab; icon: typeof LayoutDashboard }[] = [
  { label: "Home", icon: LayoutDashboard },
  { label: "Integration", icon: ShieldCheck },
  { label: "Versions", icon: PackageCheck },
  { label: "Extensions", icon: Boxes },
  { label: "API access", icon: KeyRound },
  { label: "Webhooks", icon: Webhook },
  { label: "Plans & pricing", icon: CreditCard },
  { label: "Settings", icon: Settings },
];

const sectionRoutes: Record<Section, string> = {
  Overview: "/overview",
  Apps: "/apps",
  Installations: "/stores",
  Documentation: "/docs",
};

const tabRoutes: Record<DetailTab, string> = {
  Home: "",
  Integration: "integration",
  Versions: "versions",
  Extensions: "extensions",
  "API access": "api-access",
  Webhooks: "webhooks",
  "Plans & pricing": "plans",
  Settings: "settings",
};

function getSection(pathname: string): Section {
  if (pathname === '/docs') return 'Documentation';
  if (pathname.startsWith("/apps")) return "Apps";
  if (pathname.startsWith("/stores")) return "Installations";
  return "Overview";
}

function getDetailTab(pathname: string): DetailTab {
  const slug = pathname.split("/").filter(Boolean)[2] ?? "";
  return (
    (Object.entries(tabRoutes).find(([, route]) => route === slug)?.[0] as
      DetailTab | undefined) ?? "Home"
  );
}

export default function Dashboard() {
  const pathname = usePathname();
  const router = useRouter();
  const {
    apps,
    loading,
    error,
    session,
    organizations,
    refreshApps,
    createApp: createAppRequest,
    updateApp,
    archiveApp,
    switchOrganization,
    logout,
  } = useAppPlatform();
  const section = getSection(pathname);
  const appSlug = pathname.split("/").filter(Boolean)[1];
  const selectedApp =
    section === "Apps" && appSlug
      ? (apps.find((app) => app.slug === appSlug) ?? null)
      : null;
  const detailTab = getDetailTab(pathname);
  const [query, setQuery] = useState("");
  const [mobileOpen, setMobileOpen] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [accountOpen, setAccountOpen] = useState(false);
  const [toast, setToast] = useState<{
    message: string;
    tone: "success" | "info" | "warning";
  } | null>(null);

  useEffect(() => {
    const detailRoute = pathname.split("/").filter(Boolean)[2];
    if (
      selectedApp &&
      detailRoute &&
      !Object.values(tabRoutes).includes(detailRoute)
    ) {
      router.replace(`/apps/${selectedApp.slug}`);
    }
  }, [pathname, router, selectedApp]);

  const notify = (
    message: string,
    tone: "success" | "info" | "warning" = "success",
  ) => {
    setToast({ message, tone });
    window.setTimeout(() => setToast(null), 2800);
  };

  const selectSection = (next: Section) => {
    setMobileOpen(false);
    router.push(sectionRoutes[next]);
  };

  const openApp = (app: App) => {
    router.push(`/apps/${app.slug}`);
  };

  const openTab = (tab: DetailTab) => {
    if (!selectedApp) return;
    const suffix = tabRoutes[tab];
    router.push(`/apps/${selectedApp.slug}${suffix ? `/${suffix}` : ""}`);
  };

  const createApp = async (name: string) => {
    const app = await createAppRequest({
      name,
      distribution: "custom",
      description: "Custom integration",
    });
    setCreateOpen(false);
    notify(`${app.name} created as a draft.`);
    router.push(`/apps/${app.slug}`);
  };

  const breadcrumb = selectedApp ? `Apps / ${selectedApp.name}` : section;

  return (
    <div className="app-shell">
      {mobileOpen ? (
        <button
          className="sidebar-scrim"
          aria-label="Close navigation"
          onClick={() => setMobileOpen(false)}
        />
      ) : null}
      <aside className={`main-sidebar ${mobileOpen ? "is-open" : ""}`}>
        <div className="brand">
          <div className="brand-mark">E</div>
          <div>
            <strong>Emisell</strong>
            <span>App Platform</span>
          </div>
          <button
            className="sidebar-close"
            aria-label="Close navigation"
            onClick={() => setMobileOpen(false)}
          >
            <X size={18} />
          </button>
        </div>

        <nav className="main-nav" aria-label="Main navigation">
          <p className="nav-label">Workspace</p>
          {navItems.map(({ label, icon: Icon, count }) => (
            <button
              className={`nav-item ${section === label ? "is-active" : ""}`}
              key={label}
              onClick={() => selectSection(label)}
            >
              <Icon size={18} strokeWidth={1.8} />
              <span>{label}</span>
              {count ? <small>{loading ? "…" : apps.length}</small> : null}
            </button>
          ))}
        </nav>

        <div className="sidebar-foot">
          <button
            className="workspace-switcher"
            onClick={() => setAccountOpen((open) => !open)}
            aria-expanded={accountOpen}
          >
            <span className="avatar">
              {appInitials(session?.displayName ?? "Emisell")}
            </span>
            <span>
              <strong>
                {session?.activeOrganization?.name ?? "Choose workspace"}
              </strong>
              <small>
                {session?.role
                  ? titleCase(session.role)
                  : "No active workspace"}
              </small>
            </span>
            <ChevronDown size={16} />
          </button>
        </div>
      </aside>

      <div className="main-column">
        <header className="topbar">
          <button
            className="icon-button mobile-menu"
            aria-label="Open navigation"
            onClick={() => setMobileOpen(true)}
          >
            <Menu size={20} />
          </button>
          <div className="crumb">
            <Boxes size={17} />
            <span>Developer workspace</span>
            <ChevronRight size={13} />
            <strong>{breadcrumb}</strong>
          </div>
          <div className="top-actions">
            <button
              className="top-avatar"
              aria-label="Open account menu"
              aria-expanded={accountOpen}
              onClick={() => setAccountOpen((open) => !open)}
            >
              {appInitials(session?.displayName ?? "Emisell")}
            </button>
          </div>
        </header>

        {accountOpen ? (
          <AccountMenu
            session={session}
            organizations={organizations}
            onClose={() => setAccountOpen(false)}
            onSwitch={switchOrganization}
            onLogout={logout}
            onOpenAdmin={() => router.push("/admin")}
          />
        ) : null}

        {selectedApp ? (
          <AppDetail
            app={selectedApp}
            activeTab={detailTab}
            onTabChange={openTab}
            onBack={() => router.push("/apps")}
            notify={notify}
            refreshApps={refreshApps}
            updateApp={updateApp}
            archiveApp={archiveApp}
            onArchived={() => router.push("/apps")}
            canDispatchWebhooks={
              session?.role === "owner" || session?.role === "admin"
            }
          />
        ) : section === "Apps" && appSlug && loading ? (
          <AppRouteState loading />
        ) : section === "Apps" && appSlug ? (
          <AppRouteState
            error={error ?? `No app found for “${appSlug}”.`}
            onRetry={refreshApps}
            onBack={() => router.push("/apps")}
          />
        ) : section === "Apps" ? (
          <AppsView
            apps={apps}
            loading={loading}
            error={error}
            onRetry={refreshApps}
            query={query}
            onQueryChange={setQuery}
            onOpenApp={openApp}
            onCreate={() => setCreateOpen(true)}
          />
        ) : section === "Documentation" ? (
          <DeveloperDocumentation />
        ) : section === "Overview" ? (
          <OverviewView
            apps={apps}
            loading={loading}
            session={session}
            onOpenApps={() => selectSection("Apps")}
          />
        ) : section === "Installations" ? (
          <StoresView
            apps={apps}
            loadingApps={loading}
            notify={notify}
            canManage={session?.role === "owner" || session?.role === "admin"}
            canSimulate={Boolean(session?.platformOperator)}
          />
        ) : null}
      </div>

      {createOpen ? (
        <CreateAppModal
          onClose={() => setCreateOpen(false)}
          onCreate={createApp}
        />
      ) : null}
      {toast ? (
        <Toast
          message={toast.message}
          tone={toast.tone}
          onClose={() => setToast(null)}
        />
      ) : null}
    </div>
  );
}

function AccountMenu({
  session,
  organizations,
  onClose,
  onSwitch,
  onLogout,
  onOpenAdmin,
}: {
  session: SessionActor | null;
  organizations: OrganizationMembership[];
  onClose: () => void;
  onSwitch: (organizationId: string) => Promise<void>;
  onLogout: () => Promise<void>;
  onOpenAdmin: () => void;
}) {
  const [busy, setBusy] = useState<string | null>(null);
  const runSwitch = async (organizationId: string) => {
    if (organizationId === session?.organizationId) return;
    setBusy(organizationId);
    try {
      await onSwitch(organizationId);
    } finally {
      setBusy(null);
    }
  };
  const runLogout = async () => {
    setBusy("logout");
    try {
      await onLogout();
    } finally {
      setBusy(null);
    }
  };
  return (
    <>
      <button
        className="account-menu-scrim"
        aria-label="Close account menu"
        onClick={onClose}
      />
      <section className="account-menu" aria-label="Account and workspaces">
        <header>
          <span className="account-avatar">
            {appInitials(session?.displayName ?? "Emisell")}
          </span>
          <span>
            <strong>{session?.displayName ?? "Developer"}</strong>
            <small>{session?.email ?? "Loading identity…"}</small>
          </span>
        </header>
        <div className="account-menu-label">Organizations</div>
        <div className="organization-list">
          {organizations.map((organization) => {
            const active =
              organization.organizationId === session?.organizationId;
            return (
              <button
                key={organization.organizationId}
                className={active ? "is-active" : ""}
                disabled={Boolean(busy) || organization.status !== "active"}
                title={
                  organization.status !== "active"
                    ? "This organization is suspended"
                    : undefined
                }
                onClick={() => void runSwitch(organization.organizationId)}
              >
                <span className="organization-mark">
                  {appInitials(organization.name)}
                </span>
                <span>
                  <strong>{organization.name}</strong>
                  <small>
                    {titleCase(organization.role)} ·{" "}
                    {titleCase(organization.status)}
                  </small>
                </span>
                {busy === organization.organizationId ? (
                  <LoaderCircle className="spin" size={15} />
                ) : active ? (
                  <Check size={15} />
                ) : null}
              </button>
            );
          })}
        </div>
        <footer>
          {session?.platformOperator ? (
            <button
              className="admin-console-link"
              disabled={Boolean(busy)}
              onClick={() => {
                onClose();
                onOpenAdmin();
              }}
            >
              <ShieldCheck size={15} />
              Open Admin Console
              <ArrowUpRight size={14} />
            </button>
          ) : null}
          <button disabled={Boolean(busy)} onClick={() => void runLogout()}>
            {busy === "logout" ? (
              <LoaderCircle className="spin" size={15} />
            ) : (
              <LogOut size={15} />
            )}
            Sign out
          </button>
        </footer>
      </section>
    </>
  );
}

const APP_TONES = ["violet", "blue", "amber", "green"] as const;

function titleCase(value: string) {
  return value.charAt(0).toUpperCase() + value.slice(1);
}

function appInitials(name: string) {
  return (
    name
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((part) => part[0])
      .join("")
      .toUpperCase() || "AP"
  );
}

function appTone(app: App) {
  const total = [...app.slug].reduce(
    (sum, character) => sum + character.charCodeAt(0),
    0,
  );
  return APP_TONES[total % APP_TONES.length];
}

function appReleaseLabel(app: App) {
  return app.status === "archived" ? "Archived" : titleCase(app.releaseStatus);
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat("en", {
    day: "numeric",
    month: "short",
    year: "numeric",
  }).format(new Date(value));
}

function formatRelativeDate(value: string) {
  const elapsed = Date.now() - new Date(value).getTime();
  const minutes = Math.max(0, Math.round(elapsed / 60_000));
  if (minutes < 1) return "Just now";
  if (minutes < 60) return `${minutes} min ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours} hr ago`;
  const days = Math.round(hours / 24);
  if (days === 1) return "Yesterday";
  if (days < 7) return `${days} days ago`;
  return formatDate(value);
}

function AppRouteState({
  loading = false,
  error,
  onRetry,
  onBack,
}: {
  loading?: boolean;
  error?: string;
  onRetry?: () => void;
  onBack?: () => void;
}) {
  return (
    <main className="page-content">
      <Panel className="resource-state">
        {loading ? (
          <LoaderCircle className="spin" size={24} />
        ) : (
          <AlertCircle size={24} />
        )}
        <strong>{loading ? "Loading app…" : "App unavailable"}</strong>
        <p>{loading ? "Reading the latest data from App Gateway." : error}</p>
        {!loading ? (
          <div>
            {onBack ? (
              <button className="secondary-button" onClick={onBack}>
                Back to apps
              </button>
            ) : null}
            {onRetry ? (
              <button className="primary-button" onClick={() => void onRetry()}>
                <RefreshCw size={14} />
                Retry
              </button>
            ) : null}
          </div>
        ) : null}
      </Panel>
    </main>
  );
}

function AppsView({
  apps,
  loading,
  error,
  onRetry,
  query,
  onQueryChange,
  onOpenApp,
  onCreate,
}: {
  apps: App[];
  loading: boolean;
  error: string | null;
  onRetry: () => Promise<void>;
  query: string;
  onQueryChange: (value: string) => void;
  onOpenApp: (app: App) => void;
  onCreate: () => void;
}) {
  const [statusFilter, setStatusFilter] = useState("All");
  const [typeFilter, setTypeFilter] = useState("All");
  const types = useMemo(
    () => [
      "All",
      ...Array.from(new Set(apps.map((app) => titleCase(app.distribution)))),
    ],
    [apps],
  );
  const filteredApps = useMemo(
    () =>
      apps.filter((app) => {
        const status = appReleaseLabel(app);
        const distribution = titleCase(app.distribution);
        return (
          `${app.name} ${distribution} ${status}`
            .toLowerCase()
            .includes(query.toLowerCase()) &&
          (statusFilter === "All" || status === statusFilter) &&
          (typeFilter === "All" || distribution === typeFilter)
        );
      }),
    [apps, query, statusFilter, typeFilter],
  );

  return (
    <main className="page-content">
      <section className="page-heading">
        <div>
          <p className="eyebrow">Build &amp; manage</p>
          <h1>Apps</h1>
          <p>
            Create, configure, and release apps across the Emisell ecosystem.
          </p>
        </div>
        <button className="primary-button" onClick={onCreate}>
          <Plus size={17} />
          Create app
        </button>
      </section>

      <section className="apps-card">
        <div className="table-toolbar">
          <label className="search-field">
            <Search size={17} />
            <input
              aria-label="Search apps"
              placeholder="Search apps"
              value={query}
              onChange={(event) => onQueryChange(event.target.value)}
            />
            <kbd>/</kbd>
          </label>
          <div className="filters">
            <label className="filter-select">
              <span>Status</span>
              <select
                aria-label="Filter by status"
                value={statusFilter}
                onChange={(event) => setStatusFilter(event.target.value)}
              >
                <option>All</option>
                <option>Development</option>
                <option>Released</option>
                <option>Archived</option>
              </select>
              <ChevronDown size={13} />
            </label>
            <label className="filter-select">
              <span>Distribution</span>
              <select
                aria-label="Filter by type"
                value={typeFilter}
                onChange={(event) => setTypeFilter(event.target.value)}
              >
                {types.map((type) => (
                  <option key={type}>{type}</option>
                ))}
              </select>
              <ChevronDown size={13} />
            </label>
          </div>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>App</th>
                <th>Status</th>
                <th>Distribution</th>
                <th>Updated</th>
                <th aria-label="Open" />
              </tr>
            </thead>
            <tbody>
              {!loading && !error
                ? filteredApps.map((app) => (
                    <tr
                      key={app.id}
                      onClick={() => onOpenApp(app)}
                      tabIndex={0}
                      onKeyDown={(event) =>
                        event.key === "Enter" && onOpenApp(app)
                      }
                    >
                      <td>
                        <div className="app-identity">
                          <span className={`app-icon ${appTone(app)}`}>
                            {appInitials(app.name)}
                          </span>
                          <span>
                            <strong>{app.name}</strong>
                            <small>
                              {app.description ?? "No description yet"}
                            </small>
                          </span>
                        </div>
                      </td>
                      <td>
                        <StatusBadge status={appReleaseLabel(app)} />
                      </td>
                      <td>
                        <span className="type-label">
                          {titleCase(app.distribution)}
                        </span>
                      </td>
                      <td className="updated">
                        {formatRelativeDate(app.updatedAt)}
                      </td>
                      <td>
                        <ChevronRight className="row-arrow" size={16} />
                      </td>
                    </tr>
                  ))
                : null}
            </tbody>
          </table>
          {loading ? (
            <div className="empty-state">
              <LoaderCircle className="spin" size={22} />
              <strong>Loading apps</strong>
              <p>Reading the latest data from App Gateway.</p>
            </div>
          ) : error ? (
            <div className="empty-state">
              <AlertCircle size={22} />
              <strong>Could not load apps</strong>
              <p>{error}</p>
              <button
                className="secondary-button empty-action"
                onClick={() => void onRetry()}
              >
                <RefreshCw size={14} />
                Retry
              </button>
            </div>
          ) : filteredApps.length === 0 ? (
            <div className="empty-state">
              <Search size={22} />
              <strong>
                {apps.length ? "No apps found" : "Create your first app"}
              </strong>
              <p>
                {apps.length
                  ? "Try another app name, status, or type."
                  : "Apps created here are stored by the Go App Gateway."}
              </p>
              {apps.length ? null : (
                <button
                  className="primary-button empty-action"
                  onClick={onCreate}
                >
                  <Plus size={14} />
                  Create app
                </button>
              )}
            </div>
          ) : null}
        </div>
        <footer className="table-footer">
          <span>
            {loading
              ? "Loading…"
              : `${filteredApps.length} ${filteredApps.length === 1 ? "app" : "apps"}`}
          </span>
          <span>{error ? "Disconnected" : "Live from App Gateway"}</span>
        </footer>
      </section>
    </main>
  );
}

function OverviewView({
  apps,
  loading,
  session,
  onOpenApps,
}: {
  apps: App[];
  loading: boolean;
  session: SessionActor | null;
  onOpenApps: () => void;
}) {
  const releasedApps = apps.filter(
    (app) => app.status !== "archived" && app.releaseStatus === "released",
  ).length;
  const developmentApps = apps.filter(
    (app) => app.status !== "archived" && app.releaseStatus === "development",
  ).length;
  const [installationCount, setInstallationCount] = useState<number | null>(
    null,
  );
  useEffect(() => {
    if (loading) return;
    const controller = new AbortController();
    Promise.all(
      apps.map((app) =>
        appPlatformClient.listInstallations(app.id, controller.signal),
      ),
    )
      .then((responses) =>
        setInstallationCount(
          responses.reduce(
            (total, response) =>
              total +
              response.data.filter(
                (installation) => installation.status === "active",
              ).length,
            0,
          ),
        ),
      )
      .catch(() => setInstallationCount(null));
    return () => controller.abort();
  }, [apps, loading]);
  return (
    <main className="page-content">
      <section className="page-heading overview-heading">
        <div>
          <p className="eyebrow">Developer workspace</p>
          <h1>
            Good morning, {session?.displayName.split(/\s+/)[0] || "developer"}
          </h1>
          <p>Here is what is happening across your Emisell apps.</p>
        </div>
        <button className="secondary-button" onClick={onOpenApps}>
          View all apps <ArrowUpRight size={15} />
        </button>
      </section>
      <div className="metric-grid">
        <MetricCard
          icon={AppWindow}
          label="Total apps"
          value={loading ? "…" : String(apps.length)}
          note={loading ? "Loading from gateway" : `${releasedApps} released`}
          tone="violet"
        />
        <MetricCard
          icon={CheckCircle2}
          label="Released apps"
          value={loading ? "…" : String(releasedApps)}
          note="Available through App Store"
          tone="green"
        />
        <MetricCard
          icon={FileCode2}
          label="Development apps"
          value={loading ? "…" : String(developmentApps)}
          note="Merchant ID testing"
          tone="amber"
        />
        <MetricCard
          icon={Store}
          label="Installations"
          value={installationCount === null ? "…" : String(installationCount)}
          note="Active merchant connections"
          tone="blue"
        />
      </div>
      <Panel>
        <div className="panel-heading">
          <div>
            <p className="section-kicker">Workspace data</p>
            <h2>Recently updated apps</h2>
            <p>Only apps stored for this organization are shown here.</p>
          </div>
          <button className="text-button" onClick={onOpenApps}>
            View apps
          </button>
        </div>
        {loading ? (
          <div className="empty-state">
            <LoaderCircle className="spin" size={22} />
            <strong>Loading apps</strong>
          </div>
        ) : apps.length ? (
          <div className="activity-list">
            {[...apps]
              .sort(
                (left, right) =>
                  Date.parse(right.updatedAt) - Date.parse(left.updatedAt),
              )
              .slice(0, 4)
              .map((app) => (
                <button
                  className="activity-item"
                  key={app.id}
                  onClick={onOpenApps}
                >
                  <span className={`activity-icon ${appTone(app)}`}>
                    <AppWindow size={15} />
                  </span>
                  <span>
                    <strong>{app.name}</strong>
                    <small>
                      {appReleaseLabel(app)} · Updated{" "}
                      {formatRelativeDate(app.updatedAt)}
                    </small>
                  </span>
                  <ChevronRight size={14} />
                </button>
              ))}
          </div>
        ) : (
          <div className="empty-state">
            <AppWindow size={22} />
            <strong>No apps yet</strong>
            <p>Create a custom app to begin development.</p>
          </div>
        )}
      </Panel>
    </main>
  );
}

function MetricCard({
  icon: Icon,
  label,
  value,
  note,
  tone,
}: {
  icon: typeof AppWindow;
  label: string;
  value: string;
  note: string;
  tone: string;
}) {
  return (
    <Panel className="metric-card">
      <span className={`metric-icon ${tone}`}>
        <Icon size={18} />
      </span>
      <p>{label}</p>
      <strong>{value}</strong>
      <small>{note}</small>
    </Panel>
  );
}

type InstallationRow = { installation: Installation; app: App };

function StoresView({
  apps,
  loadingApps,
  notify,
  canManage,
  canSimulate,
}: {
  apps: App[];
  loadingApps: boolean;
  notify: (message: string, tone?: "success" | "info" | "warning") => void;
  canManage: boolean;
  canSimulate: boolean;
}) {
  const [rows, setRows] = useState<InstallationRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState("Connected");
  const [simulatorOpen, setSimulatorOpen] = useState(false);
  const [upgradeRow, setUpgradeRow] = useState<InstallationRow | null>(null);
  const [savingId, setSavingId] = useState<string | null>(null);
  const [pending, setPending] = useState<{
    row: InstallationRow;
    action: "toggle" | "uninstall";
  } | null>(null);
  const eligibleApps = apps.filter(
    (app) => app.status === "active" && app.activeVersionId,
  );

  useEffect(() => {
    if (loadingApps) return;
    const controller = new AbortController();
    Promise.all(
      apps.map(async (app) => {
        const response = await appPlatformClient.listInstallations(
          app.id,
          controller.signal,
        );
        return response.data.map((installation) => ({ installation, app }));
      }),
    )
      .then((items) =>
        setRows(
          items
            .flat()
            .sort((a, b) =>
              b.installation.updatedAt.localeCompare(a.installation.updatedAt),
            ),
        ),
      )
      .catch((loadError) => {
        if (!(
          loadError instanceof DOMException && loadError.name === "AbortError"
        ))
          setError(apiErrorMessage(loadError));
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [apps, loadingApps]);

  const addSimulatedInstallation = (
    installation: Installation,
    appId: string,
  ) => {
    const app = apps.find((item) => item.id === appId);
    if (!app) return;
    setRows((current) => [
      { installation, app },
      ...current.filter((item) => item.installation.id !== installation.id),
    ]);
    notify(
      `${installation.merchantName} completed the OAuth installation.`,
    );
  };

  const upgradeInstallation = async (row: InstallationRow) => {
    if (!row.app.activeVersionId) return;
    setSavingId(row.installation.id);
    setError(null);
    try {
      const installation = await appPlatformClient.upgradeInstallation(
        row.app.id,
        row.installation.id,
        {
          targetVersionId: row.app.activeVersionId,
          expectedInstalledVersionId: row.installation.installedVersionId,
          revision: row.installation.revision,
        },
      );
      setRows((current) =>
        current.map((item) =>
          item.installation.id === installation.id
            ? { ...item, installation }
            : item,
        ),
      );
      setUpgradeRow(null);
      notify(
        `${row.installation.merchantName} upgraded to the active ${row.app.name} version.`,
      );
    } catch (saveError) {
      const message = apiErrorMessage(saveError);
      setError(message);
      notify(message, "warning");
      throw saveError;
    } finally {
      setSavingId(null);
    }
  };

  const applyAction = async () => {
    if (!pending) return;
    const { row, action } = pending;
    setSavingId(row.installation.id);
    setError(null);
    try {
      if (action === "uninstall") {
        await appPlatformClient.uninstallInstallation(
          row.app.id,
          row.installation.id,
        );
        const now = new Date().toISOString();
        setRows((current) =>
          current.map((item) =>
            item.installation.id === row.installation.id
              ? {
                  ...item,
                  installation: {
                    ...item.installation,
                    status: "uninstalled",
                    uninstalledAt: now,
                    updatedAt: now,
                    revision: item.installation.revision + 1,
                  },
                }
              : item,
          ),
        );
        notify(
          `${row.installation.merchantName} uninstalled from ${row.app.name}.`,
          "warning",
        );
      } else {
        const status =
          row.installation.status === "suspended" ? "active" : "suspended";
        const updated = await appPlatformClient.updateInstallation(
          row.app.id,
          row.installation.id,
          { status, revision: row.installation.revision },
        );
        setRows((current) =>
          current.map((item) =>
            item.installation.id === updated.id
              ? { ...item, installation: updated }
              : item,
          ),
        );
        notify(
          `${row.installation.merchantName} ${status === "active" ? "resumed" : "suspended"}.`,
        );
      }
      setPending(null);
    } catch (saveError) {
      const message = apiErrorMessage(saveError);
      setError(message);
      notify(message, "warning");
    } finally {
      setSavingId(null);
    }
  };

  const visibleRows = rows.filter(({ installation, app }) => {
    const matchesSearch =
      `${installation.merchantName} ${installation.merchantDomain ?? ""} ${installation.merchantId} ${app.name}`
        .toLowerCase()
        .includes(query.toLowerCase());
    const matchesStatus =
      statusFilter === "All" ||
      (statusFilter === "Connected"
        ? installation.status !== "uninstalled"
        : titleCase(installation.status) === statusFilter);
    return matchesSearch && matchesStatus;
  });
  const active = rows.filter(
    ({ installation }) => installation.status === "active",
  ).length;
  const development = rows.filter(
    ({ installation, app }) =>
      installation.status === "active" && app.releaseStatus === "development",
  ).length;
  const uninstalled = rows.filter(
    ({ installation }) => installation.status === "uninstalled",
  ).length;

  return (
    <main className="page-content">
      <section className="page-heading">
        <div>
          <p className="eyebrow">Merchant connections</p>
          <h1>Installations</h1>
          <p>
            Review merchant installations across development and released apps.
          </p>
        </div>
        <div className="heading-actions">
          {canSimulate ? (
            <button
              className="primary-button"
              onClick={() => setSimulatorOpen(true)}
              disabled={!eligibleApps.length || !canManage}
              title={canManage ? undefined : "Owner or admin access is required"}
            >
              <Rocket size={16} />
              Internal OAuth simulator
            </button>
          ) : null}
        </div>
      </section>
      {!loadingApps && !eligibleApps.length ? (
        <InlineNotice tone="warning" title="Activate an app version first">
          An app needs an active version before it can be installed.
        </InlineNotice>
      ) : null}
      {error ? (
        <InlineNotice tone="warning" title="Installation request failed">
          {error}
        </InlineNotice>
      ) : null}
      <div className="summary-grid installation-summary">
        <SummaryCard
          icon={Store}
          label="Active installations"
          value={loading ? "…" : String(active)}
          note="Currently connected"
        />
        <SummaryCard
          icon={Code2}
          label="Development"
          value={loading ? "…" : String(development)}
          note="Merchant ID test installs"
        />
        <SummaryCard
          icon={PauseCircle}
          label="Suspended"
          value={
            loading
              ? "…"
              : String(
                  rows.filter(
                    ({ installation }) => installation.status === "suspended",
                  ).length,
                )
          }
          note="Temporarily paused"
        />
        <SummaryCard
          icon={Trash2}
          label="Uninstalled"
          value={loading ? "…" : String(uninstalled)}
          note="Historical records"
        />
      </div>
      <section className="apps-card installation-card">
        <div className="table-toolbar">
          <label className="search-field">
            <Search size={17} />
            <input
              aria-label="Search installations"
              placeholder="Search merchants, domains, or apps"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
            />
          </label>
          <div className="filters">
            <label className="filter-select">
              <span>Status</span>
              <select
                aria-label="Filter installations"
                value={statusFilter}
                onChange={(event) => setStatusFilter(event.target.value)}
              >
                <option>Connected</option>
                <option>All</option>
                <option>Active</option>
                <option>Suspended</option>
                <option>Uninstalled</option>
              </select>
              <ChevronDown size={13} />
            </label>
          </div>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Merchant</th>
                <th>App</th>
                <th>Status</th>
                <th>Version</th>
                <th>Updated</th>
                <th aria-label="Actions" />
              </tr>
            </thead>
            <tbody>
              {!loading
                ? visibleRows.map((row) => (
                    <InstallationTableRow
                      key={row.installation.id}
                      row={row}
                      saving={savingId === row.installation.id}
                      canManage={canManage}
                      onUpgrade={() => setUpgradeRow(row)}
                      onToggle={() => setPending({ row, action: "toggle" })}
                      onUninstall={() =>
                        setPending({ row, action: "uninstall" })
                      }
                    />
                  ))
                : null}
            </tbody>
          </table>
          {loading ? (
            <div className="empty-state">
              <LoaderCircle className="spin" size={22} />
              <strong>Loading installations</strong>
              <p>Reading merchant connections from App Gateway.</p>
            </div>
          ) : visibleRows.length === 0 ? (
            <div className="empty-state">
              <Store size={22} />
              <strong>
                {rows.length
                  ? "No installations found"
                  : "No merchant installations yet"}
              </strong>
              <p>
                {rows.length
                  ? "Try another search or status filter."
                  : "Send a development test install from an app page."}
              </p>
              {rows.length || !eligibleApps.length || !canSimulate ? null : (
                <button
                  className="primary-button empty-action"
                  onClick={() => setSimulatorOpen(true)}
                  disabled={!canManage}
                >
                  <Rocket size={14} />
                  Internal OAuth simulator
                </button>
              )}
            </div>
          ) : null}
        </div>
        <footer className="table-footer">
          <span>{visibleRows.length} installations</span>
          <span>Live from App Gateway</span>
        </footer>
      </section>
      {simulatorOpen && canSimulate ? (
        <OAuthInstallSimulatorModal
          apps={eligibleApps}
          onClose={() => setSimulatorOpen(false)}
          onInstalled={addSimulatedInstallation}
        />
      ) : null}
      {upgradeRow ? (
        <UpgradeInstallationModal
          row={upgradeRow}
          saving={savingId === upgradeRow.installation.id}
          onClose={() => setUpgradeRow(null)}
          onUpgrade={upgradeInstallation}
        />
      ) : null}
      <ConfirmDialog
        open={Boolean(pending)}
        title={
          pending?.action === "uninstall"
            ? "Uninstall this app?"
            : pending?.row.installation.status === "suspended"
              ? "Resume installation?"
              : "Suspend installation?"
        }
        description={
          pending?.action === "uninstall"
            ? "The merchant will lose app access. Historical installation and audit records remain available."
            : pending?.row.installation.status === "suspended"
              ? "The merchant will regain access to the installed app version."
              : "The merchant will temporarily lose access until this installation is resumed."
        }
        confirmLabel={
          pending?.action === "uninstall"
            ? "Uninstall app"
            : pending?.row.installation.status === "suspended"
              ? "Resume installation"
              : "Suspend installation"
        }
        tone={pending?.action === "uninstall" ? "danger" : undefined}
        loading={Boolean(pending && savingId === pending.row.installation.id)}
        onClose={() => setPending(null)}
        onConfirm={() => void applyAction()}
      />
    </main>
  );
}

function InstallationTableRow({
  row,
  saving,
  canManage,
  onUpgrade,
  onToggle,
  onUninstall,
}: {
  row: InstallationRow;
  saving: boolean;
  canManage: boolean;
  onUpgrade: () => void;
  onToggle: () => void;
  onUninstall: () => void;
}) {
  const { installation, app } = row;
  const mutable = installation.status !== "uninstalled";
  const updateAvailable = Boolean(
    mutable &&
    app.activeVersionId &&
    app.activeVersionId !== installation.installedVersionId,
  );
  return (
    <tr>
      <td>
        <div className="merchant-identity">
          <span>{appInitials(installation.merchantName)}</span>
          <span>
            <strong>{installation.merchantName}</strong>
            <small>
              {installation.merchantDomain ?? installation.merchantId}
            </small>
          </span>
        </div>
      </td>
      <td>
        <div className="installation-app">
          <span className={`app-icon ${appTone(app)}`}>
            {appInitials(app.name)}
          </span>
          <strong>{app.name}</strong>
        </div>
      </td>
      <td>
        <StatusBadge status={titleCase(installation.status)} />
      </td>
      <td>
        <div className="installed-version">
          <code className="version-code">
            {installation.installedVersionId.slice(0, 8)}
          </code>
          {updateAvailable ? (
            <span className="update-available">
              <RefreshCw size={10} /> Update available
            </span>
          ) : null}
        </div>
      </td>
      <td className="updated">{formatRelativeDate(installation.updatedAt)}</td>
      <td>
        <div className="row-actions">
          {updateAvailable ? (
            <button
              className="secondary-button compact-button review-update"
              disabled={!canManage || saving}
              onClick={onUpgrade}
              title={
                canManage ? undefined : "Owner or admin access is required"
              }
            >
              <RefreshCw size={13} />
              Review update
            </button>
          ) : null}
          <button
            className="secondary-button compact-button"
            disabled={!mutable || !canManage || saving}
            onClick={onToggle}
          >
            {saving ? (
              <LoaderCircle className="spin" size={13} />
            ) : installation.status === "suspended" ? (
              <Play size={13} />
            ) : (
              <PauseCircle size={13} />
            )}
            {installation.status === "suspended" ? "Resume" : "Suspend"}
          </button>
          <button
            className="icon-button danger-icon"
            aria-label={`Uninstall ${installation.merchantName}`}
            disabled={!mutable || !canManage || saving}
            onClick={onUninstall}
          >
            <Trash2 size={14} />
          </button>
        </div>
      </td>
    </tr>
  );
}

type OAuthSimulationResult = {
  authorizationId: string;
  redirectUri: string;
  stateValidated: boolean;
  token: Awaited<ReturnType<typeof appPlatformClient.exchangeOAuthToken>>;
  accessContext: Awaited<
    ReturnType<typeof appPlatformClient.getInstallationContext>
  > | null;
  verificationError: string | null;
  merchantProfile: Awaited<
    ReturnType<typeof appPlatformClient.getMerchantProfile>
  > | null;
  resourceError: string | null;
};

function secureBase64Url(byteLength: number) {
  const bytes = new Uint8Array(byteLength);
  globalThis.crypto.getRandomValues(bytes);
  let binary = "";
  bytes.forEach((value) => {
    binary += String.fromCharCode(value);
  });
  return globalThis
    .btoa(binary)
    .replaceAll("+", "-")
    .replaceAll("/", "_")
    .replaceAll("=", "");
}

async function createPKCEPair() {
  const verifier = secureBase64Url(48);
  const digest = await globalThis.crypto.subtle.digest(
    "SHA-256",
    new TextEncoder().encode(verifier),
  );
  const bytes = new Uint8Array(digest);
  let binary = "";
  bytes.forEach((value) => {
    binary += String.fromCharCode(value);
  });
  return {
    verifier,
    challenge: globalThis
      .btoa(binary)
      .replaceAll("+", "-")
      .replaceAll("/", "_")
      .replaceAll("=", ""),
  };
}

function OAuthInstallSimulatorModal({
  apps,
  onClose,
  onInstalled,
}: {
  apps: App[];
  onClose: () => void;
  onInstalled: (installation: Installation, appId: string) => void;
}) {
  const [stage, setStage] = useState<"setup" | "consent" | "result">("setup");
  const [appId, setAppId] = useState(apps[0]?.id ?? "");
  const [credentials, setCredentials] = useState<AppCredential[]>([]);
  const [credentialId, setCredentialId] = useState("");
  const [version, setVersion] = useState<AppVersion | null>(null);
  const [resourcesLoading, setResourcesLoading] = useState(true);
  const [clientSecret, setClientSecret] = useState("");
  const [merchantName, setMerchantName] = useState("OAuth Test Merchant");
  const [merchantDomain, setMerchantDomain] = useState("oauth-sandbox.test");
  const [merchantId, setMerchantId] = useState(
    () => globalThis.crypto?.randomUUID?.() ?? "",
  );
  const [selectedOptionalScopes, setSelectedOptionalScopes] = useState<
    string[]
  >([]);
  const [working, setWorking] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<OAuthSimulationResult | null>(null);
  const [tokenVisible, setTokenVisible] = useState(false);
  const [tokenCopied, setTokenCopied] = useState(false);
  const [openedAt] = useState(() => Date.now());

  const app = apps.find((item) => item.id === appId) ?? null;
  const activeCredentials = credentials.filter(
    (credential) =>
      credential.environment === "sandbox" &&
      credential.status === "active" &&
      (!credential.expiresAt ||
        new Date(credential.expiresAt).getTime() > openedAt),
  );
  const selectedCredential =
    activeCredentials.find((item) => item.id === credentialId) ?? null;
  const redirectUri = version?.snapshot.redirectUrls[0] ?? "";
  const requiredScopes =
    version?.snapshot.scopes.filter((scope) => scope.access === "required") ??
    [];
  const optionalScopes =
    version?.snapshot.scopes.filter((scope) => scope.access === "optional") ??
    [];

  useEffect(() => {
    if (!app?.activeVersionId) return;
    const controller = new AbortController();
    Promise.all([
      appPlatformClient.listCredentials(app.id, controller.signal),
      appPlatformClient.getVersion(
        app.id,
        app.activeVersionId,
        controller.signal,
      ),
    ])
      .then(([credentialPage, activeVersion]) => {
        const available = credentialPage.data.filter(
          (credential) =>
            credential.environment === "sandbox" &&
            credential.status === "active" &&
            (!credential.expiresAt ||
              new Date(credential.expiresAt).getTime() > Date.now()),
        );
        setCredentials(credentialPage.data);
        setCredentialId(available[0]?.id ?? "");
        setVersion(activeVersion);
        setSelectedOptionalScopes([]);
      })
      .catch((loadError) => {
        if (!(
          loadError instanceof DOMException && loadError.name === "AbortError"
        ))
          setError(apiErrorMessage(loadError));
      })
      .finally(() => {
        if (!controller.signal.aborted) setResourcesLoading(false);
      });
    return () => controller.abort();
  }, [app?.activeVersionId, app?.id]);

  const selectApp = (nextAppId: string) => {
    setAppId(nextAppId);
    setCredentials([]);
    setCredentialId("");
    setVersion(null);
    setSelectedOptionalScopes([]);
    setResourcesLoading(true);
    setError("");
  };

  const reviewConsent = () => {
    if (!app || !version)
      return setError("Choose an app with an active released version.");
    if (!selectedCredential)
      return setError(
        "Create an active sandbox credential in API access before running the simulator.",
      );
    if (!redirectUri)
      return setError(
        "Set an HTTPS app URL and release a new version before running OAuth.",
      );
    if (!clientSecret.trim())
      return setError(
        "Enter the one-time client secret saved when this credential was created or rotated.",
      );
    if (merchantName.trim().length < 2)
      return setError("Merchant name must contain at least 2 characters.");
    if (!/^[0-9a-f-]{36}$/i.test(merchantId))
      return setError("Merchant ID must be a valid UUID.");
    if (
      merchantDomain &&
      !/^[a-z0-9][a-z0-9.-]*[a-z0-9]$/i.test(merchantDomain)
    )
      return setError("Use a hostname such as sandbox.example.test.");
    setError("");
    setStage("consent");
  };

  const runSimulation = async () => {
    if (!app || !version || !selectedCredential || !redirectUri) return;
    setWorking(true);
    setError("");
    try {
      const state = secureBase64Url(32);
      const pkce = await createPKCEPair();
      const grantedScopes = [
        ...requiredScopes.map((scope) => scope.scope),
        ...selectedOptionalScopes,
      ];
      const authorization = await appPlatformClient.authorizeOAuth({
        clientId: selectedCredential.clientId,
        redirectUri,
        state,
        codeChallenge: pkce.challenge,
        merchantId,
        merchantName: merchantName.trim(),
        ...(merchantDomain.trim()
          ? { merchantDomain: merchantDomain.trim().toLowerCase() }
          : {}),
        environment: "sandbox",
        grantedScopes,
      });
      const callback = new URL(authorization.redirectTo);
      const callbackCode = callback.searchParams.get("code");
      const callbackState = callback.searchParams.get("state");
      if (!callbackCode || callbackState !== state) {
        throw new Error(
          "OAuth callback validation failed. The authorization was not exchanged.",
        );
      }
      const token = await appPlatformClient.exchangeOAuthToken({
        clientId: selectedCredential.clientId,
        clientSecret,
        code: callbackCode,
        redirectUri,
        codeVerifier: pkce.verifier,
      });
      let accessContext: OAuthSimulationResult["accessContext"] = null;
      let verificationError: string | null = null;
      let merchantProfile: OAuthSimulationResult["merchantProfile"] = null;
      let resourceError: string | null = null;
      try {
        accessContext = await appPlatformClient.getInstallationContext(
          token.access_token,
        );
      } catch (verificationFailure) {
        verificationError = apiErrorMessage(verificationFailure);
      }
      if (accessContext) {
        try {
          merchantProfile = await appPlatformClient.getMerchantProfile(
            token.access_token,
          );
        } catch (resourceFailure) {
          resourceError = apiErrorMessage(resourceFailure);
        }
      }
      setClientSecret("");
      setResult({
        authorizationId: authorization.authorization.id,
        redirectUri,
        stateValidated: true,
        token,
        accessContext,
        verificationError,
        merchantProfile,
        resourceError,
      });
      onInstalled(token.installation, app.id);
      setStage("result");
    } catch (simulationError) {
      setError(apiErrorMessage(simulationError));
    } finally {
      setWorking(false);
    }
  };

  const toggleOptionalScope = (scope: string) => {
    setSelectedOptionalScopes((current) =>
      current.includes(scope)
        ? current.filter((item) => item !== scope)
        : [...current, scope],
    );
  };

  const copyToken = async () => {
    if (!result) return;
    await navigator.clipboard?.writeText(result.token.access_token);
    setTokenCopied(true);
    window.setTimeout(() => setTokenCopied(false), 1600);
  };

  return (
    <div
      className="modal-backdrop"
      role="presentation"
      onMouseDown={() => {
        if (!working) onClose();
      }}
    >
      <section
        className="modal oauth-simulator-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="oauth-simulator-title"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="modal-head oauth-simulator-head">
          <div>
            <p className="section-kicker">Sandbox developer tool</p>
            <h2 id="oauth-simulator-title">OAuth installation simulator</h2>
            <p>
              Run the real authorization-code and PKCE flow without leaving the
              developer workspace.
            </p>
          </div>
          <div className="simulator-stepper" aria-label="Simulation progress">
            {[
              ["setup", "1", "Client"],
              ["consent", "2", "Consent"],
              ["result", "3", "Result"],
            ].map(([value, number, label]) => (
              <span
                className={
                  stage === value ||
                  (stage === "consent" && value === "setup") ||
                  (stage === "result" && value !== "result")
                    ? "is-active"
                    : ""
                }
                key={value}
              >
                <i>{number}</i>
                {label}
              </span>
            ))}
          </div>
          <button
            className="icon-button"
            onClick={onClose}
            aria-label="Close"
            disabled={working}
          >
            <X size={17} />
          </button>
        </div>

        <div className="modal-body oauth-simulator-body">
          {stage === "setup" ? (
            <>
              <div className="simulator-context">
                <span>
                  <ShieldCheck size={16} />
                </span>
                <div>
                  <strong>Real sandbox flow</strong>
                  <p>
                    PKCE values are generated with Web Crypto. Secrets stay in
                    this dialog and are never written to browser storage.
                  </p>
                </div>
              </div>
              <div className="modal-grid">
                <label>
                  App
                  <select
                    value={appId}
                    onChange={(event) => selectApp(event.target.value)}
                    disabled={resourcesLoading}
                  >
                    {apps.map((item) => (
                      <option value={item.id} key={item.id}>
                        {item.name}
                      </option>
                    ))}
                  </select>
                </label>
                <label>
                  Environment
                  <input value="Sandbox" disabled />
                </label>
              </div>
              {resourcesLoading ? (
                <div className="simulator-loading">
                  <LoaderCircle className="spin" size={16} />
                  Reading active version and credentials…
                </div>
              ) : (
                <>
                  <label>
                    OAuth client
                    <select
                      value={credentialId}
                      onChange={(event) => {
                        setCredentialId(event.target.value);
                        setError("");
                      }}
                    >
                      {activeCredentials.length ? (
                        activeCredentials.map((credential) => (
                          <option value={credential.id} key={credential.id}>
                            {credential.clientId}
                          </option>
                        ))
                      ) : (
                        <option value="">No active sandbox credential</option>
                      )}
                    </select>
                  </label>
                  <label>
                    One-time client secret
                    <input
                      type="password"
                      autoComplete="new-password"
                      value={clientSecret}
                      onChange={(event) => {
                        setClientSecret(event.target.value);
                        setError("");
                      }}
                      placeholder="Paste the saved client secret"
                    />
                    <small className="field-hint">
                      In a real integration, token exchange belongs on the app
                      backend—not in merchant-facing browser code.
                    </small>
                  </label>
                  <div className="simulator-readonly-field">
                    <small>Exact callback URL</small>
                    <code>
                      {redirectUri || "No callback in active version"}
                    </code>
                  </div>
                </>
              )}
              <div className="modal-grid">
                <label>
                  Test merchant
                  <input
                    value={merchantName}
                    onChange={(event) => {
                      setMerchantName(event.target.value);
                      setError("");
                    }}
                  />
                </label>
                <label>
                  Merchant domain
                  <input
                    value={merchantDomain}
                    onChange={(event) => {
                      setMerchantDomain(event.target.value);
                      setError("");
                    }}
                  />
                </label>
              </div>
              <label>
                Merchant ID
                <input
                  value={merchantId}
                  onChange={(event) => {
                    setMerchantId(event.target.value);
                    setError("");
                  }}
                />
              </label>
              {error ? (
                <InlineNotice tone="warning" title="Cannot continue">
                  {error}
                </InlineNotice>
              ) : null}
            </>
          ) : stage === "consent" && app && version ? (
            <>
              <div className="consent-app-summary">
                <span className={`app-icon ${appTone(app)}`}>
                  {appInitials(app.name)}
                </span>
                <div>
                  <strong>{app.name}</strong>
                  <p>
                    wants access to <b>{merchantName}</b> using version v
                    {version.version}.
                  </p>
                </div>
                <span className="environment-pill sandbox">
                  <i /> Sandbox
                </span>
              </div>
              <section className="consent-scope-group">
                <header>
                  <div>
                    <strong>Required access</strong>
                    <p>Always granted for this released version.</p>
                  </div>
                  <span>{requiredScopes.length}</span>
                </header>
                <div>
                  {requiredScopes.length ? (
                    requiredScopes.map((scope) => (
                      <div className="consent-scope-row" key={scope.scope}>
                        <span className="scope-check">
                          <Check size={12} />
                        </span>
                        <span>
                          <code>{scope.scope}</code>
                          <small>{scopeDescription(scope.scope)}</small>
                        </span>
                        <small>Required</small>
                      </div>
                    ))
                  ) : (
                    <p className="consent-empty">No required scopes.</p>
                  )}
                </div>
              </section>
              <section className="consent-scope-group optional">
                <header>
                  <div>
                    <strong>Optional access</strong>
                    <p>Enable only what this test merchant needs.</p>
                  </div>
                  <span>{selectedOptionalScopes.length} selected</span>
                </header>
                <div>
                  {optionalScopes.length ? (
                    optionalScopes.map((scope) => {
                      const selected = selectedOptionalScopes.includes(
                        scope.scope,
                      );
                      return (
                        <button
                          className={`consent-scope-row ${selected ? "is-selected" : ""}`}
                          onClick={() => toggleOptionalScope(scope.scope)}
                          key={scope.scope}
                          type="button"
                        >
                          <span className="scope-check">
                            {selected ? <Check size={12} /> : null}
                          </span>
                          <span>
                            <code>{scope.scope}</code>
                            <small>{scopeDescription(scope.scope)}</small>
                          </span>
                          <small>{selected ? "Enabled" : "Optional"}</small>
                        </button>
                      );
                    })
                  ) : (
                    <p className="consent-empty">No optional scopes.</p>
                  )}
                </div>
              </section>
              <InlineNotice title="Short-lived authorization">
                The authorization code expires in five minutes, can be used
                once, and is bound to this callback URL and PKCE challenge.
              </InlineNotice>
              {error ? (
                <InlineNotice tone="warning" title="Simulation failed">
                  {error}
                </InlineNotice>
              ) : null}
            </>
          ) : stage === "result" && result ? (
            <>
              <div className="simulation-success">
                <span>
                  <CheckCircle2 size={24} />
                </span>
                <div>
                  <p className="section-kicker">Installation active</p>
                  <h3>{result.token.installation.merchantName} connected</h3>
                  <p>
                    Authorization code consumed once. The installation and
                    hashed token record were committed together
                    {result.accessContext
                      ? ", then authenticated against the live installation context."
                      : "."}
                  </p>
                </div>
              </div>
              <div className="simulation-timeline">
                {[
                  ["PKCE S256 challenge generated", "Browser simulator"],
                  ["Access consent approved", result.authorizationId],
                  ["Callback state validated", result.redirectUri],
                  ["Authorization code exchanged", "Single use"],
                  [
                    "Installation activated",
                    `v${version?.version ?? result.token.installation.installedVersionId.slice(0, 8)}`,
                  ],
                  ...(result.accessContext
                    ? [
                        [
                          "Access token authenticated",
                          `${result.accessContext.environment} · ${result.accessContext.scopes.length} scopes`,
                        ],
                      ]
                    : []),
                  ...(result.merchantProfile
                    ? [
                        [
                          "Protected merchant resource read",
                          "GET /v1/merchant/profile",
                        ],
                      ]
                    : []),
                ].map(([title, detail]) => (
                  <div key={title}>
                    <span>
                      <Check size={12} />
                    </span>
                    <span>
                      <strong>{title}</strong>
                      <small>{detail}</small>
                    </span>
                  </div>
                ))}
              </div>
              <section className="simulator-token-result">
                <header>
                  <div>
                    <strong>Sandbox access token</strong>
                    <p>
                      Expires in {Math.round(result.token.expires_in / 60)}
                      minutes · {result.token.scope || "No scopes"}
                    </p>
                  </div>
                  <span>Shown once</span>
                </header>
                <div>
                  <code>
                    {tokenVisible
                      ? result.token.access_token
                      : `••••••••••••••••${result.token.access_token.slice(-8)}`}
                  </code>
                  <button
                    className="icon-button"
                    onClick={() => setTokenVisible((visible) => !visible)}
                    aria-label={
                      tokenVisible ? "Hide access token" : "Show access token"
                    }
                  >
                    {tokenVisible ? <EyeOff size={14} /> : <Eye size={14} />}
                  </button>
                  <button
                    className="secondary-button compact-button"
                    onClick={() => void copyToken()}
                  >
                    {tokenCopied ? <Check size={13} /> : <Copy size={13} />}
                    {tokenCopied ? "Copied" : "Copy token"}
                  </button>
                </div>
              </section>
              {result.accessContext ? (
                <section className="simulation-context-result">
                  <header>
                    <div>
                      <strong>Authenticated installation context</strong>
                      <p>
                        Safe identity and grants resolved from the opaque token.
                      </p>
                    </div>
                    <span>Active</span>
                  </header>
                  <dl>
                    <div>
                      <dt>App</dt>
                      <dd>{result.accessContext.appName}</dd>
                    </div>
                    <div>
                      <dt>Merchant</dt>
                      <dd>{result.accessContext.merchantName}</dd>
                    </div>
                    <div>
                      <dt>Environment</dt>
                      <dd>{result.accessContext.environment}</dd>
                    </div>
                    <div>
                      <dt>Installed version</dt>
                      <dd>
                        {version?.id === result.accessContext.installedVersionId
                          ? `v${version.version}`
                          : result.accessContext.installedVersionId.slice(0, 8)}
                      </dd>
                    </div>
                    <div className="simulation-context-scopes">
                      <dt>Effective scopes</dt>
                      <dd>
                        {result.accessContext.scopes.length
                          ? result.accessContext.scopes.map((scope) => (
                              <code key={scope}>{scope}</code>
                            ))
                          : "No scopes granted"}
                      </dd>
                    </div>
                    <div>
                      <dt>Token expires</dt>
                      <dd>
                        {new Date(
                          result.accessContext.tokenExpiresAt,
                        ).toLocaleString()}
                      </dd>
                    </div>
                  </dl>
                </section>
              ) : null}
              {result.verificationError ? (
                <InlineNotice
                  tone="warning"
                  title="Installation active, token verification unavailable"
                >
                  {result.verificationError}
                </InlineNotice>
              ) : null}
              {result.merchantProfile ? (
                <section className="simulation-resource-result">
                  <header>
                    <span>
                      <ShieldCheck size={15} />
                    </span>
                    <div>
                      <strong>Protected resource verified</strong>
                      <code>GET /v1/merchant/profile</code>
                    </div>
                    <b>200 · read_merchant</b>
                  </header>
                  <div>
                    <span>
                      <small>Merchant</small>
                      <strong>{result.merchantProfile.name}</strong>
                    </span>
                    <span>
                      <small>Domain</small>
                      <strong>
                        {result.merchantProfile.domain ?? "Not provided"}
                      </strong>
                    </span>
                  </div>
                </section>
              ) : result.resourceError ? (
                <InlineNotice
                  tone="warning"
                  title={
                    result.accessContext?.scopes.includes("read_merchant")
                      ? "Protected resource check failed"
                      : "Protected resource blocked by scope"
                  }
                >
                  {result.accessContext?.scopes.includes("read_merchant")
                    ? result.resourceError
                    : "The gateway correctly rejected this request. Add read_merchant to the app scopes, release a new version, then grant it during the next installation."}
                </InlineNotice>
              ) : null}
              <InlineNotice tone="warning" title="Keep this token private">
                It is available only until this dialog closes. Store it in a
                local secret vault and never commit it to source control.
              </InlineNotice>
            </>
          ) : null}
        </div>

        <div className="modal-foot modal-foot-split">
          {stage === "setup" ? (
            <>
              <span>Owner/admin sandbox tool</span>
              <button className="secondary-button" onClick={onClose}>
                Cancel
              </button>
              <button
                className="primary-button"
                onClick={reviewConsent}
                disabled={resourcesLoading}
              >
                Review access
                <ChevronRight size={15} />
              </button>
            </>
          ) : stage === "consent" ? (
            <>
              <span>Uses the real OAuth endpoints</span>
              <button
                className="secondary-button"
                onClick={() => {
                  setError("");
                  setStage("setup");
                }}
                disabled={working}
              >
                Back
              </button>
              <button
                className="primary-button"
                onClick={() => void runSimulation()}
                disabled={working}
              >
                {working ? (
                  <LoaderCircle className="spin" size={15} />
                ) : (
                  <ShieldCheck size={15} />
                )}
                {working ? "Running OAuth flow…" : "Approve & simulate"}
              </button>
            </>
          ) : (
            <>
              <span>Secret values clear when closed</span>
              <button className="primary-button" onClick={onClose}>
                Done
              </button>
            </>
          )}
        </div>
      </section>
    </div>
  );
}

function UpgradeInstallationModal({
  row,
  saving,
  onClose,
  onUpgrade,
}: {
  row: InstallationRow;
  saving: boolean;
  onClose: () => void;
  onUpgrade: (row: InstallationRow) => Promise<void>;
}) {
  const [installedVersion, setInstalledVersion] = useState<AppVersion | null>(
    null,
  );
  const [targetVersion, setTargetVersion] = useState<AppVersion | null>(null);
  const [loading, setLoading] = useState(Boolean(row.app.activeVersionId));
  const [error, setError] = useState(() =>
    row.app.activeVersionId ? "" : "This app no longer has an active version.",
  );

  useEffect(() => {
    const targetVersionId = row.app.activeVersionId;
    if (!targetVersionId) return;
    const controller = new AbortController();
    Promise.all([
      appPlatformClient.getVersion(
        row.app.id,
        row.installation.installedVersionId,
        controller.signal,
      ),
      appPlatformClient.getVersion(
        row.app.id,
        targetVersionId,
        controller.signal,
      ),
    ])
      .then(([installed, target]) => {
        setInstalledVersion(installed);
        setTargetVersion(target);
      })
      .catch((loadError) => {
        if (!(
          loadError instanceof DOMException && loadError.name === "AbortError"
        ))
          setError(apiErrorMessage(loadError));
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [row]);

  const diff = useMemo(() => {
    if (!installedVersion || !targetVersion) return null;
    const currentExtensions = new Map(
      installedVersion.snapshot.extensions.map((item) => [
        item.extensionId,
        item,
      ]),
    );
    const nextExtensions = new Map(
      targetVersion.snapshot.extensions.map((item) => [item.extensionId, item]),
    );
    const extensionChanges = [
      ...targetVersion.snapshot.extensions
        .filter((item) => !currentExtensions.has(item.extensionId))
        .map((item) => ({ tone: "added" as const, label: item.name })),
      ...installedVersion.snapshot.extensions
        .filter((item) => !nextExtensions.has(item.extensionId))
        .map((item) => ({ tone: "removed" as const, label: item.name })),
      ...targetVersion.snapshot.extensions
        .filter((item) => {
          const current = currentExtensions.get(item.extensionId);
          return current && JSON.stringify(current) !== JSON.stringify(item);
        })
        .map((item) => ({ tone: "changed" as const, label: item.name })),
    ];

    const currentWebhooks = new Map(
      installedVersion.snapshot.webhookSubscriptions.map((item) => [
        item.subscriptionId,
        item,
      ]),
    );
    const nextWebhooks = new Map(
      targetVersion.snapshot.webhookSubscriptions.map((item) => [
        item.subscriptionId,
        item,
      ]),
    );
    const webhookChanges = [
      ...targetVersion.snapshot.webhookSubscriptions
        .filter((item) => !currentWebhooks.has(item.subscriptionId))
        .map((item) => ({ tone: "added" as const, label: item.event })),
      ...installedVersion.snapshot.webhookSubscriptions
        .filter((item) => !nextWebhooks.has(item.subscriptionId))
        .map((item) => ({ tone: "removed" as const, label: item.event })),
      ...targetVersion.snapshot.webhookSubscriptions
        .filter((item) => {
          const current = currentWebhooks.get(item.subscriptionId);
          return current && JSON.stringify(current) !== JSON.stringify(item);
        })
        .map((item) => ({ tone: "changed" as const, label: item.event })),
    ];

    const granted = new Set(row.installation.grantedScopes);
    const targetScopes = new Map(
      targetVersion.snapshot.scopes.map((item) => [item.scope, item.access]),
    );
    const requiredAdded = targetVersion.snapshot.scopes
      .filter((item) => item.access === "required" && !granted.has(item.scope))
      .map((item) => item.scope);
    const removed = row.installation.grantedScopes.filter(
      (scope) => !targetScopes.has(scope),
    );
    const optionalPreserved = targetVersion.snapshot.scopes
      .filter((item) => item.access === "optional" && granted.has(item.scope))
      .map((item) => item.scope);
    const optionalNotGranted = targetVersion.snapshot.scopes
      .filter((item) => item.access === "optional" && !granted.has(item.scope))
      .map((item) => item.scope);
    return {
      extensionChanges,
      webhookChanges,
      requiredAdded,
      removed,
      optionalPreserved,
      optionalNotGranted,
    };
  }, [installedVersion, row.installation.grantedScopes, targetVersion]);

  const submit = async () => {
    setError("");
    try {
      await onUpgrade(row);
    } catch (saveError) {
      setError(apiErrorMessage(saveError));
    }
  };

  return (
    <div
      className="modal-backdrop"
      role="presentation"
      onMouseDown={() => {
        if (!saving) onClose();
      }}
    >
      <section
        className="modal upgrade-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="upgrade-installation-title"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="modal-head">
          <div>
            <p className="section-kicker">Version upgrade</p>
            <h2 id="upgrade-installation-title">
              Review changes for {row.installation.merchantName}
            </h2>
            <p>
              Confirm the released configuration before moving this sandbox
              installation forward.
            </p>
          </div>
          <button
            className="icon-button"
            onClick={onClose}
            aria-label="Close"
            disabled={saving}
          >
            <X size={17} />
          </button>
        </div>
        <div className="modal-body upgrade-modal-body">
          {loading ? (
            <div className="upgrade-loading">
              <LoaderCircle className="spin" size={18} />
              Reading released version snapshots…
            </div>
          ) : error && !diff ? (
            <InlineNotice tone="warning" title="Cannot review this update">
              {error}
            </InlineNotice>
          ) : installedVersion && targetVersion && diff ? (
            <>
              <div className="version-transition">
                <div>
                  <small>Installed</small>
                  <strong>v{installedVersion.version}</strong>
                  <span>Pinned sandbox version</span>
                </div>
                <ArrowUpRight size={18} />
                <div className="is-target">
                  <small>Active release</small>
                  <strong>v{targetVersion.version}</strong>
                  <span>
                    {targetVersion.releaseNote ?? "Current app release"}
                  </span>
                </div>
              </div>

              {diff.requiredAdded.length ? (
                <InlineNotice tone="warning" title="New required access">
                  Upgrading grants {diff.requiredAdded.join(", ")}. New optional
                  scopes remain disabled until a separate consent flow exists.
                </InlineNotice>
              ) : (
                <InlineNotice title="Scope-safe upgrade">
                  Existing optional access is preserved, removed scopes are
                  dropped, and new optional scopes are not auto-enabled.
                </InlineNotice>
              )}

              <div className="upgrade-diff-grid">
                <UpgradeDiffSection
                  title="Scopes"
                  empty="No scope changes"
                  items={[
                    ...diff.requiredAdded.map((label) => ({
                      tone: "added" as const,
                      label,
                      note: "Required access",
                    })),
                    ...diff.removed.map((label) => ({
                      tone: "removed" as const,
                      label,
                      note: "No longer in release",
                    })),
                    ...diff.optionalPreserved.map((label) => ({
                      tone: "kept" as const,
                      label,
                      note: "Optional · preserved",
                    })),
                    ...diff.optionalNotGranted.map((label) => ({
                      tone: "pending" as const,
                      label,
                      note: "Optional · not granted",
                    })),
                  ]}
                />
                <UpgradeDiffSection
                  title="Extensions"
                  empty="No extension changes"
                  items={diff.extensionChanges}
                />
                <UpgradeDiffSection
                  title="Webhooks"
                  empty="No webhook changes"
                  items={diff.webhookChanges}
                />
              </div>
              {error ? (
                <InlineNotice tone="warning" title="Upgrade failed">
                  {error}
                </InlineNotice>
              ) : null}
            </>
          ) : null}
        </div>
        <div className="modal-foot modal-foot-split">
          <span>Only owner and admin roles can approve this change.</span>
          <button
            className="secondary-button"
            onClick={onClose}
            disabled={saving}
          >
            Cancel
          </button>
          <button
            className="primary-button"
            onClick={() => void submit()}
            disabled={loading || !diff || saving}
          >
            {saving ? (
              <LoaderCircle className="spin" size={15} />
            ) : (
              <RefreshCw size={15} />
            )}
            {saving ? "Upgrading…" : "Upgrade installation"}
          </button>
        </div>
      </section>
    </div>
  );
}

type UpgradeDiffItem = {
  tone: "added" | "removed" | "changed" | "kept" | "pending";
  label: string;
  note?: string;
};

function UpgradeDiffSection({
  title,
  empty,
  items,
}: {
  title: string;
  empty: string;
  items: UpgradeDiffItem[];
}) {
  return (
    <section className="upgrade-diff-section">
      <header>
        <strong>{title}</strong>
        <span>{items.length}</span>
      </header>
      {items.length ? (
        <div>
          {items.map((item, index) => (
            <div
              className={`upgrade-diff-item ${item.tone}`}
              key={`${item.label}-${index}`}
            >
              <span>
                {item.tone === "removed" ? (
                  <Trash2 size={12} />
                ) : item.tone === "changed" ? (
                  <RefreshCw size={12} />
                ) : item.tone === "pending" ? (
                  <Clock3 size={12} />
                ) : (
                  <Check size={12} />
                )}
              </span>
              <span>
                <code>{item.label}</code>
                <small>
                  {item.note ??
                    (item.tone === "added"
                      ? "Added"
                      : item.tone === "removed"
                        ? "Removed"
                        : "Configuration changed")}
                </small>
              </span>
            </div>
          ))}
        </div>
      ) : (
        <p>{empty}</p>
      )}
    </section>
  );
}

function AppDetail({
  app,
  activeTab,
  onTabChange,
  onBack,
  notify,
  refreshApps,
  updateApp,
  archiveApp,
  onArchived,
  canDispatchWebhooks,
}: {
  app: App;
  activeTab: DetailTab;
  onTabChange: (tab: DetailTab) => void;
  onBack: () => void;
  notify: (message: string, tone?: "success" | "info" | "warning") => void;
  refreshApps: () => Promise<void>;
  updateApp: (
    appId: string,
    input: Parameters<typeof appPlatformClient.updateApp>[1],
  ) => Promise<App>;
  archiveApp: (appId: string) => Promise<void>;
  onArchived: () => void;
  canDispatchWebhooks: boolean;
}) {
  return (
    <main className="detail-page">
      <div className="detail-top">
        <button className="back-button" onClick={onBack}>
          <ArrowLeft size={15} />
          Apps
        </button>
        <div className="detail-title-row">
          <div className="detail-app-title">
            <span className={`app-icon ${appTone(app)}`}>
              {appInitials(app.name)}
            </span>
            <div>
              <span className="title-line">
                <h1>{app.name}</h1>
                <StatusBadge status={appReleaseLabel(app)} />
              </span>
              <p>
                {app.description ?? "No description yet"} · App ID: {app.id}
              </p>
            </div>
          </div>
        </div>
      </div>
      <div className="detail-layout">
        <nav className="app-subnav" aria-label="App navigation">
          {detailTabs.map(({ label, icon: Icon }) => (
            <button
              className={activeTab === label ? "is-active" : ""}
              key={label}
              onClick={() => onTabChange(label)}
            >
              <Icon size={16} />
              <span>{label}</span>
            </button>
          ))}
        </nav>
        <div className="detail-content">
          {activeTab === "Integration" && <IntegrationReadinessView key={`${app.id}-${app.revision}`} appId={app.id} onNavigate={(section) => onTabChange(section === "home" ? "Home" : (Object.entries(tabRoutes).find(([, route]) => route === section)?.[0] as DetailTab) ?? "Home")} />}
          {activeTab === "Plans & pricing" && <AppPlansView key={app.id} appId={app.id} />}
          {activeTab === "Home" && (
            <AppHome
              key={`${app.id}-${app.activeVersionId}`}
              app={app}
              onNavigate={onTabChange}
              notify={notify}
            />
          )}
          {activeTab === "Versions" && (
            <VersionsView
              key={app.id}
              app={app}
              notify={notify}
              refreshApps={refreshApps}
            />
          )}
          {activeTab === "Extensions" && (
            <ExtensionsView key={app.id} app={app} notify={notify} />
          )}
          {activeTab === "API access" && (
            <ApiAccessView key={app.id} app={app} notify={notify} />
          )}
          {activeTab === "Webhooks" && (
            <WebhooksView
              key={app.id}
              app={app}
              notify={notify}
              canDispatch={canDispatchWebhooks}
            />
          )}
          {activeTab === "Settings" && (
            <AppSettingsView
              key={`${app.id}-${app.revision}`}
              app={app}
              notify={notify}
              updateApp={updateApp}
              archiveApp={archiveApp}
              onArchived={onArchived}
            />
          )}
        </div>
      </div>
    </main>
  );
}

function DetailHeading({
  eyebrow,
  title,
  description,
  action,
}: {
  eyebrow: string;
  title: string;
  description: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="detail-heading">
      <div>
        <p className="section-kicker">{eyebrow}</p>
        <h2>{title}</h2>
        <p>{description}</p>
      </div>
      {action}
    </div>
  );
}

function AppHome({
  app,
  onNavigate,
  notify,
}: {
  app: App;
  onNavigate: (tab: DetailTab) => void;
  notify: (message: string, tone?: "success" | "info" | "warning") => void;
}) {
  const [activeVersion, setActiveVersion] = useState<AppVersion | null>(null);
  const [versionLoading, setVersionLoading] = useState(
    Boolean(app.activeVersionId),
  );
  const [installationCount, setInstallationCount] = useState<number | null>(
    null,
  );
  const [testRequests, setTestRequests] = useState<DevelopmentInstallRequest[]>([]);
  const [testRequestsLoading, setTestRequestsLoading] = useState(true);
  const [testRequestOpen, setTestRequestOpen] = useState(false);
  const [requestingTestInstall, setRequestingTestInstall] = useState(false);
  const [requestListTimestamp] = useState(() => Date.now());

  useEffect(() => {
    if (!app.activeVersionId) return;
    const controller = new AbortController();
    appPlatformClient
      .getVersion(app.id, app.activeVersionId, controller.signal)
      .then(setActiveVersion)
      .catch((loadError) => {
        if (!(
          loadError instanceof DOMException && loadError.name === "AbortError"
        ))
          setActiveVersion(null);
      })
      .finally(() => {
        if (!controller.signal.aborted) setVersionLoading(false);
      });
    return () => controller.abort();
  }, [app.activeVersionId, app.id]);

  useEffect(() => {
    const controller = new AbortController();
    appPlatformClient
      .listInstallations(app.id, controller.signal)
      .then((response) =>
        setInstallationCount(
          response.data.filter(
            (installation) => installation.status !== "uninstalled",
          ).length,
        ),
      )
      .catch(() => setInstallationCount(null));
    return () => controller.abort();
  }, [app.id]);

  useEffect(() => {
    const controller = new AbortController();
    appPlatformClient
      .listDevelopmentInstallRequests(app.id, controller.signal)
      .then((response) => setTestRequests(response.data))
      .catch(() => setTestRequests([]))
      .finally(() => {
        if (!controller.signal.aborted) setTestRequestsLoading(false);
      });
    return () => controller.abort();
  }, [app.id]);

  const requestTestInstall = async (merchantId: string) => {
    setRequestingTestInstall(true);
    try {
      const request = await appPlatformClient.createDevelopmentInstallRequest(
        app.id,
        { merchantId },
      );
      setTestRequests((current) => [request, ...current]);
      notify(`Test install link created for ${request.merchantName}.`);
      return request;
    } finally {
      setRequestingTestInstall(false);
    }
  };

  const snapshot = activeVersion?.snapshot;
  const canSendDevelopmentInstall =
    app.releaseStatus === "development" && Boolean(app.activeVersionId);
  return (
    <div className="detail-stack">
      <DetailHeading
        eyebrow="App overview"
        title="Home"
        description="Status, configuration, and available actions for this app."
      />
      <Panel className="release-banner">
        <div className="release-icon">
          <Rocket size={21} />
        </div>
        <div>
          <span className="release-label">Active version</span>
          <h3>
            {versionLoading
              ? "Loading release…"
              : activeVersion
                ? `Version ${activeVersion.version}`
                : "No active release"}
          </h3>
          <p>
            {activeVersion?.releasedAt
              ? `Released ${formatDate(activeVersion.releasedAt)}`
              : "Release a draft version when this app is ready."}
          </p>
        </div>
        <StatusBadge status={activeVersion ? "Active" : "Draft"} />
        <button
          className="secondary-button"
          onClick={() => onNavigate("Versions")}
        >
          {activeVersion ? "View version" : "Create version"}{" "}
          <ChevronRight size={14} />
        </button>
      </Panel>
      <div className="summary-grid">
        <SummaryCard
          icon={Store}
          label="Installations"
          value={installationCount === null ? "…" : String(installationCount)}
          note="Merchant connections"
        />
        <SummaryCard
          icon={Boxes}
          label="Extensions"
          value={snapshot ? String(snapshot.extensions.length) : "0"}
          note="Active snapshot"
        />
        <SummaryCard
          icon={ShieldCheck}
          label="Scopes"
          value={snapshot ? String(snapshot.scopes.length) : "0"}
          note="Active snapshot"
        />
        <SummaryCard
          icon={Webhook}
          label="Webhooks"
          value={snapshot ? String(snapshot.webhookSubscriptions.length) : "0"}
          note="Active snapshot"
        />
      </div>
      <div className="two-column">
        <Panel>
          <div className="panel-heading">
            <div>
              <p className="section-kicker">Configuration</p>
              <h2>App details</h2>
            </div>
            <button
              className="text-button"
              onClick={() => onNavigate("Settings")}
            >
              Edit
            </button>
          </div>
          <InfoRows
            rows={[
              ["App ID", app.id],
              ["Status", appReleaseLabel(app)],
              ["Distribution", "Custom / invite-only"],
              ["Created", formatDate(app.createdAt)],
            ]}
          />
        </Panel>
        <Panel>
          <div className="panel-heading">
            <div>
              <p className="section-kicker">Available actions</p>
              <h2>Development workflow</h2>
            </div>
          </div>
          <div className="action-grid">
            <button onClick={() => onNavigate("Versions")}>
              <PackageCheck size={17} />
              <span>
                <strong>Create version</strong>
                <small>Bundle configuration</small>
              </span>
            </button>
            <button
              onClick={() => setTestRequestOpen(true)}
              disabled={!canSendDevelopmentInstall}
            >
              <Store size={17} />
              <span>
                <strong>Send test install</strong>
                <small>
                  {app.releaseStatus === "released"
                    ? "Install through App Store"
                    : app.activeVersionId
                    ? "Enter an approved Merchant ID"
                    : "Activate a version first"}
                </small>
              </span>
            </button>
          </div>
        </Panel>
      </div>
      <Panel className="development-install-panel">
        <div className="panel-heading">
          <div>
            <p className="section-kicker">Pre-release testing</p>
            <h2>Development install requests</h2>
            <p>
              {app.releaseStatus === "released"
                ? "This app is Released. New installations now start from the Emisell App Store."
                : "Enter a Merchant ID; merchant consent and the provider OAuth PKCE flow remain required."}
            </p>
          </div>
          <button
            className="secondary-button"
            onClick={() => setTestRequestOpen(true)}
            disabled={!canSendDevelopmentInstall}
            title={
              app.releaseStatus === "released"
                ? "Released apps are installed through the App Store"
                : undefined
            }
          >
            <Plus size={14} /> New request
          </button>
        </div>
        {testRequestsLoading ? (
          <div className="development-install-empty">
            <LoaderCircle className="spin" size={16} /> Loading test requests…
          </div>
        ) : testRequests.length ? (
          <div className="development-install-list">
            {testRequests.slice(0, 5).map((request) => {
              const expired =
                request.status === "pending" &&
                new Date(request.expiresAt).getTime() <= requestListTimestamp;
              return (
                <div key={request.id}>
                  <span className="development-install-merchant">
                    <strong>{request.merchantName}</strong>
                    <small>{request.merchantDomain ?? request.merchantId}</small>
                  </span>
                  <span>
                    <StatusBadge
                      status={expired ? "Expired" : titleCase(request.status)}
                    />
                    <small>Expires {formatDate(request.expiresAt)}</small>
                  </span>
                  <button
                    className="secondary-button compact-button"
                    disabled={request.status !== "pending" || expired}
                    onClick={() => void navigator.clipboard.writeText(request.launchUrl)}
                  >
                    <Copy size={13} /> Copy link
                  </button>
                </div>
              );
            })}
          </div>
        ) : (
          <div className="development-install-empty">
            <Store size={17} /> No development install requests yet.
          </div>
        )}
      </Panel>
      {testRequestOpen && canSendDevelopmentInstall ? (
        <DevelopmentInstallRequestModal
          app={app}
          saving={requestingTestInstall}
          onClose={() => setTestRequestOpen(false)}
          onCreate={requestTestInstall}
        />
      ) : null}
    </div>
  );
}

function DevelopmentInstallRequestModal({
  app,
  saving,
  onClose,
  onCreate,
}: {
  app: App;
  saving: boolean;
  onClose: () => void;
  onCreate: (merchantId: string) => Promise<DevelopmentInstallRequest>;
}) {
  const [merchantId, setMerchantId] = useState("");
  const [created, setCreated] = useState<DevelopmentInstallRequest | null>(null);
  const [copied, setCopied] = useState(false);
  const [error, setError] = useState("");
  const submit = async () => {
    const value = merchantId.trim();
    if (!/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(value)) {
      setError("Enter a valid Emisell Merchant ID.");
      return;
    }
    setError("");
    try {
      setCreated(await onCreate(value));
    } catch (saveError) {
      setError(apiErrorMessage(saveError));
    }
  };
  const copy = async () => {
    if (!created) return;
    await navigator.clipboard.writeText(created.launchUrl);
    setCopied(true);
  };
  return (
    <div className="modal-backdrop" role="presentation" onMouseDown={() => !saving && onClose()}>
      <section
        className="modal development-install-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="development-install-title"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="modal-head">
          <div>
            <p className="section-kicker">Development test</p>
            <h2 id="development-install-title">
              {created ? "Test install link ready" : `Send ${app.name} for testing`}
            </h2>
            <p>
              {created
                ? "Share this link with the selected merchant. It expires automatically."
                : "Enter the Merchant ID registered in Emisell. The app must still be in Development status."}
            </p>
          </div>
          <button className="icon-button" onClick={onClose} aria-label="Close" disabled={saving}>
            <X size={17} />
          </button>
        </div>
        <div className="modal-body">
          {created ? (
            <>
              <div className="test-install-success">
                <CheckCircle2 size={20} />
                <span>
                  <strong>{created.merchantName}</strong>
                  <small>{created.merchantDomain ?? created.merchantId}</small>
                </span>
                <StatusBadge status="Pending consent" />
              </div>
              <label>
                Development launch link
                <div className="copy-field">
                  <input value={created.launchUrl} readOnly />
                  <button className="secondary-button" onClick={() => void copy()}>
                    {copied ? <Check size={14} /> : <Copy size={14} />}
                    {copied ? "Copied" : "Copy"}
                  </button>
                </div>
              </label>
              <InlineNotice title="Provider handoff required">
                The provider app must preserve <code>emisell_test_install_request</code>
                and send it back as <code>test_install_request</code> when starting
                the normal Emisell OAuth authorization flow.
              </InlineNotice>
              <div className="test-install-steps">
                <span><b>1</b> Merchant opens the provider link.</span>
                <span><b>2</b> Provider starts OAuth with PKCE S256.</span>
                <span><b>3</b> Merchant reviews scopes and approves.</span>
              </div>
            </>
          ) : (
            <>
              <label>
                Merchant ID
                <input
                  autoFocus
                  value={merchantId}
                  onChange={(event) => {
                    setMerchantId(event.target.value);
                    setError("");
                  }}
                  placeholder="cmmerchantdemo000000000001"
                  autoComplete="off"
                  spellCheck={false}
                  disabled={saving}
                />
                {error ? <span className="field-error">{error}</span> : null}
              </label>
              <InlineNotice title="No forced installation">
                Merchant ID identifies the intended merchant. It does not authorize
                access; the matching merchant session and consent are mandatory.
              </InlineNotice>
            </>
          )}
        </div>
        <div className="modal-foot">
          <button className="secondary-button" onClick={onClose} disabled={saving}>
            {created ? "Done" : "Cancel"}
          </button>
          {!created ? (
            <button className="primary-button" onClick={() => void submit()} disabled={saving}>
              {saving ? <LoaderCircle className="spin" size={15} /> : <Rocket size={15} />}
              {saving ? "Creating…" : "Create test link"}
            </button>
          ) : null}
        </div>
      </section>
    </div>
  );
}

function SummaryCard({
  icon: Icon,
  label,
  value,
  note,
}: {
  icon: typeof Store;
  label: string;
  value: string;
  note: string;
}) {
  return (
    <Panel className="summary-card">
      <span>
        <Icon size={17} />
      </span>
      <p>{label}</p>
      <strong>{value}</strong>
      <small>{note}</small>
    </Panel>
  );
}

function InfoRows({ rows }: { rows: string[][] }) {
  return (
    <div className="info-rows">
      {rows.map(([label, value]) => (
        <div key={label}>
          <span>{label}</span>
          <strong>{value}</strong>
        </div>
      ))}
    </div>
  );
}

function VersionsView({
  app,
  notify,
  refreshApps,
}: {
  app: App;
  notify: (message: string, tone?: "success" | "info" | "warning") => void;
  refreshApps: () => Promise<void>;
}) {
  const [versions, setVersions] = useState<AppVersion[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [pending, setPending] = useState<{
    item: AppVersion;
    action: "release" | "rollback";
  } | null>(null);
  const [working, setWorking] = useState(false);
  const active =
    versions.find((version) => version.status === "active") ?? null;

  const loadVersions = async () => {
    setError(null);
    const response = await appPlatformClient.listVersions(app.id);
    setVersions(response.data);
  };

  useEffect(() => {
    const controller = new AbortController();
    appPlatformClient
      .listVersions(app.id, controller.signal)
      .then((response) => setVersions(response.data))
      .catch((loadError) => {
        if (!(
          loadError instanceof DOMException && loadError.name === "AbortError"
        ))
          setError(apiErrorMessage(loadError));
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [app.id]);

  const applyVersionAction = async () => {
    if (!pending) return;
    setWorking(true);
    try {
      if (pending.action === "release") {
        await appPlatformClient.releaseVersion(app.id, pending.item.id, {
          expectedActiveVersionId: app.activeVersionId,
        });
      } else {
        await appPlatformClient.rollbackVersion(app.id, pending.item.id);
      }
      await Promise.all([loadVersions(), refreshApps()]);
      notify(`Version ${pending.item.version} is now active.`);
      setPending(null);
    } catch (actionError) {
      const message = apiErrorMessage(actionError);
      setError(message);
      notify(message, "warning");
    } finally {
      setWorking(false);
    }
  };

  const createVersion = async (version: string, note: string) => {
    const created = await appPlatformClient.createVersion(app.id, {
      version,
      releaseNote: note,
    });
    setVersions((current) => [
      created,
      ...current.filter((item) => item.id !== created.id),
    ]);
    setCreateOpen(false);
    notify(`Draft version ${version} created.`);
  };

  return (
    <div className="detail-stack">
      <DetailHeading
        eyebrow="Release management"
        title="Versions"
        description="Create immutable snapshots of app configuration and extensions."
        action={
          <button
            className="primary-button"
            onClick={() => setCreateOpen(true)}
          >
            <Plus size={16} />
            Create version
          </button>
        }
      />
      {error ? (
        <InlineNotice tone="warning" title="App Gateway request failed">
          {error}
        </InlineNotice>
      ) : null}
      <Panel className="version-highlight">
        <div>
          <p className="section-kicker">Current release</p>
          <span className="version-title">
            <h3>
              {loading
                ? "Loading…"
                : active
                  ? `Version ${active.version}`
                  : "No active release"}
            </h3>
            <StatusBadge status={active ? "Active" : "Draft"} />
          </span>
          <p>
            {active?.releasedAt
              ? `Released ${formatDate(active.releasedAt)}`
              : "Release a draft when the configuration is ready."}
          </p>
        </div>
        <div className="version-summary">
          <span>
            <Boxes size={15} />
            {active?.snapshot.extensions.length ?? 0} extensions
          </span>
          <span>
            <ShieldCheck size={15} />
            {active?.snapshot.scopes.length ?? 0} scopes
          </span>
          <span>
            <Webhook size={15} />
            {active?.snapshot.webhookSubscriptions.length ?? 0} subscriptions
          </span>
        </div>
      </Panel>
      <Panel>
        <div className="panel-heading">
          <div>
            <p className="section-kicker">Release history</p>
            <h2>All versions</h2>
          </div>
          <button className="filter-button" onClick={() => void loadVersions()}>
            <RefreshCw size={14} />
            Refresh
          </button>
        </div>
        <div className="version-list">
          {loading ? (
            <div className="empty-state">
              <LoaderCircle className="spin" size={22} />
              <strong>Loading versions</strong>
            </div>
          ) : versions.length ? (
            versions.map((item) => (
              <VersionRow
                key={item.id}
                item={item}
                onAction={() =>
                  item.status === "draft"
                    ? setPending({ item, action: "release" })
                    : item.status === "released"
                      ? setPending({ item, action: "rollback" })
                      : notify(
                          `Version ${item.version} is the active release.`,
                          "info",
                        )
                }
              />
            ))
          ) : (
            <div className="empty-state">
              <FileCode2 size={22} />
              <strong>No versions yet</strong>
              <p>
                Create an immutable configuration snapshot to start release
                management.
              </p>
            </div>
          )}
        </div>
      </Panel>
      {createOpen ? (
        <CreateVersionModal
          existing={versions.map((version) => version.version)}
          onClose={() => setCreateOpen(false)}
          onCreate={createVersion}
        />
      ) : null}
      <ConfirmDialog
        open={Boolean(pending)}
        title={
          pending?.action === "release"
            ? `Release version ${pending.item.version}?`
            : `Roll back to version ${pending?.item.version}?`
        }
        description={
          pending?.action === "release"
            ? "This snapshot will become active for all new installations. The current version remains available for rollback."
            : "This previously released snapshot will become active again. No app data will be deleted."
        }
        confirmLabel={
          pending?.action === "release" ? "Release version" : "Roll back"
        }
        loading={working}
        onClose={() => setPending(null)}
        onConfirm={() => void applyVersionAction()}
      />
    </div>
  );
}

function VersionRow({
  item,
  onAction,
}: {
  item: AppVersion;
  onAction: () => void;
}) {
  const date = item.releasedAt
    ? `Released ${formatDate(item.releasedAt)}`
    : `Created ${formatRelativeDate(item.createdAt)}`;
  const actor = (item.releasedBy ?? item.createdBy).slice(0, 8);
  return (
    <div className="version-row">
      <span className="version-node">
        <FileCode2 size={15} />
      </span>
      <span className="version-main">
        <span>
          <strong>Version {item.version}</strong>
          <StatusBadge status={titleCase(item.status)} />
        </span>
        <small>{item.releaseNote ?? "Configuration snapshot"}</small>
      </span>
      <span className="version-meta">
        <strong>{date}</strong>
        <small>by {actor}</small>
      </span>
      <button className="row-action" onClick={onAction}>
        {item.status === "draft"
          ? "Release"
          : item.status === "released"
            ? "Roll back"
            : "View"}
        <ChevronRight size={13} />
      </button>
    </div>
  );
}

function CreateVersionModal({
  existing,
  onClose,
  onCreate,
}: {
  existing: string[];
  onClose: () => void;
  onCreate: (version: string, note: string) => Promise<void>;
}) {
  const [version, setVersion] = useState("1.0.0");
  const [note, setNote] = useState("Initial configuration snapshot");
  const [error, setError] = useState("");
  const [creating, setCreating] = useState(false);
  const submit = async () => {
    if (!/^\d+\.\d+\.\d+$/.test(version))
      return setError("Use semantic version format, for example 1.0.0.");
    if (existing.includes(version))
      return setError("This version already exists.");
    setCreating(true);
    setError("");
    try {
      await onCreate(version, note.trim() || "Configuration update");
    } catch (createError) {
      setError(apiErrorMessage(createError));
      setCreating(false);
    }
  };
  return (
    <div
      className="modal-backdrop"
      role="presentation"
      onMouseDown={() => {
        if (!creating) onClose();
      }}
    >
      <section
        className="modal compact-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="version-title"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="modal-head">
          <div>
            <p className="section-kicker">Configuration snapshot</p>
            <h2 id="version-title">Create version</h2>
            <p>
              Bundle the current extensions, scopes, and webhook subscriptions.
            </p>
          </div>
          <button
            className="icon-button"
            onClick={onClose}
            aria-label="Close"
            disabled={creating}
          >
            <X size={17} />
          </button>
        </div>
        <div className="modal-body">
          <label>
            Version number
            <input
              autoFocus
              value={version}
              onChange={(event) => {
                setVersion(event.target.value);
                setError("");
              }}
              aria-invalid={Boolean(error)}
              disabled={creating}
            />
            {error ? <span className="field-error">{error}</span> : null}
          </label>
          <label>
            Release note
            <input
              value={note}
              onChange={(event) => setNote(event.target.value)}
              disabled={creating}
            />
          </label>
          <InlineNotice title="Draft first">
            New versions start as drafts. You decide when to release them.
          </InlineNotice>
        </div>
        <div className="modal-foot">
          <button
            className="secondary-button"
            onClick={onClose}
            disabled={creating}
          >
            Cancel
          </button>
          <button
            className="primary-button"
            onClick={() => void submit()}
            disabled={creating}
          >
            {creating ? <LoaderCircle className="spin" size={15} /> : null}
            {creating ? "Creating…" : "Create draft"}
          </button>
        </div>
      </section>
    </div>
  );
}

function extensionTone(type: ExtensionType) {
  return type === "shipping" ? "blue" : type === "payment" ? "violet" : "amber";
}

function ExtensionsView({
  app,
  notify,
}: {
  app: App;
  notify: (message: string, tone?: "success" | "info" | "warning") => void;
}) {
  const [extensions, setExtensions] = useState<AppExtension[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [addOpen, setAddOpen] = useState(false);
  const [selected, setSelected] = useState<AppExtension | null>(null);
  const [disableTarget, setDisableTarget] = useState<AppExtension | null>(null);
  const [disabling, setDisabling] = useState(false);

  const loadExtensions = async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await appPlatformClient.listExtensions(app.id);
      setExtensions(response.data);
    } catch (loadError) {
      setError(apiErrorMessage(loadError));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    const controller = new AbortController();
    appPlatformClient
      .listExtensions(app.id, controller.signal)
      .then((response) => setExtensions(response.data))
      .catch((loadError) => {
        if (!(
          loadError instanceof DOMException && loadError.name === "AbortError"
        ))
          setError(apiErrorMessage(loadError));
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [app.id]);

  const addExtension = async (
    name: string,
    type: ExtensionType,
    runtimeUrl: string,
  ) => {
    const extension = await appPlatformClient.createExtension(app.id, {
      name,
      type,
      ...(runtimeUrl ? { runtimeUrl } : {}),
      configuration: {},
    });
    setExtensions((current) => [extension, ...current]);
    setAddOpen(false);
    notify(`${name} added as a draft extension.`);
  };

  const updateExtension = async (
    extension: AppExtension,
    name: string,
    runtimeUrl: string,
    configuration: Record<string, unknown>,
  ) => {
    const updated = await appPlatformClient.updateExtension(
      app.id,
      extension.id,
      {
        name,
        runtimeUrl: runtimeUrl || null,
        configuration,
        revision: extension.revision,
      },
    );
    setExtensions((current) =>
      current.map((item) => (item.id === updated.id ? updated : item)),
    );
    setSelected(null);
    notify(`${updated.name} configuration saved as a draft.`);
  };

  const disableExtension = async () => {
    if (!disableTarget) return;
    setDisabling(true);
    try {
      await appPlatformClient.disableExtension(app.id, disableTarget.id);
      setExtensions((current) =>
        current.map((item) =>
          item.id === disableTarget.id
            ? {
                ...item,
                status: "disabled",
                revision: item.revision + 1,
                updatedAt: new Date().toISOString(),
              }
            : item,
        ),
      );
      notify(`${disableTarget.name} disabled for the next version.`, "warning");
      setDisableTarget(null);
    } catch (disableError) {
      const message = apiErrorMessage(disableError);
      setError(message);
      notify(message, "warning");
    } finally {
      setDisabling(false);
    }
  };

  return (
    <div className="detail-stack">
      <DetailHeading
        eyebrow="Versioned configuration"
        title="Extensions"
        description="Extension configurations bundled with your app. A released configuration is not proof that its runtime is connected."
        action={
          <button
            className="primary-button"
            onClick={() => setAddOpen(true)}
            disabled={app.status === "archived"}
          >
            <Plus size={16} />
            Add extension
          </button>
        }
      />
      {error ? (
        <InlineNotice tone="warning" title="Could not load extensions">
          {error}
          <button className="secondary-button" onClick={() => void loadExtensions()}>Retry</button>
        </InlineNotice>
      ) : null}
      {loading ? (
        <Panel className="resource-state compact-resource">
          <LoaderCircle className="spin" size={22} />
          <strong>Loading extensions…</strong>
        </Panel>
      ) : extensions.length ? (
        <div className="extension-grid">
          {extensions.map((extension) => (
            <ExtensionCard
              key={extension.id}
              extension={extension}
              onConfigure={() => setSelected(extension)}
            />
          ))}
        </div>
      ) : (
        <Panel className="resource-state compact-resource">
          <Boxes size={23} />
          <strong>No extensions yet</strong>
          <p>
            Add a Payment, Shipping, or General extension configuration.
            Review the catalog below for actual runtime support.
          </p>
          <button className="primary-button" onClick={() => setAddOpen(true)}>
            <Plus size={14} />
            Add extension
          </button>
        </Panel>
      )}
      <ExtensionCatalogPanel />
      {addOpen ? (
        <AddExtensionModal
          onClose={() => setAddOpen(false)}
          onAdd={addExtension}
        />
      ) : null}
      {selected ? (
        <ConfigureExtensionModal
          extension={selected}
          onClose={() => setSelected(null)}
          onSave={updateExtension}
          onDisable={(extension) => {
            setSelected(null);
            setDisableTarget(extension);
          }}
        />
      ) : null}
      <ConfirmDialog
        open={Boolean(disableTarget)}
        eyebrow="Working configuration"
        title={`Disable ${disableTarget?.name ?? "extension"}?`}
        description="It will be excluded from newly created versions. Existing immutable releases are not modified."
        confirmLabel="Disable extension"
        tone="danger"
        loading={disabling}
        onClose={() => setDisableTarget(null)}
        onConfirm={() => void disableExtension()}
      />
    </div>
  );
}

function ExtensionCard({
  extension,
  onConfigure,
}: {
  extension: AppExtension;
  onConfigure: () => void;
}) {
  const Icon =
    extension.type === "payment"
      ? CreditCard
      : extension.type === "shipping"
        ? Truck
        : Boxes;
  return (
    <Panel className="extension-card">
      <div className="extension-top">
        <span className={`extension-icon ${extensionTone(extension.type)}`}>
          <Icon size={21} />
        </span>
        <StatusBadge status={titleCase(extension.status)} />
      </div>
      <h3>{extension.name}</h3>
      <p>{extension.type === 'custom' ? 'General' : titleCase(extension.type)} extension · configuration</p>
      <div className="extension-meta">
        <span>
          <small>Working revision</small>
          <strong>r{extension.revision}</strong>
        </span>
        <span>
          <small>Runtime</small>
          <strong>
            {extension.runtimeUrl ? "URL saved · not verified" : "Not configured"}
          </strong>
        </span>
      </div>
      <button className="secondary-button" onClick={onConfigure}>
        Configure <ChevronRight size={14} />
      </button>
    </Panel>
  );
}

function AddExtensionModal({
  onClose,
  onAdd,
}: {
  onClose: () => void;
  onAdd: (
    name: string,
    type: ExtensionType,
    runtimeUrl: string,
  ) => Promise<void>;
}) {
  const { catalog, error: catalogError, reload: reloadCatalog } = useExtensionCatalog();
  const [type, setType] = useState<ExtensionType>("custom");
  const [name, setName] = useState("");
  const [runtimeUrl, setRuntimeUrl] = useState("");
  const [error, setError] = useState("");
  const [creating, setCreating] = useState(false);
  const submit = async () => {
    if (!catalog?.families.some(family => family.type === type && family.configurationSupported)) return setError("Load the official extension catalog before continuing.");
    if (name.trim().length < 3) return setError("Enter an extension name.");
    if (runtimeUrl && !runtimeUrl.startsWith("https://"))
      return setError("Runtime URL must use HTTPS.");
    setCreating(true);
    try {
      await onAdd(name.trim(), type, runtimeUrl.trim());
    } catch (createError) {
      setError(apiErrorMessage(createError));
      setCreating(false);
    }
  };
  return (
    <div
      className="modal-backdrop"
      role="presentation"
      onMouseDown={() => {
        if (!creating) onClose();
      }}
    >
      <section
        className="modal compact-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="extension-title"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="modal-head">
          <div>
            <p className="section-kicker">New configuration</p>
            <h2 id="extension-title">Add extension</h2>
            <p>Extensions are bundled into the next app version.</p>
          </div>
          <button
            className="icon-button"
            onClick={onClose}
            aria-label="Close"
            disabled={creating}
          >
            <X size={17} />
          </button>
        </div>
        <div className="modal-body">
          <label>
            Extension name
            <input
              autoFocus
              placeholder="e.g. Merchant Data Integration"
              value={name}
              onChange={(event) => {
                setName(event.target.value);
                setError("");
              }}
              disabled={creating}
            />
          </label>
          <label>
            Extension family · not app category
            <select
              value={type}
              onChange={(event) => setType(event.target.value as ExtensionType)}
              disabled={creating || !catalog || Boolean(catalogError)}
            >
              {!catalog ? <option value="custom">Loading catalog…</option> : catalog.families.filter(family => family.configurationSupported).map(family => <option key={family.type} value={family.type}>{family.name}</option>)}
            </select>
          </label>
          {catalogError ? <div role="alert"><p className="field-error">{catalogError}</p><button className="secondary-button" onClick={reloadCatalog}>Reload catalog</button></div> : <p className="field-hint">{catalog?.families.find(family => family.type === type)?.description}</p>}
          <label>
            Runtime URL <small className="optional-label">Optional</small>
            <input
              placeholder="https://api.example.com/extensions/run"
              value={runtimeUrl}
              onChange={(event) => {
                setRuntimeUrl(event.target.value);
                setError("");
              }}
              disabled={creating}
            />
            {error ? <span className="field-error">{error}</span> : null}
          </label>
          <InlineNotice title="Versioned configuration">
            This extension remains a draft until included in a released app
            version. Saving or releasing it does not grant scopes, enable
            Payment/Shipping execution, or add UI to a merchant page.
          </InlineNotice>
        </div>
        <div className="modal-foot">
          <button
            className="secondary-button"
            onClick={onClose}
            disabled={creating}
          >
            Cancel
          </button>
          <button
            className="primary-button"
            onClick={() => void submit()}
            disabled={creating}
          >
            {creating ? <LoaderCircle className="spin" size={15} /> : null}
            {creating ? "Adding…" : "Add extension"}
          </button>
        </div>
      </section>
    </div>
  );
}

function ConfigureExtensionModal({
  extension,
  onClose,
  onSave,
  onDisable,
}: {
  extension: AppExtension;
  onClose: () => void;
  onSave: (
    extension: AppExtension,
    name: string,
    runtimeUrl: string,
    configuration: Record<string, unknown>,
  ) => Promise<void>;
  onDisable: (extension: AppExtension) => void;
}) {
  const [name, setName] = useState(extension.name);
  const [runtimeUrl, setRuntimeUrl] = useState(extension.runtimeUrl ?? "");
  const [configuration, setConfiguration] = useState(
    JSON.stringify(extension.configuration, null, 2),
  );
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const submit = async () => {
    if (name.trim().length < 3) return setError("Enter an extension name.");
    if (runtimeUrl && !runtimeUrl.startsWith("https://"))
      return setError("Runtime URL must use HTTPS.");
    let parsed: Record<string, unknown>;
    try {
      parsed = JSON.parse(configuration) as Record<string, unknown>;
      if (!parsed || Array.isArray(parsed) || typeof parsed !== "object")
        throw new Error();
    } catch {
      return setError("Configuration must be a valid JSON object.");
    }
    setSaving(true);
    try {
      await onSave(extension, name.trim(), runtimeUrl.trim(), parsed);
    } catch (saveError) {
      setError(apiErrorMessage(saveError));
      setSaving(false);
    }
  };
  return (
    <div
      className="modal-backdrop"
      role="presentation"
      onMouseDown={() => {
        if (!saving) onClose();
      }}
    >
      <section
        className="modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="configure-extension-title"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="modal-head">
          <div>
            <p className="section-kicker">
              {titleCase(extension.type)} extension · revision{" "}
              {extension.revision}
            </p>
            <h2 id="configure-extension-title">Configure extension</h2>
            <p>Changes return this working configuration to draft status.</p>
          </div>
          <button
            className="icon-button"
            onClick={onClose}
            aria-label="Close"
            disabled={saving}
          >
            <X size={17} />
          </button>
        </div>
        <div className="modal-body">
          <label>
            Extension name
            <input
              value={name}
              onChange={(event) => {
                setName(event.target.value);
                setError("");
              }}
              disabled={saving}
            />
          </label>
          <label>
            Runtime URL
            <input
              placeholder="https://api.example.com/extensions/run"
              value={runtimeUrl}
              onChange={(event) => {
                setRuntimeUrl(event.target.value);
                setError("");
              }}
              disabled={saving}
            />
          </label>
          <label>
            Configuration JSON
            <textarea
              rows={6}
              value={configuration}
              onChange={(event) => {
                setConfiguration(event.target.value);
                setError("");
              }}
              disabled={saving}
              spellCheck={false}
            />
            {error ? <span className="field-error">{error}</span> : null}
          </label>
        </div>
        <div className="modal-foot modal-foot-split">
          <button
            className="danger-text-button"
            onClick={() => onDisable(extension)}
            disabled={saving || extension.status === "disabled"}
          >
            {extension.status === "disabled"
              ? "Extension disabled"
              : "Disable extension"}
          </button>
          <span />
          <button
            className="secondary-button"
            onClick={onClose}
            disabled={saving}
          >
            Cancel
          </button>
          <button
            className="primary-button"
            onClick={() => void submit()}
            disabled={saving || extension.status === "disabled"}
          >
            {saving ? (
              <LoaderCircle className="spin" size={15} />
            ) : (
              <Check size={15} />
            )}
            {saving ? "Saving…" : "Save draft"}
          </button>
        </div>
      </section>
    </div>
  );
}

const SCOPE_DESCRIPTIONS: Record<string, string> = {
  read_merchant: "View the connected merchant identity",
  read_products: "View products and variants",
  write_products: "Create and update products and variants",
  read_orders: "View orders and their line items",
  write_orders: "Create and update orders",
  read_customers: "View customer profiles",
  write_customers: "Create and update customer profiles",
  read_inventory: "View inventory levels",
  write_inventory: "Adjust inventory quantities",
  read_fulfillments: "View fulfillment and shipment status",
  write_fulfillments: "Create fulfillment updates",
};

function scopeDescription(scope: string) {
  return SCOPE_DESCRIPTIONS[scope] ?? "Registered Emisell permission";
}

function ApiAccessView({
  app,
  notify,
}: {
  app: App;
  notify: (message: string, tone?: "success" | "info" | "warning") => void;
}) {
  const [credentials, setCredentials] = useState<AppCredential[]>([]);
  const [credentialsLoading, setCredentialsLoading] = useState(true);
  const [credentialError, setCredentialError] = useState<string | null>(null);
  const [createCredentialOpen, setCreateCredentialOpen] = useState(false);
  const [credentialSaving, setCredentialSaving] = useState(false);
  const [credentialAction, setCredentialAction] = useState<{
    credential: AppCredential;
    action: "rotate" | "revoke";
  } | null>(null);
  const [revealedSecret, setRevealedSecret] = useState<{
    title: string;
    identifier: string;
    secret: string;
    label: string;
  } | null>(null);
  const [scopeModal, setScopeModal] = useState<"Required" | "Optional" | null>(
    null,
  );
  const [scopes, setScopes] = useState<VersionedScope[]>([]);
  const [scopeCatalog, setScopeCatalog] = useState<ScopeDefinition[]>([]);
  const [scopesLoading, setScopesLoading] = useState(true);
  const [scopesSaving, setScopesSaving] = useState(false);
  const [scopeError, setScopeError] = useState<string | null>(null);
  const requiredScopes = scopes.filter((scope) => scope.access === "required");
  const optionalScopes = scopes.filter((scope) => scope.access === "optional");

  useEffect(() => {
    const controller = new AbortController();
    appPlatformClient
      .listCredentials(app.id, controller.signal)
      .then((response) => setCredentials(response.data))
      .catch((loadError) => {
        if (!(
          loadError instanceof DOMException && loadError.name === "AbortError"
        ))
          setCredentialError(apiErrorMessage(loadError));
      })
      .finally(() => {
        if (!controller.signal.aborted) setCredentialsLoading(false);
      });
    appPlatformClient
      .listScopes(app.id, controller.signal)
      .then((response) => setScopes(response.data))
      .catch((loadError) => {
        if (!(
          loadError instanceof DOMException && loadError.name === "AbortError"
        ))
          setScopeError(apiErrorMessage(loadError));
      })
      .finally(() => {
        if (!controller.signal.aborted) setScopesLoading(false);
      });
    appPlatformClient
      .listScopeCatalog(controller.signal)
      .then((response) => setScopeCatalog(response.data))
      .catch((loadError) => {
        if (!(loadError instanceof DOMException && loadError.name === "AbortError"))
          setScopeError(apiErrorMessage(loadError));
      });
    return () => controller.abort();
  }, [app.id]);

  const createCredential = async (
    environment: Environment,
    expiresAt?: string,
  ) => {
    setCredentialSaving(true);
    setCredentialError(null);
    try {
      const result = await appPlatformClient.createCredential(app.id, {
        environment,
        ...(expiresAt ? { expiresAt } : {}),
      });
      setCredentials((current) => [result.credential, ...current]);
      setCreateCredentialOpen(false);
      setRevealedSecret({
        title: "Credential created",
        identifier: result.credential.clientId,
        secret: result.clientSecret,
        label: "Client secret",
      });
      notify(`${titleCase(environment)} credential created.`);
    } catch (saveError) {
      const message = apiErrorMessage(saveError);
      setCredentialError(message);
      throw saveError;
    } finally {
      setCredentialSaving(false);
    }
  };

  const applyCredentialAction = async () => {
    if (!credentialAction) return;
    setCredentialSaving(true);
    setCredentialError(null);
    try {
      if (credentialAction.action === "rotate") {
        const result = await appPlatformClient.rotateCredential(
          app.id,
          credentialAction.credential.id,
        );
        setCredentials((current) =>
          current.map((item) =>
            item.id === result.credential.id ? result.credential : item,
          ),
        );
        setRevealedSecret({
          title: "Secret rotated",
          identifier: result.credential.clientId,
          secret: result.clientSecret,
          label: "New client secret",
        });
        notify("Client secret rotated. Save the replacement now.");
      } else {
        await appPlatformClient.revokeCredential(
          app.id,
          credentialAction.credential.id,
        );
        setCredentials((current) =>
          current.map((item) =>
            item.id === credentialAction.credential.id
              ? {
                  ...item,
                  status: "revoked",
                  revokedAt: new Date().toISOString(),
                }
              : item,
          ),
        );
        notify("Credential revoked. Existing clients can no longer use it.");
      }
      setCredentialAction(null);
    } catch (saveError) {
      const message = apiErrorMessage(saveError);
      setCredentialError(message);
      notify(message, "warning");
    } finally {
      setCredentialSaving(false);
    }
  };
  const replaceScopes = async (
    next: VersionedScope[],
    successMessage: string,
  ) => {
    setScopesSaving(true);
    setScopeError(null);
    try {
      const saved = await appPlatformClient.replaceScopes(app.id, {
        scopes: next,
      });
      setScopes(saved);
      setScopeModal(null);
      notify(successMessage);
    } catch (saveError) {
      const message = apiErrorMessage(saveError);
      setScopeError(message);
      notify(message, "warning");
      throw saveError;
    } finally {
      setScopesSaving(false);
    }
  };
  const addScope = async (name: string) => {
    if (scopes.some((scope) => scope.scope === name))
      throw new Error("This scope is already configured.");
    const access: ScopeAccess =
      scopeModal === "Required" ? "required" : "optional";
    await replaceScopes(
      [...scopes, { scope: name, access }],
      `${name} added to ${access} scopes.`,
    );
  };
  const removeScope = async (name: string) => {
    try {
      await replaceScopes(
        scopes.filter((scope) => scope.scope !== name),
        `${name} removed from the working configuration.`,
      );
    } catch {
      // replaceScopes already exposes the recoverable API error in the view.
    }
  };
  return (
    <div className="detail-stack">
      <DetailHeading
        eyebrow="Security & permissions"
        title="API access"
        description="Manage credentials and data permissions used by this app."
      />
      <Panel>
        <div className="panel-heading">
          <div>
            <p className="section-kicker">Encrypted credentials</p>
            <h2>Client credentials</h2>
            <p>
              Secrets are encrypted at rest and only shown after creation or
              rotation.
            </p>
          </div>
          <button
            className="primary-button"
            onClick={() => setCreateCredentialOpen(true)}
          >
            <Plus size={15} />
            Create credential
          </button>
        </div>
        {credentialError ? (
          <div className="panel-notice">
            <InlineNotice tone="warning" title="Credential request failed">
              {credentialError}
            </InlineNotice>
          </div>
        ) : null}
        <div className="credential-list">
          {credentialsLoading ? (
            <div className="credential-empty">
              <LoaderCircle className="spin" size={18} />
              <span>Loading credentials…</span>
            </div>
          ) : credentials.length ? (
            credentials.map((credential) => (
              <CredentialRow
                key={credential.id}
                credential={credential}
                working={credentialSaving}
                onRotate={() =>
                  setCredentialAction({ credential, action: "rotate" })
                }
                onRevoke={() =>
                  setCredentialAction({ credential, action: "revoke" })
                }
                notify={notify}
              />
            ))
          ) : (
            <div className="credential-empty">
              <KeyRound size={20} />
              <strong>No client credentials</strong>
              <span>Create a sandbox credential to connect this app.</span>
            </div>
          )}
        </div>
      </Panel>
      <InlineNotice title="Sandbox credentials only">
        Production credentials remain unavailable until Emisell enables
        production access for the organization.
      </InlineNotice>
      <InlineNotice title="Availability is enforced">
        Scope configuration is stored in PostgreSQL and versioned. Only
        read_merchant has a callable Provider API resource today; planned scopes
        remain unavailable and block App Store publication.
      </InlineNotice>
      {scopeError ? (
        <InlineNotice tone="warning" title="Could not save scopes">
          {scopeError}
        </InlineNotice>
      ) : null}
      <div className="scope-columns">
        <ScopePanel
          type="Required"
          description="Granted during every installation."
          scopes={requiredScopes}
          loading={scopesLoading}
          saving={scopesSaving}
          onAdd={() => setScopeModal("Required")}
          onRemove={removeScope}
        />
        <ScopePanel
          type="Optional"
          description="Requested only when a feature needs it."
          scopes={optionalScopes}
          loading={scopesLoading}
          saving={scopesSaving}
          onAdd={() => setScopeModal("Optional")}
          onRemove={removeScope}
        />
      </div>
      {createCredentialOpen ? (
        <CreateCredentialModal
          saving={credentialSaving}
          onClose={() => setCreateCredentialOpen(false)}
          onCreate={createCredential}
        />
      ) : null}
      <ConfirmDialog
        open={Boolean(credentialAction)}
        title={
          credentialAction?.action === "rotate"
            ? "Rotate client secret?"
            : "Revoke credential?"
        }
        description={
          credentialAction?.action === "rotate"
            ? "The current secret will stop working immediately. The replacement is shown only once."
            : "This client ID will permanently lose API access. This action cannot be undone."
        }
        confirmLabel={
          credentialAction?.action === "rotate"
            ? "Rotate secret"
            : "Revoke credential"
        }
        tone="danger"
        loading={credentialSaving}
        onClose={() => setCredentialAction(null)}
        onConfirm={() => void applyCredentialAction()}
      />
      {revealedSecret ? (
        <OneTimeSecretModal
          {...revealedSecret}
          onClose={() => setRevealedSecret(null)}
        />
      ) : null}
      {scopeModal ? (
        <AddScopeModal
          type={scopeModal}
          catalog={scopeCatalog}
          configuredScopes={scopes.map((item) => item.scope)}
          saving={scopesSaving}
          onClose={() => setScopeModal(null)}
          onAdd={addScope}
        />
      ) : null}
    </div>
  );
}

function CredentialRow({
  credential,
  working,
  onRotate,
  onRevoke,
  notify,
}: {
  credential: AppCredential;
  working: boolean;
  onRotate: () => void;
  onRevoke: () => void;
  notify: (message: string) => void;
}) {
  const copyClientId = async () => {
    await navigator.clipboard?.writeText(credential.clientId);
    notify("Client ID copied to clipboard.");
  };
  const canChange = credential.status === "active";
  return (
    <div className="credential-item">
      <span className={`credential-mark ${credential.environment}`}>
        <KeyRound size={15} />
      </span>
      <span className="credential-main">
        <span>
          <code>{credential.clientId}</code>
          <StatusBadge status={titleCase(credential.status)} />
        </span>
        <small>
          {titleCase(credential.environment)} · Fingerprint{" "}
          {credential.secretFingerprint} · Created{" "}
          {formatDate(credential.createdAt)}
        </small>
      </span>
      <div className="row-actions">
        <button
          className="icon-button"
          aria-label="Copy client ID"
          onClick={() => void copyClientId()}
        >
          <Copy size={14} />
        </button>
        <button
          className="secondary-button compact-button"
          disabled={!canChange || working}
          onClick={onRotate}
        >
          <RefreshCw size={13} />
          Rotate
        </button>
        <button
          className="icon-button danger-icon"
          aria-label="Revoke credential"
          disabled={!canChange || working}
          onClick={onRevoke}
        >
          <Trash2 size={14} />
        </button>
      </div>
    </div>
  );
}

function CreateCredentialModal({
  saving,
  onClose,
  onCreate,
}: {
  saving: boolean;
  onClose: () => void;
  onCreate: (environment: Environment, expiresAt?: string) => Promise<void>;
}) {
  const [expiresAt, setExpiresAt] = useState("");
  const [error, setError] = useState("");
  const submit = async () => {
    let expiry: string | undefined;
    if (expiresAt) {
      const parsed = new Date(expiresAt);
      if (Number.isNaN(parsed.getTime()) || parsed <= new Date())
        return setError("Expiration must be a future date and time.");
      expiry = parsed.toISOString();
    }
    try {
      await onCreate("sandbox", expiry);
    } catch (saveError) {
      setError(apiErrorMessage(saveError));
    }
  };
  return (
    <div
      className="modal-backdrop"
      role="presentation"
      onMouseDown={() => {
        if (!saving) onClose();
      }}
    >
      <section
        className="modal compact-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="credential-create-title"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="modal-head">
          <div>
            <p className="section-kicker">Secure API access</p>
            <h2 id="credential-create-title">Create credential</h2>
            <p>The generated secret is encrypted and displayed only once.</p>
          </div>
          <button
            className="icon-button"
            onClick={onClose}
            aria-label="Close"
            disabled={saving}
          >
            <X size={17} />
          </button>
        </div>
        <div className="modal-body">
          <label>
            Environment
            <input value="Sandbox" disabled />
          </label>
          <label>
            Expiration (optional)
            <input
              type="datetime-local"
              value={expiresAt}
              onChange={(event) => {
                setExpiresAt(event.target.value);
                setError("");
              }}
              disabled={saving}
            />
            {error ? <span className="field-error">{error}</span> : null}
          </label>
          <InlineNotice title="Store it safely">
            Emisell cannot show this secret again. Rotate the credential if it
            is lost.
          </InlineNotice>
        </div>
        <div className="modal-foot">
          <button
            className="secondary-button"
            onClick={onClose}
            disabled={saving}
          >
            Cancel
          </button>
          <button
            className="primary-button"
            onClick={() => void submit()}
            disabled={saving}
          >
            {saving ? (
              <LoaderCircle className="spin" size={15} />
            ) : (
              <KeyRound size={15} />
            )}
            {saving ? "Creating…" : "Create credential"}
          </button>
        </div>
      </section>
    </div>
  );
}

function OneTimeSecretModal({
  title,
  identifier,
  secret,
  label,
  onClose,
}: {
  title: string;
  identifier: string;
  secret: string;
  label: string;
  onClose: () => void;
}) {
  const [copied, setCopied] = useState<"identifier" | "secret" | null>(null);
  const copy = async (value: string, field: "identifier" | "secret") => {
    await navigator.clipboard?.writeText(value);
    setCopied(field);
    window.setTimeout(() => setCopied(null), 1600);
  };
  return (
    <div className="modal-backdrop" role="presentation">
      <section
        className="modal compact-modal secret-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="secret-title"
      >
        <div className="modal-head">
          <div>
            <p className="section-kicker">Shown once</p>
            <h2 id="secret-title">{title}</h2>
            <p>
              Copy both values now. Closing this dialog permanently hides the
              secret.
            </p>
          </div>
        </div>
        <div className="modal-body">
          <div className="secret-field">
            <small>Identifier</small>
            <code>{identifier}</code>
            <button
              className="icon-button"
              onClick={() => void copy(identifier, "identifier")}
              aria-label="Copy identifier"
            >
              {copied === "identifier" ? (
                <Check size={15} />
              ) : (
                <Copy size={15} />
              )}
            </button>
          </div>
          <div className="secret-field">
            <small>{label}</small>
            <code>{secret}</code>
            <button
              className="icon-button"
              onClick={() => void copy(secret, "secret")}
              aria-label={`Copy ${label}`}
            >
              {copied === "secret" ? <Check size={15} /> : <Copy size={15} />}
            </button>
          </div>
          <InlineNotice tone="warning" title="This value cannot be recovered">
            Store it in a password manager or secret vault, never in source
            code.
          </InlineNotice>
        </div>
        <div className="modal-foot">
          <button className="primary-button" onClick={onClose}>
            I saved the secret
          </button>
        </div>
      </section>
    </div>
  );
}

function ScopePanel({
  type,
  description,
  scopes,
  loading,
  saving,
  onAdd,
  onRemove,
}: {
  type: string;
  description: string;
  scopes: VersionedScope[];
  loading: boolean;
  saving: boolean;
  onAdd: () => void;
  onRemove: (name: string) => Promise<void>;
}) {
  return (
    <Panel className="scope-panel">
      <div className="panel-heading">
        <div>
          <p className="section-kicker">{type} scopes</p>
          <h2>{type} permissions</h2>
          <p>{description}</p>
        </div>
        <span className="scope-count">{loading ? "…" : scopes.length}</span>
      </div>
      <div className="scope-list">
        {loading ? (
          <div className="scope-loading">
            <LoaderCircle className="spin" size={17} />
            <span>Loading scopes…</span>
          </div>
        ) : scopes.length ? (
          scopes.map(({ scope }) => (
            <div key={scope}>
              <span className="scope-check">
                <Check size={12} />
              </span>
              <span>
                <code>{scope}</code>
                <small>{scopeDescription(scope)}</small>
              </span>
              <button
                aria-label={`Remove ${scope}`}
                onClick={() => void onRemove(scope)}
                disabled={saving}
              >
                <X size={14} />
              </button>
            </div>
          ))
        ) : (
          <div className="scope-empty">
            <span>No {type.toLowerCase()} scopes configured.</span>
          </div>
        )}
      </div>
      <button
        className="secondary-button scope-action"
        onClick={onAdd}
        disabled={loading || saving}
      >
        <Plus size={14} />
        Add scope
      </button>
    </Panel>
  );
}

function AddScopeModal({
  type,
  catalog,
  configuredScopes,
  saving,
  onClose,
  onAdd,
}: {
  type: string;
  catalog: ScopeDefinition[];
  configuredScopes: string[];
  saving: boolean;
  onClose: () => void;
  onAdd: (name: string) => Promise<void>;
}) {
  const [scope, setScope] = useState("");
  const [error, setError] = useState("");
  const selected = catalog.find((definition) => definition.scope === scope);
  const availableScopes = catalog.filter(
    (definition) => !configuredScopes.includes(definition.scope),
  );
  const submit = async () => {
    if (!selected) return setError("Choose a scope from the Emisell catalog.");
    if (selected.availability !== "available")
      return setError(
        "This scope is documented for the roadmap but has no callable Provider API endpoint yet.",
      );
    try {
      await onAdd(scope);
    } catch (saveError) {
      setError(apiErrorMessage(saveError));
    }
  };
  return (
    <div
      className="modal-backdrop"
      role="presentation"
      onMouseDown={() => {
        if (!saving) onClose();
      }}
    >
      <section
        className="modal compact-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="scope-title"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="modal-head">
          <div>
            <p className="section-kicker">{type} permission</p>
            <h2 id="scope-title">Add scope</h2>
            <p>
              Scope changes are stored immediately and included in the next app
              version.
            </p>
          </div>
          <button
            className="icon-button"
            onClick={onClose}
            aria-label="Close"
            disabled={saving}
          >
            <X size={17} />
          </button>
        </div>
        <div className="modal-body">
          <label>
            Official Emisell scope
            <select
              autoFocus
              value={scope}
              onChange={(event) => {
                setScope(event.target.value);
                setError("");
              }}
              disabled={saving}
            >
              <option value="">Select a scope</option>
              {availableScopes.map((definition) => (
                <option
                  key={definition.scope}
                  value={definition.scope}
                  disabled={definition.availability !== "available"}
                >
                  {definition.scope} · {definition.availability}
                </option>
              ))}
            </select>
            {error ? <span className="field-error">{error}</span> : null}
          </label>
          {selected ? (
            <div className="scope-catalog-summary">
              <div>
                <strong>{selected.name}</strong>
                <span className={`scope-availability ${selected.availability}`}>
                  {selected.availability}
                </span>
              </div>
              <p>{selected.description}</p>
              <small>
                Risk: {selected.risk} · Approval:{" "}
                {selected.approval.replaceAll("_", " ")}
              </small>
            </div>
          ) : (
            <InlineNotice title="Controlled scope catalog">
              Providers can request only scopes registered by Emisell. Planned
              scopes remain visible but cannot be added until their API is live.
            </InlineNotice>
          )}
        </div>
        <div className="modal-foot">
          <button
            className="secondary-button"
            onClick={onClose}
            disabled={saving}
          >
            Cancel
          </button>
          <button
            className="primary-button"
            onClick={() => void submit()}
            disabled={
              saving || !selected || selected.availability !== "available"
            }
          >
            {saving ? <LoaderCircle className="spin" size={15} /> : null}
            {saving ? "Saving…" : "Add scope"}
          </button>
        </div>
      </section>
    </div>
  );
}

type WebhookTestRun = {
  eventId: string;
  event: string;
  subscriptionIds: string[];
  status: "delivering" | "delivered" | "partial" | "failed" | "timed_out";
  totalAttempts: number;
  pendingAttempts: number;
  deliveredAttempts: number;
  failedAttempts: number;
  startedAt: string;
};

async function fetchWebhookDeliveries(
  appId: string,
  subscriptionIds: string[],
  signal?: AbortSignal,
) {
  const uniqueIds = [...new Set(subscriptionIds)];
  const pages = await Promise.all(
    uniqueIds.map((subscriptionId) =>
      appPlatformClient.listWebhookDeliveries(appId, subscriptionId, signal),
    ),
  );
  return [
    ...new Map(
      pages.flatMap((page) => page.data).map((item) => [item.id, item]),
    ).values(),
  ].sort(
    (left, right) =>
      Date.parse(right.attemptedAt) - Date.parse(left.attemptedAt),
  );
}

function mergeWebhookDeliveries(
  current: WebhookDelivery[],
  incoming: WebhookDelivery[],
) {
  return [
    ...new Map(
      [...incoming, ...current].map((item) => [item.id, item]),
    ).values(),
  ].sort(
    (left, right) =>
      Date.parse(right.attemptedAt) - Date.parse(left.attemptedAt),
  );
}

function WebhooksView({
  app,
  notify,
  canDispatch,
}: {
  app: App;
  notify: (message: string, tone?: "success" | "info" | "warning") => void;
  canDispatch: boolean;
}) {
  const [addOpen, setAddOpen] = useState(false);
  const [webhooks, setWebhooks] = useState<WebhookSubscription[]>([]);
  const [eventCatalog, setEventCatalog] = useState<WebhookEventDefinition[]>([]);
  const [loading, setLoading] = useState(true);
  const [savingId, setSavingId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [editWebhook, setEditWebhook] = useState<WebhookSubscription | null>(
    null,
  );
  const [deleteWebhook, setDeleteWebhook] =
    useState<WebhookSubscription | null>(null);
  const [signingSecret, setSigningSecret] = useState<{
    title: string;
    identifier: string;
    secret: string;
    label: string;
  } | null>(null);
  const [deliveries, setDeliveries] = useState<WebhookDelivery[]>([]);
  const [deliveriesLoading, setDeliveriesLoading] = useState(true);
  const [publishOpen, setPublishOpen] = useState(false);
  const [publishing, setPublishing] = useState(false);
  const [releasedWebhooks, setReleasedWebhooks] = useState<
    VersionedWebhookSubscription[]
  >([]);
  const [testRun, setTestRun] = useState<WebhookTestRun | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    Promise.all([
      appPlatformClient.listWebhooks(app.id, controller.signal),
      appPlatformClient.listWebhookEventCatalog(controller.signal),
    ])
      .then(async ([response, catalogResponse]) => {
        setWebhooks(response.data);
        setEventCatalog(catalogResponse.data);
        const released = app.activeVersionId
          ? await appPlatformClient.getVersion(
              app.id,
              app.activeVersionId,
              controller.signal,
            )
          : null;
        const releasedSubscriptions =
          released?.snapshot.webhookSubscriptions ?? [];
        setReleasedWebhooks(releasedSubscriptions);
        const subscriptionIds = [
          ...response.data.map((webhook) => webhook.id),
          ...releasedSubscriptions.map((webhook) => webhook.subscriptionId),
        ];
        setDeliveries(
          subscriptionIds.length
            ? await fetchWebhookDeliveries(
                app.id,
                subscriptionIds,
                controller.signal,
              )
            : [],
        );
      })
      .catch((loadError) => {
        if (!(
          loadError instanceof DOMException && loadError.name === "AbortError"
        ))
          setError(apiErrorMessage(loadError));
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setLoading(false);
          setDeliveriesLoading(false);
        }
      });
    return () => controller.abort();
  }, [app.activeVersionId, app.id]);

  const testSubscriptionKey = testRun?.subscriptionIds.join("|") ?? "";
  useEffect(() => {
    if (!testRun?.eventId || !testSubscriptionKey) return;
    const controller = new AbortController();
    const eventId = testRun.eventId;
    const subscriptionIds = testSubscriptionKey.split("|");
    let timer: number | undefined;
    let pollCount = 0;

    const poll = async () => {
      pollCount += 1;
      try {
        const latest = await fetchWebhookDeliveries(
          app.id,
          subscriptionIds,
          controller.signal,
        );
        const relevant = latest.filter(
          (delivery) => delivery.eventId === eventId,
        );
        setDeliveries((current) => mergeWebhookDeliveries(current, latest));

        const latestBySubscription = new Map<string, WebhookDelivery>();
        for (const delivery of relevant) {
          const previous = latestBySubscription.get(delivery.subscriptionId);
          if (!previous || delivery.attempt > previous.attempt) {
            latestBySubscription.set(delivery.subscriptionId, delivery);
          }
        }
        const outcomes = [...latestBySubscription.values()];
        const hasPending = outcomes.some(
          (delivery) => delivery.status === "pending",
        );
        const deliveredSubscriptions = outcomes.filter(
          (delivery) => delivery.status === "delivered",
        ).length;
        const complete =
          outcomes.length === subscriptionIds.length && !hasPending;
        const status: WebhookTestRun["status"] = complete
          ? deliveredSubscriptions === subscriptionIds.length
            ? "delivered"
            : deliveredSubscriptions > 0
              ? "partial"
              : "failed"
          : pollCount >= 20
            ? "timed_out"
            : "delivering";

        setTestRun((current) =>
          current?.eventId === eventId
            ? {
                ...current,
                status,
                totalAttempts: relevant.length,
                pendingAttempts: relevant.filter(
                  (delivery) => delivery.status === "pending",
                ).length,
                deliveredAttempts: relevant.filter(
                  (delivery) => delivery.status === "delivered",
                ).length,
                failedAttempts: relevant.filter(
                  (delivery) => delivery.status === "failed",
                ).length,
              }
            : current,
        );

        if (!complete && pollCount < 20 && !controller.signal.aborted) {
          timer = window.setTimeout(() => void poll(), 750);
        }
      } catch (pollError) {
        if (!(
          pollError instanceof DOMException && pollError.name === "AbortError"
        )) {
          setTestRun((current) =>
            current?.eventId === eventId
              ? { ...current, status: "timed_out" }
              : current,
          );
        }
      }
    };

    timer = window.setTimeout(() => void poll(), 500);
    return () => {
      controller.abort();
      if (timer) window.clearTimeout(timer);
    };
  }, [app.id, testRun?.eventId, testSubscriptionKey]);

  const refreshDeliveryLog = async () => {
    const subscriptionIds = [
      ...webhooks.map((webhook) => webhook.id),
      ...releasedWebhooks.map((webhook) => webhook.subscriptionId),
    ];
    setDeliveriesLoading(true);
    setError(null);
    try {
      setDeliveries(
        subscriptionIds.length
          ? await fetchWebhookDeliveries(app.id, subscriptionIds)
          : [],
      );
    } catch (loadError) {
      setError(apiErrorMessage(loadError));
    } finally {
      setDeliveriesLoading(false);
    }
  };

  const addWebhook = async (event: string, endpointUrl: string) => {
    setSavingId("create");
    setError(null);
    try {
      const result = await appPlatformClient.createWebhook(app.id, {
        event,
        endpointUrl,
      });
      setWebhooks((current) => [result.subscription, ...current]);
      setAddOpen(false);
      setSigningSecret({
        title: "Webhook created",
        identifier: result.subscription.event,
        secret: result.signingSecret,
        label: "Signing secret",
      });
      notify(`${event} webhook subscription created.`);
    } catch (saveError) {
      setError(apiErrorMessage(saveError));
      throw saveError;
    } finally {
      setSavingId(null);
    }
  };

  const updateWebhook = async (
    webhook: WebhookSubscription,
    input: { endpointUrl?: string; status?: "active" | "paused" },
  ) => {
    setSavingId(webhook.id);
    setError(null);
    try {
      const updated = await appPlatformClient.updateWebhook(
        app.id,
        webhook.id,
        { ...input, revision: webhook.revision },
      );
      setWebhooks((current) =>
        current.map((item) => (item.id === updated.id ? updated : item)),
      );
      setEditWebhook(null);
      notify(
        `${updated.event} webhook ${updated.status === "paused" ? "paused" : "updated"}.`,
      );
    } catch (saveError) {
      const message = apiErrorMessage(saveError);
      setError(message);
      notify(message, "warning");
      throw saveError;
    } finally {
      setSavingId(null);
    }
  };

  const removeWebhook = async () => {
    if (!deleteWebhook) return;
    setSavingId(deleteWebhook.id);
    try {
      await appPlatformClient.deleteWebhook(app.id, deleteWebhook.id);
      setWebhooks((current) =>
        current.filter((item) => item.id !== deleteWebhook.id),
      );
      notify(`${deleteWebhook.event} webhook deleted.`);
      setDeleteWebhook(null);
    } catch (saveError) {
      const message = apiErrorMessage(saveError);
      setError(message);
      notify(message, "warning");
    } finally {
      setSavingId(null);
    }
  };
  const publishEvent = async (
    event: string,
    payload: Record<string, unknown>,
  ) => {
    setPublishing(true);
    setError(null);
    try {
      const result = await appPlatformClient.publishWebhookEvent(app.id, {
        event,
        payload,
      });
      setDeliveries((current) =>
        mergeWebhookDeliveries(current, result.deliveries),
      );
      setTestRun({
        eventId: result.event.id,
        event,
        subscriptionIds: [
          ...new Set(
            result.deliveries.map((delivery) => delivery.subscriptionId),
          ),
        ],
        status: "delivering",
        totalAttempts: result.deliveries.length,
        pendingAttempts: result.deliveries.length,
        deliveredAttempts: 0,
        failedAttempts: 0,
        startedAt: result.event.createdAt,
      });
      setPublishOpen(false);
      notify(
        `${event} queued for ${result.deliveries.length} endpoint${result.deliveries.length === 1 ? "" : "s"}.`,
      );
    } catch (publishError) {
      const message = apiErrorMessage(publishError);
      setError(message);
      throw publishError;
    } finally {
      setPublishing(false);
    }
  };
  const activeCount = webhooks.filter(
    (item) => item.status === "active",
  ).length;
  const pausedCount = webhooks.filter(
    (item) => item.status === "paused",
  ).length;
  const attentionCount = webhooks.filter(
    (item) => item.status === "failing" || item.status === "pending",
  ).length;
  const failedDeliveries = deliveries.filter(
    (delivery) => delivery.status === "failed",
  ).length;
  const availableEventNames = new Set(
    eventCatalog
      .filter((definition) => definition.availability === "available")
      .map((definition) => definition.event),
  );
  const releasedEvents = [
    ...new Set(
      releasedWebhooks
        .map((webhook) => webhook.event)
        .filter((event) => availableEventNames.has(event)),
    ),
  ];
  const activeWorkingWebhooks = webhooks.filter(
    (webhook) => webhook.status === "active",
  );
  const hasUnreleasedWebhookChanges = activeWorkingWebhooks.some(
    (webhook) =>
      !releasedWebhooks.some(
        (released) =>
          released.subscriptionId === webhook.id &&
          released.event === webhook.event &&
          released.endpointUrl === webhook.endpointUrl,
      ),
  );
  return (
    <div className="detail-stack">
      <DetailHeading
        eyebrow="Event delivery"
        title="Webhooks"
        description="Receive signed real-time platform events at your HTTPS endpoints."
        action={
          <div className="heading-actions">
            <button
              className="secondary-button"
              onClick={() => setPublishOpen(true)}
              disabled={!releasedEvents.length || !canDispatch}
              title={
                !canDispatch
                  ? "Only organization owners and administrators can dispatch webhook tests."
                  : !releasedEvents.length
                    ? "Release a version with an active webhook first."
                    : undefined
              }
            >
              <BellRing size={15} />
              Send test event
            </button>
            <button
              className="primary-button"
              onClick={() => setAddOpen(true)}
              disabled={loading || availableEventNames.size === 0}
            >
              <Plus size={16} />
              Add webhook
            </button>
          </div>
        }
      />
      {error ? (
        <InlineNotice tone="warning" title="Webhook request failed">
          {error}
        </InlineNotice>
      ) : null}
      {hasUnreleasedWebhookChanges ? (
        <InlineNotice tone="warning" title="Webhook changes are not released">
          Create and release a new app version before testing the latest
          endpoint configuration.
        </InlineNotice>
      ) : null}
      {testRun ? <WebhookTestRunPanel run={testRun} /> : null}
      <div className="webhook-stats">
        <Panel>
          <span>
            <Webhook size={16} />
          </span>
          <p>Active endpoints</p>
          <strong>{loading ? "…" : activeCount}</strong>
          <small>Included in the next version</small>
        </Panel>
        <Panel>
          <span>
            <CheckCircle2 size={16} />
          </span>
          <p>Delivered attempts</p>
          <strong>
            {deliveriesLoading
              ? "…"
              : deliveries.filter((delivery) => delivery.status === "delivered")
                  .length}
          </strong>
          <small>Signed requests accepted</small>
        </Panel>
        <Panel>
          <span>
            <AlertCircle size={16} />
          </span>
          <p>Need attention</p>
          <strong>
            {deliveriesLoading ? "…" : failedDeliveries + attentionCount}
          </strong>
          <small>Failed attempts or endpoints</small>
        </Panel>
      </div>
      <Panel>
        <div className="panel-heading">
          <div>
            <p className="section-kicker">Platform contract</p>
            <h2>Webhook event catalog</h2>
            <p>
              Loaded from App Gateway. Planned events are documented but cannot
              be subscribed until their Emisell Backend producer is live.
            </p>
          </div>
          <span className="quiet-count">
            {eventCatalog.filter((item) => item.availability === "available").length}{" "}
            available
          </span>
        </div>
        <div className="webhook-event-catalog">
          {loading ? (
            <div className="webhook-empty">
              <LoaderCircle className="spin" size={18} />
              <span>Loading event catalog…</span>
            </div>
          ) : eventCatalog.length ? (
            eventCatalog.map((definition) => (
              <div className="webhook-event-row" key={definition.event}>
                <span className="webhook-icon">
                  <BellRing size={14} />
                </span>
                <span className="webhook-event-copy">
                  <code>{definition.event}</code>
                  <strong>{definition.name}</strong>
                  <small>{definition.description}</small>
                </span>
                <span className="webhook-event-meta">
                  <span className={`scope-availability ${definition.availability}`}>
                    {definition.availability}
                  </span>
                  <small>
                    {definition.source === "app_platform" ? "App Platform" : "Emisell Backend"}
                    {definition.requiredScope ? ` · ${definition.requiredScope}` : ""}
                  </small>
                </span>
              </div>
            ))
          ) : (
            <div className="webhook-empty">
              <AlertCircle size={20} />
              <strong>No event definitions available</strong>
              <span>Check the App Gateway catalog configuration.</span>
            </div>
          )}
        </div>
      </Panel>
      <Panel>
        <div className="panel-heading">
          <div>
            <p className="section-kicker">Subscriptions</p>
            <h2>Webhook endpoints</h2>
            <p>Active subscriptions are bundled into every new app version.</p>
          </div>
          <span className="quiet-count">{pausedCount} paused</span>
        </div>
        <div className="webhook-list">
          {loading ? (
            <div className="webhook-empty">
              <LoaderCircle className="spin" size={19} />
              <span>Loading subscriptions…</span>
            </div>
          ) : webhooks.length ? (
            webhooks.map((webhook) => (
              <WebhookRow
                key={webhook.id}
                webhook={webhook}
                saving={savingId === webhook.id}
                onEdit={() => setEditWebhook(webhook)}
                onToggle={() =>
                  void updateWebhook(webhook, {
                    status: webhook.status === "paused" ? "active" : "paused",
                  }).catch(() => undefined)
                }
                onDelete={() => setDeleteWebhook(webhook)}
              />
            ))
          ) : (
            <div className="webhook-empty">
              <Webhook size={21} />
              <strong>No webhook subscriptions</strong>
              <span>
                Add an HTTPS endpoint to start receiving signed events.
              </span>
            </div>
          )}
        </div>
      </Panel>
      <Panel>
        <div className="panel-heading">
          <div>
            <p className="section-kicker">Delivery log</p>
            <h2>Recent attempts</h2>
            <p>
              Payloads and signing secrets are intentionally hidden from this
              view.
            </p>
          </div>
          <button
            className="filter-button"
            onClick={() => void refreshDeliveryLog()}
            disabled={deliveriesLoading}
          >
            <RefreshCw size={14} />
            Refresh
          </button>
        </div>
        <div className="delivery-list">
          {deliveriesLoading ? (
            <div className="webhook-empty">
              <LoaderCircle className="spin" size={18} />
              <span>Loading delivery history…</span>
            </div>
          ) : deliveries.length ? (
            deliveries
              .slice(0, 12)
              .map((delivery) => (
                <WebhookDeliveryRow delivery={delivery} key={delivery.id} />
              ))
          ) : (
            <div className="webhook-empty">
              <Clock3 size={20} />
              <strong>No delivery attempts</strong>
              <span>
                Send a test event after releasing a version with an active
                subscription.
              </span>
            </div>
          )}
        </div>
      </Panel>
      {addOpen ? (
        <AddWebhookModal
          events={eventCatalog.filter(
            (definition) => definition.availability === "available",
          )}
          saving={savingId === "create"}
          onClose={() => setAddOpen(false)}
          onAdd={addWebhook}
        />
      ) : null}
      {publishOpen ? (
        <PublishWebhookModal
          events={releasedEvents}
          saving={publishing}
          onClose={() => setPublishOpen(false)}
          onPublish={publishEvent}
        />
      ) : null}
      {editWebhook ? (
        <EditWebhookModal
          webhook={editWebhook}
          saving={savingId === editWebhook.id}
          onClose={() => setEditWebhook(null)}
          onSave={(endpointUrl) => updateWebhook(editWebhook, { endpointUrl })}
        />
      ) : null}
      <ConfirmDialog
        open={Boolean(deleteWebhook)}
        title="Delete webhook?"
        description="This endpoint will stop receiving events and will be excluded from future app versions."
        confirmLabel="Delete webhook"
        tone="danger"
        loading={Boolean(deleteWebhook && savingId === deleteWebhook.id)}
        onClose={() => setDeleteWebhook(null)}
        onConfirm={() => void removeWebhook()}
      />
      {signingSecret ? (
        <OneTimeSecretModal
          {...signingSecret}
          onClose={() => setSigningSecret(null)}
        />
      ) : null}
    </div>
  );
}

function WebhookTestRunPanel({ run }: { run: WebhookTestRun }) {
  const running = run.status === "delivering";
  const successful = run.status === "delivered";
  const statusLabel = running
    ? "Delivering"
    : successful
      ? "Delivered"
      : run.status === "partial"
        ? "Partial"
        : run.status === "failed"
          ? "Failed"
          : "Timed out";
  const message = running
    ? "The dispatcher is sending signed requests and checking endpoint responses."
    : successful
      ? "Every released endpoint accepted the latest test event."
      : run.status === "partial"
        ? "At least one endpoint accepted the event while another endpoint still failed."
        : run.status === "failed"
          ? "The latest attempt for every endpoint failed. Review the delivery log before retrying."
          : "The outcome was not final within 15 seconds. Delivery processing may still continue in the background.";
  return (
    <Panel className={`webhook-test-run ${run.status}`}>
      <span className="webhook-test-icon">
        {running ? (
          <LoaderCircle className="spin" size={18} />
        ) : successful ? (
          <CheckCircle2 size={18} />
        ) : (
          <AlertCircle size={18} />
        )}
      </span>
      <div className="webhook-test-copy">
        <p className="section-kicker">Latest development test</p>
        <h3>{run.event}</h3>
        <span>{message}</span>
        <small>
          Event {run.eventId.slice(0, 8)} · Started{" "}
          {formatRelativeDate(run.startedAt)}
        </small>
      </div>
      <div className="webhook-test-outcome">
        <StatusBadge status={statusLabel} />
        <div>
          <span>
            <strong>{run.deliveredAttempts}</strong>
            <small>Delivered</small>
          </span>
          <span>
            <strong>{run.failedAttempts}</strong>
            <small>Failed</small>
          </span>
          <span>
            <strong>{run.pendingAttempts}</strong>
            <small>Pending</small>
          </span>
        </div>
      </div>
    </Panel>
  );
}

function WebhookDeliveryRow({ delivery }: { delivery: WebhookDelivery }) {
  const outcome = delivery.responseStatus
    ? `HTTP ${delivery.responseStatus}`
    : delivery.errorCode
      ? titleCase(delivery.errorCode.replaceAll("_", " "))
      : "Queued";
  return (
    <div className="delivery-row">
      <span className={`delivery-state ${delivery.status}`}>
        {delivery.status === "delivered" ? (
          <Check size={13} />
        ) : delivery.status === "failed" ? (
          <AlertCircle size={13} />
        ) : (
          <Clock3 size={13} />
        )}
      </span>
      <span>
        <code>{delivery.event}</code>
        <small>
          Attempt {delivery.attempt} · {outcome}
        </small>
      </span>
      <span className="delivery-meta">
        <StatusBadge status={titleCase(delivery.status)} />
        <small>
          {delivery.responseTimeMs === null
            ? "—"
            : `${delivery.responseTimeMs} ms`}{" "}
          · {formatRelativeDate(delivery.attemptedAt)}
        </small>
      </span>
    </div>
  );
}

function PublishWebhookModal({
  events,
  saving,
  onClose,
  onPublish,
}: {
  events: string[];
  saving: boolean;
  onClose: () => void;
  onPublish: (event: string, payload: Record<string, unknown>) => Promise<void>;
}) {
  const [eventName, setEventName] = useState(events[0] ?? "");
  const [payload, setPayload] = useState('{\n  "resourceId": "example_123"\n}');
  const [error, setError] = useState("");
  const submit = async () => {
    try {
      const parsed = JSON.parse(payload) as unknown;
      if (!parsed || Array.isArray(parsed) || typeof parsed !== "object")
        throw new Error("Payload must be a JSON object.");
      await onPublish(eventName, parsed as Record<string, unknown>);
    } catch (publishError) {
      setError(
        publishError instanceof SyntaxError
          ? "Payload must contain valid JSON."
          : apiErrorMessage(publishError),
      );
    }
  };
  return (
    <div
      className="modal-backdrop"
      role="presentation"
      onMouseDown={() => {
        if (!saving) onClose();
      }}
    >
      <section
        className="modal compact-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="publish-webhook-title"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="modal-head">
          <div>
            <p className="section-kicker">Development delivery test</p>
            <h2 id="publish-webhook-title">Run webhook test</h2>
            <p>
              The active released subscription receives a signed request through
              the normal retry queue.
            </p>
          </div>
          <button
            className="icon-button"
            onClick={onClose}
            aria-label="Close"
            disabled={saving}
          >
            <X size={17} />
          </button>
        </div>
        <div className="modal-body">
          <label>
            Event
            <select
              value={eventName}
              onChange={(event) => setEventName(event.target.value)}
              disabled={saving}
            >
              {events.map((event) => (
                <option key={event}>{event}</option>
              ))}
            </select>
          </label>
          <label>
            Payload
            <textarea
              rows={7}
              value={payload}
              onChange={(event) => {
                setPayload(event.target.value);
                setError("");
              }}
              spellCheck={false}
              disabled={saving}
            />
            {error ? <span className="field-error">{error}</span> : null}
          </label>
          <InlineNotice tone="warning" title="Use non-sensitive test data">
            Never include access tokens, credentials, card data, or unnecessary
            personal information in a webhook payload.
          </InlineNotice>
        </div>
        <div className="modal-foot">
          <button
            className="secondary-button"
            onClick={onClose}
            disabled={saving}
          >
            Cancel
          </button>
          <button
            className="primary-button"
            onClick={() => void submit()}
            disabled={saving || !eventName}
          >
            {saving ? (
              <LoaderCircle className="spin" size={15} />
            ) : (
              <BellRing size={15} />
            )}
            {saving ? "Queueing…" : "Run test"}
          </button>
        </div>
      </section>
    </div>
  );
}

function WebhookRow({
  webhook,
  saving,
  onEdit,
  onToggle,
  onDelete,
}: {
  webhook: WebhookSubscription;
  saving: boolean;
  onEdit: () => void;
  onToggle: () => void;
  onDelete: () => void;
}) {
  return (
    <div className="webhook-row">
      <span className="webhook-icon">
        <Webhook size={15} />
      </span>
      <span className="webhook-main">
        <code>{webhook.event}</code>
        <small>{webhook.endpointUrl}</small>
        <em>Signing fingerprint {webhook.signingSecretFingerprint}</em>
      </span>
      <StatusBadge status={titleCase(webhook.status)} />
      <div className="row-actions">
        <button
          className="icon-button"
          aria-label="Edit webhook"
          disabled={saving}
          onClick={onEdit}
        >
          <Pencil size={14} />
        </button>
        <button
          className="secondary-button compact-button"
          disabled={saving}
          onClick={onToggle}
        >
          {saving ? (
            <LoaderCircle className="spin" size={13} />
          ) : webhook.status === "paused" ? (
            <Play size={13} />
          ) : (
            <PauseCircle size={13} />
          )}
          {webhook.status === "paused" ? "Resume" : "Pause"}
        </button>
        <button
          className="icon-button danger-icon"
          aria-label="Delete webhook"
          disabled={saving}
          onClick={onDelete}
        >
          <Trash2 size={14} />
        </button>
      </div>
    </div>
  );
}

function AddWebhookModal({
  events,
  saving,
  onClose,
  onAdd,
}: {
  events: WebhookEventDefinition[];
  saving: boolean;
  onClose: () => void;
  onAdd: (event: string, endpoint: string) => Promise<void>;
}) {
  const [eventName, setEventName] = useState(events[0]?.event ?? "");
  const [endpoint, setEndpoint] = useState("https://");
  const [error, setError] = useState("");
  const submit = async () => {
    try {
      const url = new URL(endpoint);
      if (url.protocol !== "https:") throw new Error();
      await onAdd(eventName, endpoint);
    } catch (saveError) {
      setError(
        saveError instanceof TypeError
          ? "Use a valid HTTPS endpoint."
          : apiErrorMessage(saveError),
      );
    }
  };
  return (
    <div
      className="modal-backdrop"
      role="presentation"
      onMouseDown={() => {
        if (!saving) onClose();
      }}
    >
      <section
        className="modal compact-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="webhook-title"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="modal-head">
          <div>
            <p className="section-kicker">Event subscription</p>
            <h2 id="webhook-title">Add webhook</h2>
            <p>Emisell signs each request with a unique secret shown once.</p>
          </div>
          <button
            className="icon-button"
            onClick={onClose}
            aria-label="Close"
            disabled={saving}
          >
            <X size={17} />
          </button>
        </div>
        <div className="modal-body">
          <label>
            Event
            <select
              value={eventName}
              onChange={(event) => setEventName(event.target.value)}
              disabled={saving}
            >
              {events.map((definition) => (
                <option key={definition.event} value={definition.event}>
                  {definition.event} — {definition.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            HTTPS endpoint
            <input
              value={endpoint}
              onChange={(event) => {
                setEndpoint(event.target.value);
                setError("");
              }}
              aria-invalid={Boolean(error)}
              disabled={saving}
            />
            {error ? <span className="field-error">{error}</span> : null}
          </label>
          <InlineNotice title="Signed delivery">
            Verify every request using the signing secret and reject invalid
            signatures.
          </InlineNotice>
        </div>
        <div className="modal-foot">
          <button
            className="secondary-button"
            onClick={onClose}
            disabled={saving}
          >
            Cancel
          </button>
          <button
            className="primary-button"
            onClick={() => void submit()}
            disabled={saving || !eventName}
          >
            {saving ? <LoaderCircle className="spin" size={15} /> : null}
            {saving ? "Creating…" : "Create subscription"}
          </button>
        </div>
      </section>
    </div>
  );
}

function EditWebhookModal({
  webhook,
  saving,
  onClose,
  onSave,
}: {
  webhook: WebhookSubscription;
  saving: boolean;
  onClose: () => void;
  onSave: (endpointUrl: string) => Promise<void>;
}) {
  const [endpoint, setEndpoint] = useState(webhook.endpointUrl);
  const [error, setError] = useState("");
  const submit = async () => {
    try {
      const url = new URL(endpoint);
      if (url.protocol !== "https:") throw new TypeError();
      await onSave(endpoint);
    } catch (saveError) {
      setError(
        saveError instanceof TypeError
          ? "Use a valid HTTPS endpoint."
          : apiErrorMessage(saveError),
      );
    }
  };
  return (
    <div
      className="modal-backdrop"
      role="presentation"
      onMouseDown={() => {
        if (!saving) onClose();
      }}
    >
      <section
        className="modal compact-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="webhook-edit-title"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="modal-head">
          <div>
            <p className="section-kicker">{webhook.event}</p>
            <h2 id="webhook-edit-title">Edit endpoint</h2>
            <p>
              Updating this working configuration does not alter previously
              released versions.
            </p>
          </div>
          <button
            className="icon-button"
            onClick={onClose}
            aria-label="Close"
            disabled={saving}
          >
            <X size={17} />
          </button>
        </div>
        <div className="modal-body">
          <label>
            HTTPS endpoint
            <input
              autoFocus
              value={endpoint}
              onChange={(event) => {
                setEndpoint(event.target.value);
                setError("");
              }}
              aria-invalid={Boolean(error)}
              disabled={saving}
            />
            {error ? <span className="field-error">{error}</span> : null}
          </label>
        </div>
        <div className="modal-foot">
          <button
            className="secondary-button"
            onClick={onClose}
            disabled={saving}
          >
            Cancel
          </button>
          <button
            className="primary-button"
            onClick={() => void submit()}
            disabled={saving}
          >
            {saving ? <LoaderCircle className="spin" size={15} /> : null}
            {saving ? "Saving…" : "Save endpoint"}
          </button>
        </div>
      </section>
    </div>
  );
}

function AppSettingsView({
  app,
  notify,
  updateApp,
  archiveApp,
  onArchived,
}: {
  app: App;
  notify: (message: string, tone?: "success" | "info" | "warning") => void;
  updateApp: (
    appId: string,
    input: Parameters<typeof appPlatformClient.updateApp>[1],
  ) => Promise<App>;
  archiveApp: (appId: string) => Promise<void>;
  onArchived: () => void;
}) {
  const [name, setName] = useState(app.name);
  const [description, setDescription] = useState(app.description ?? "");
  const [appUrl, setAppUrl] = useState(app.appUrl ?? "");
  const [contactEmail, setContactEmail] = useState(app.contactEmail ?? "");
  const [saving, setSaving] = useState(false);
  const [archiveOpen, setArchiveOpen] = useState(false);
  const [archiving, setArchiving] = useState(false);
  const [formError, setFormError] = useState("");

  const save = async () => {
    if (name.trim().length < 3) {
      setFormError("App name must contain at least 3 characters.");
      return;
    }
    setSaving(true);
    setFormError("");
    try {
      await updateApp(app.id, {
        name: name.trim(),
        description: description.trim() || null,
        appUrl: appUrl.trim() || null,
        contactEmail: contactEmail.trim() || null,
        revision: app.revision,
      });
      notify("App settings saved.");
    } catch (saveError) {
      const message = apiErrorMessage(saveError);
      setFormError(message);
      notify(message, "warning");
    } finally {
      setSaving(false);
    }
  };
  const archive = async () => {
    setArchiving(true);
    try {
      await archiveApp(app.id);
      setArchiveOpen(false);
      notify(
        "App archived. Existing installations remain connected.",
        "warning",
      );
      onArchived();
    } catch (archiveError) {
      const message = apiErrorMessage(archiveError);
      setFormError(message);
      notify(message, "warning");
    } finally {
      setArchiving(false);
    }
  };
  return (
    <div className="detail-stack">
      <DetailHeading
        eyebrow="App configuration"
        title="Settings"
        description="Manage identity, URLs, and app lifecycle."
        action={
          <button
            className="primary-button"
            onClick={() => void save()}
            disabled={saving || app.status === "archived"}
          >
            {saving ? (
              <LoaderCircle className="spin" size={15} />
            ) : (
              <Check size={15} />
            )}
            {saving ? "Saving…" : "Save changes"}
          </button>
        }
      />
      {formError ? (
        <InlineNotice tone="warning" title="Could not save changes">
          {formError}
        </InlineNotice>
      ) : null}
      <Panel className="settings-form">
        <div className="form-section">
          <div>
            <h3>App identity</h3>
            <p>Shown to merchants during installation.</p>
          </div>
          <div className="form-fields">
            <label>
              App name
              <input
                value={name}
                onChange={(event) => {
                  setName(event.target.value);
                  setFormError("");
                }}
                disabled={app.status === "archived"}
              />
            </label>
            <label>
              Description
              <input
                value={description}
                onChange={(event) => setDescription(event.target.value)}
                placeholder="Describe what this app does"
                disabled={app.status === "archived"}
              />
            </label>
            <label>
              App URL
              <input
                value={appUrl}
                onChange={(event) => setAppUrl(event.target.value)}
                placeholder="https://app.example.com"
                disabled={app.status === "archived"}
              />
            </label>
            <label>
              Contact email
              <input
                value={contactEmail}
                onChange={(event) => setContactEmail(event.target.value)}
                placeholder="developers@example.com"
                type="email"
                disabled={app.status === "archived"}
              />
            </label>
          </div>
        </div>
        <div className="form-section">
          <div>
            <h3>Distribution</h3>
            <p>Public distribution is not part of the invite-only phase.</p>
          </div>
          <div className="form-fields">
            <label>
              Distribution method
              <input value="Custom / invite-only" disabled />
            </label>
          </div>
        </div>
      </Panel>
      <InlineNotice title="Invite-only development">
        App identity can be maintained here. Production access and public app
        distribution require a later Emisell approval flow.
      </InlineNotice>
      <Panel className="danger-zone">
        <div>
          <span>
            <AlertCircle size={17} />
          </span>
          <div>
            <h3>
              {app.status === "archived" ? "App archived" : "Archive app"}
            </h3>
            <p>
              {app.status === "archived"
                ? "This app is no longer available for active development."
                : "Remove this app from active development. Installed merchants will not be affected."}
            </p>
          </div>
        </div>
        <button
          onClick={() => setArchiveOpen(true)}
          disabled={app.status === "archived"}
        >
          {app.status === "archived" ? "Archived" : "Archive app"}
        </button>
      </Panel>
      <ConfirmDialog
        open={archiveOpen}
        eyebrow="Danger zone"
        title="Archive this app?"
        description="The app will disappear from active development lists. Existing installations remain connected and the app can be restored later."
        confirmLabel="Archive app"
        tone="danger"
        loading={archiving}
        onClose={() => setArchiveOpen(false)}
        onConfirm={() => void archive()}
      />
    </div>
  );
}

function CreateAppModal({
  onClose,
  onCreate,
}: {
  onClose: () => void;
  onCreate: (name: string) => Promise<void>;
}) {
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const [creating, setCreating] = useState(false);

  const submit = async () => {
    if (name.trim().length < 3) {
      setError("Use at least 3 characters for the app name.");
      return;
    }
    setCreating(true);
    setError("");
    try {
      await onCreate(name.trim());
    } catch (createError) {
      setError(apiErrorMessage(createError));
      setCreating(false);
    }
  };

  return (
    <div
      className="modal-backdrop"
      role="presentation"
      onMouseDown={() => {
        if (!creating) onClose();
      }}
    >
      <section
        className="modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="create-title"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="modal-head">
          <div>
            <p className="section-kicker">New app</p>
            <h2 id="create-title">Create an app</h2>
            <p>Create a custom app for an approved Emisell developer.</p>
          </div>
          <button
            className="icon-button"
            aria-label="Close"
            onClick={onClose}
            disabled={creating}
          >
            <X size={17} />
          </button>
        </div>
        <div className="modal-body">
          <label>
            App name
            <input
              autoFocus
              placeholder="e.g. Loyalty Connect"
              value={name}
              onChange={(event) => {
                setName(event.target.value);
                setError("");
              }}
              onKeyDown={(event) => {
                if (event.key === "Enter") void submit();
              }}
              aria-invalid={Boolean(error)}
              disabled={creating}
            />
            {error ? <span className="field-error">{error}</span> : null}
          </label>
          <InlineNotice title="Custom app · Sandbox only">
            New apps are private to this organization. Public listing and
            production installations are not enabled in this phase.
          </InlineNotice>
        </div>
        <div className="modal-foot">
          <button
            className="secondary-button"
            onClick={onClose}
            disabled={creating}
          >
            Cancel
          </button>
          <button
            className="primary-button"
            onClick={() => void submit()}
            disabled={creating}
          >
            {creating ? <LoaderCircle className="spin" size={15} /> : null}
            {creating ? "Creating…" : "Create app"}
            {!creating ? <ArrowUpRight size={14} /> : null}
          </button>
        </div>
      </section>
    </div>
  );
}
