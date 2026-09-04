"use client";

import { Fragment, useEffect, useMemo, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import {
  Activity,
  AppWindow,
  ArrowRight,
  Boxes,
  Building2,
  BookOpen,
  CheckCircle2,
  ChevronRight,
  ClipboardCheck,
  LayoutDashboard,
  LoaderCircle,
  LockKeyhole,
  LogOut,
  Network,
  RefreshCw,
  Search,
  ShieldCheck,
  Store,
  Users,
  X,
} from "lucide-react";
import {
  apiErrorMessage,
  appPlatformClient,
} from "../../lib/app-platform/client";
import type {
  CatalogCandidate,
  CatalogCategory,
} from "../../lib/app-platform/contracts";
import type {
  DeveloperApplication,
  DeveloperOrganization,
  DeveloperOrganizationDetail,
} from "../../lib/app-platform/domain";
import { useAppPlatform } from "../app-platform-store";
import { DeveloperRequestsView } from "../developer-requests/developer-requests-view";
import { Toast } from "../components/ui";
import { DocumentationView } from './docs/documentation-view';
import { IntegrationReadinessView } from '../integration-readiness';
import type { IntegrationReadiness } from '../../lib/app-platform/integration';
import './admin-theme.css';

type AdminSection = "Overview" | "Developer requests" | "Organizations" | "App catalog" | "Documentation";

const adminNavigation: Array<{
  label: AdminSection;
  path: string;
  icon: typeof LayoutDashboard;
  group: string;
}> = [
  { label: "Overview", path: "/admin", icon: LayoutDashboard, group: "Workspace" },
  {
    label: "Developer requests",
    path: "/admin/developer-requests",
    icon: ClipboardCheck,
    group: "Developer program",
  },
  { label: "Organizations", path: "/admin/organizations", icon: Building2, group: "Developer program" },
  { label: "App catalog", path: "/admin/app-catalog", icon: Store, group: "Platform" },
  { label: "Documentation", path: "/admin/docs", icon: BookOpen, group: "Platform" },
];

export default function AdminDashboard() {
  const pathname = usePathname();
  const router = useRouter();
  const { session, logout, error: identityError, loading: identityLoading } = useAppPlatform();
  const [signingOut, setSigningOut] = useState(false);
  const [applications, setApplications] = useState<DeveloperApplication[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [toast, setToast] = useState<{
    message: string;
    tone: "success" | "info" | "warning";
  } | null>(null);

  useEffect(() => {
    if (!session?.platformOperator) {
      return;
    }
    const controller = new AbortController();
    appPlatformClient
      .listDeveloperApplications({}, controller.signal)
      .then((response) => setApplications(response.data))
      .catch((requestError) => {
        if (
          !(
            requestError instanceof DOMException &&
            requestError.name === "AbortError"
          )
        )
          setError(apiErrorMessage(requestError));
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [session?.platformOperator]);

  const activeSection =
    adminNavigation.find((item) => item.path === pathname)?.label ?? "Overview";
  const notify = (
    message: string,
    tone: "success" | "info" | "warning" = "success",
  ) => {
    setToast({ message, tone });
    window.setTimeout(() => setToast(null), 2800);
  };

  if (identityLoading) return <AdminAccessState loading />;
  if (!session?.platformOperator) return <AdminAccessState error={identityError} />;

  return (
    <div className="admin-shell">
      <aside className="admin-sidebar">
        <div className="admin-brand">
          <span>E</span>
          <div>
            <strong>Emisell</strong>
            <small>App Platform</small>
          </div>
        </div>
        <div className="admin-environment">
          <span><ShieldCheck size={18} /></span>
          <div>
            <strong>Admin workspace</strong>
            <small>Internal access</small>
          </div>
        </div>
        <nav aria-label="Admin navigation">
          {adminNavigation.map(({ label, path, icon: Icon, group }, index) => (
            <Fragment key={label}>
            {index === 0 || adminNavigation[index - 1].group !== group ? <p>{group}</p> : null}
            <button
              className={activeSection === label ? "is-active" : ""}
              aria-label={label}
              aria-current={activeSection === label ? 'page' : undefined}
              title={label}
              onClick={() => router.push(path)}
            >
              <Icon size={17} />
              <span>{label}</span>
            </button>
            </Fragment>
          ))}
        </nav>
        <div className="admin-sidebar-foot">
          <button aria-label="Open Developer console" title="Developer console" onClick={() => router.push("/overview")}>
            <AppWindow size={16} />
            <span>
              <strong>Developer console</strong>
              <small>Open developer workspace</small>
            </span>
            <ArrowRight size={14} />
          </button>
          <div className="admin-sidebar-account">
            <div className="admin-identity">
              <span>{initials(session.displayName)}</span>
              <div><strong>{session.displayName}</strong><small>{session.email}</small></div>
            </div>
            <button className="admin-sign-out" disabled={signingOut} aria-label="Sign out of Admin Console" title="Sign out" onClick={() => {
              setSigningOut(true);
              void logout().catch(() => { setSigningOut(false); notify('Sign-out failed. Please try again.', 'warning'); });
            }}>{signingOut ? <LoaderCircle size={17} className="spin" /> : <LogOut size={17} />}</button>
          </div>
        </div>
      </aside>
      <div className="admin-main">
        <header className="admin-topbar">
          <div className="admin-breadcrumb">
            <LayoutDashboard size={17} />
            <span>Admin workspace</span>
            <ChevronRight size={13} />
            <strong>{activeSection}</strong>
          </div>
          <span className="admin-session-badge"><ShieldCheck size={15} /><span>Admin session</span></span>
        </header>
        {activeSection === "Overview" ? (
          <AdminOverview
            applications={applications}
            loading={loading}
            error={error}
            onOpenQueue={() => router.push("/admin/developer-requests")}
          />
        ) : activeSection === "Developer requests" ? (
          <DeveloperRequestsView notify={notify} />
        ) : activeSection === "Organizations" ? (
          <AdminOrganizations
            onOpenQueue={() => router.push("/admin/developer-requests")}
          />
        ) : activeSection === "Documentation" ? (
          <DocumentationView />
        ) : (
          <AdminCatalog notify={notify} />
        )}
      </div>
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

const catalogCategories: CatalogCategory[] = [
  "payment",
  "shipping",
  "erp",
  "marketing",
  "operations",
  "custom",
];

function defaultCatalogCategory(candidate: CatalogCandidate): CatalogCategory {
  const type = candidate.activeVersion?.snapshot.extensions[0]?.type;
  return type === "payment" || type === "shipping" ? type : "custom";
}

function AdminCatalog({ notify }: { notify: (message: string, tone?: "success" | "info" | "warning") => void }) {
  const [items, setItems] = useState<CatalogCandidate[]>([]);
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(true);
  const [savingId, setSavingId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [cursor, setCursor] = useState("");
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [refresh, setRefresh] = useState(0);
  const [reviewing, setReviewing] = useState<CatalogCandidate | null>(null);
  const [inspection, setInspection] = useState<IntegrationReadiness | null>(null);
  const [drafts, setDrafts] = useState<Record<string, { category: CatalogCategory; featured: boolean }>>({});

  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      setLoading(true);
      setError(null);
      appPlatformClient
        .listCatalogCandidates({ search, cursor }, controller.signal)
        .then((response) => {
          if (controller.signal.aborted) return;
          setItems(response.data);
          setNextCursor(response.meta.nextCursor);
          setDrafts((current) => {
            const next = { ...current };
            for (const item of response.data) {
              next[item.app.id] = {
                category: item.listing?.category ?? defaultCatalogCategory(item),
                featured: item.listing?.featured ?? false,
              };
            }
            return next;
          });
        })
        .catch((requestError) => {
          if (!controller.signal.aborted) {
            setError(apiErrorMessage(requestError));
            setItems([]);
            setNextCursor(null);
          }
        })
        .finally(() => {
          if (!controller.signal.aborted) setLoading(false);
        });
    }, 180);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [search, cursor, refresh]);

  const save = async (candidate: CatalogCandidate, status: "draft" | "published" | "hidden") => {
    const reviewed = inspection?.appId === candidate.app.id ? inspection : null;
    if (status === "published" && (!reviewed || !reviewed.activeVersionId)) return;
    const draft = drafts[candidate.app.id] ?? {
      category: defaultCatalogCategory(candidate),
      featured: false,
    };
    setSavingId(candidate.app.id);
    setError(null);
    try {
      const listing = await appPlatformClient.updateCatalogListing(
        candidate.organizationId,
        candidate.app.id,
        {
          category: draft.category,
          featured: draft.featured,
          status,
          revision: status === "published" ? reviewed!.listingRevision : candidate.listing?.revision ?? 0,
          ...(status === "published" ? { expectedAppRevision: reviewed!.appRevision, expectedActiveVersionId: reviewed!.activeVersionId! } : {}),
        },
      );
      setItems((current) =>
        current.map((item) =>
          item.app.id === candidate.app.id ? { ...item, listing } : item,
        ),
      );
      notify(
        status === "published"
          ? `${candidate.app.name} published to App Store.`
          : status === "hidden"
            ? `${candidate.app.name} hidden from App Store.`
            : `${candidate.app.name} catalog draft saved.`,
      );
      setReviewing(null);
      setInspection(null);
      setRefresh((value) => value + 1);
    } catch (saveError) {
      setError(apiErrorMessage(saveError));
      setInspection(null);
      setReviewing(null);
      notify("Catalog update failed.", "warning");
    } finally {
      setSavingId(null);
    }
  };

  return (
    <main className="admin-content admin-catalog-page">
      <section className="admin-heading">
        <div>
          <p className="section-kicker">App governance</p>
          <h1>App catalog</h1>
          <p>Inspect developer configuration and integration evidence before deciding publication.</p>
        </div>
        <div className="admin-heading-stat"><strong>{items.filter((item) => item.listing?.status === "published").length}</strong><span>published on this page</span></div>
      </section>
      <section className="admin-catalog-toolbar">
        <label><Search size={14} /><input aria-label="Search app or developer" value={search} onChange={(event) => { setSearch(event.target.value); setCursor(""); setReviewing(null); setInspection(null); }} placeholder="Search app or developer" /></label>
        <button className="secondary-button" disabled={loading || Boolean(savingId)} onClick={() => { setReviewing(null); setInspection(null); setRefresh((value) => value + 1); }}><RefreshCw size={14} />Refresh</button>
      </section>
      {error ? <div className="admin-inline-error">{error}</div> : null}
      {loading ? (
        <div className="admin-catalog-loading"><LoaderCircle className="spin" size={18} />Loading catalog candidates…</div>
      ) : (
        <section className="admin-catalog-list">
          {items.map((candidate) => {
            const draft = drafts[candidate.app.id] ?? { category: defaultCatalogCategory(candidate), featured: false };
            const published = candidate.listing?.status === "published";
            return (
              <Fragment key={candidate.app.id}><article>
                <span className="admin-catalog-app-icon">{initials(candidate.app.name)}</span>
                <div className="admin-catalog-app-copy">
                  <header><h2>{candidate.app.name}</h2><span className={`status ${candidate.listing?.status ?? "draft"}`}><i />{candidate.listing?.status ?? "draft"}</span></header>
                  <p>{candidate.developerName} · {candidate.activeVersion ? `v${candidate.activeVersion.version}` : "No active version"}</p>
                  {!candidate.eligible ? <small>{candidate.blockingReason}</small> : <small>Eligible for publication</small>}
                </div>
                <label className="admin-catalog-select">Category<select value={draft.category} onChange={(event) => setDrafts((current) => ({ ...current, [candidate.app.id]: { ...draft, category: event.target.value as CatalogCategory } }))}>{catalogCategories.map((category) => <option key={category} value={category}>{category}</option>)}</select></label>
                <label className="admin-catalog-featured"><input type="checkbox" checked={draft.featured} onChange={(event) => setDrafts((current) => ({ ...current, [candidate.app.id]: { ...draft, featured: event.target.checked } }))} />Featured</label>
                <div className="admin-catalog-actions">
                  <button className="secondary-button" disabled={Boolean(savingId)} onClick={() => void save(candidate, published ? "hidden" : "draft")}>{published ? "Hide" : "Save draft"}</button>
                  <button className="primary-button" disabled={Boolean(savingId)} aria-expanded={reviewing?.app.id === candidate.app.id} onClick={() => { setInspection(null); setReviewing(reviewing?.app.id === candidate.app.id ? null : candidate); }}>{reviewing?.app.id === candidate.app.id ? "Close review" : "Review app"}</button>
                </div>
              </article>
              {reviewing?.app.id === candidate.app.id ? <section className="admin-integration-review" aria-label={`Review ${reviewing.app.name}`}>
                <header><h2>Review {reviewing.app.name}</h2><button className="secondary-button" disabled={Boolean(savingId)} onClick={() => { setReviewing(null); setInspection(null); }}>Close review</button></header>
                <IntegrationReadinessView appId={reviewing.app.id} organizationId={reviewing.organizationId} onLoaded={setInspection} />
                <footer><p>Publishing makes an eligible app discoverable. It does not approve production access or verify the integration. The gateway rechecks eligibility when you submit.</p><button className="primary-button" disabled={!inspection?.activeVersionId || Boolean(savingId)} onClick={() => void save(reviewing, "published")}>{savingId ? <LoaderCircle className="spin" size={14} /> : null}Publish reviewed version</button></footer>
              </section> : null}</Fragment>
            );
          })}
          {!items.length ? <div className="admin-catalog-empty">No app candidates match this search.</div> : null}
        </section>
      )}
      {(cursor || nextCursor) && !loading ? <nav className="integration-pagination" aria-label="Catalog pagination"><button className="secondary-button" disabled={!cursor || Boolean(savingId)} onClick={() => { setCursor(""); setReviewing(null); setInspection(null); }}>First page</button><button className="secondary-button" disabled={!nextCursor || Boolean(savingId)} onClick={() => { setCursor(nextCursor ?? ""); setReviewing(null); setInspection(null); }}>Next page <ArrowRight size={14} /></button></nav> : null}
    </main>
  );
}

function AdminOverview({
  applications,
  loading,
  error,
  onOpenQueue,
}: {
  applications: DeveloperApplication[];
  loading: boolean;
  error: string | null;
  onOpenQueue: () => void;
}) {
  const metrics = useMemo(
    () => ({
      pending: applications.filter(
        (item) => item.status === "submitted" || item.status === "under_review",
      ).length,
      invited: applications.filter(
        (item) => item.status === "invited" || item.status === "approved",
      ).length,
      active: applications.filter((item) => item.status === "active").length,
    }),
    [applications],
  );
  return (
    <main className="admin-content">
      <section className="admin-heading">
        <div>
          <p>Platform operations</p>
          <h1>Admin overview</h1>
          <span>
            Review developer access and monitor the Emisell application
            ecosystem.
          </span>
        </div>
        <button onClick={onOpenQueue}>
          <ClipboardCheck size={16} />
          Review developer queue
        </button>
      </section>
      <div className="admin-metrics">
        <AdminMetric
          icon={ClipboardCheck}
          label="Needs review"
          value={loading ? "…" : String(metrics.pending)}
          note="Selected developer candidates"
          tone="amber"
        />
        <AdminMetric
          icon={ShieldCheck}
          label="Invitations"
          value={loading ? "…" : String(metrics.invited)}
          note="Awaiting identity activation"
          tone="violet"
        />
        <AdminMetric
          icon={Building2}
          label="Active partners"
          value={loading ? "…" : String(metrics.active)}
          note="Activated developer access"
          tone="green"
        />
        <AdminMetric
          icon={LockKeyhole}
          label="Console access"
          value="Admin"
          note="Separate, password-protected session"
          tone="blue"
        />
      </div>
      <section className="admin-panel">
        <header>
          <div>
            <p>Developer program</p>
            <h2>Recent requests</h2>
          </div>
          <button onClick={onOpenQueue}>
            View queue <ArrowRight size={14} />
          </button>
        </header>
        {loading ? (
          <div className="admin-empty">
            <LoaderCircle className="spin" size={21} />
            <span>Loading operator queue…</span>
          </div>
        ) : error ? (
          <div className="admin-empty">
            <Activity size={21} />
            <span>{error}</span>
          </div>
        ) : applications.length ? (
          <div className="admin-request-list">
            {applications.slice(0, 5).map((application) => (
              <div key={application.id}>
                <span className="admin-company-mark">
                  {initials(application.companyName)}
                </span>
                <span>
                  <strong>{application.companyName}</strong>
                  <small>
                    {application.requestedAppName} · {application.contactEmail}
                  </small>
                </span>
                <span className={`admin-status ${application.status}`}>
                  {application.status.replaceAll("_", " ")}
                </span>
                <ChevronRight size={15} />
              </div>
            ))}
          </div>
        ) : (
          <div className="admin-empty">
            <CheckCircle2 size={22} />
            <strong>Queue is clear</strong>
            <span>No developer request needs attention.</span>
          </div>
        )}
      </section>
    </main>
  );
}

function AdminMetric({
  icon: Icon,
  label,
  value,
  note,
  tone,
}: {
  icon: typeof ClipboardCheck;
  label: string;
  value: string;
  note: string;
  tone: string;
}) {
  return (
    <section className="admin-metric">
      <span className={tone}>
        <Icon size={18} />
      </span>
      <div>
        <small>{label}</small>
        <strong>{value}</strong>
        <p>{note}</p>
      </div>
    </section>
  );
}

function AdminOrganizations({ onOpenQueue }: { onOpenQueue: () => void }) {
  const [organizations, setOrganizations] = useState<DeveloperOrganization[]>(
    [],
  );
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [selected, setSelected] = useState<DeveloperOrganizationDetail | null>(
    null,
  );
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      setLoading(true);
      setError(null);
      appPlatformClient
        .listDeveloperOrganizations({ search, status }, controller.signal)
        .then((response) => setOrganizations(response.data))
        .catch((requestError) => {
          if (
            !(
              requestError instanceof DOMException &&
              requestError.name === "AbortError"
            )
          )
            setError(apiErrorMessage(requestError));
        })
        .finally(() => {
          if (!controller.signal.aborted) setLoading(false);
        });
    }, 250);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [search, status]);

  useEffect(() => {
    if (!selectedId) return;
    const controller = new AbortController();
    appPlatformClient
      .getDeveloperOrganization(selectedId, controller.signal)
      .then(setSelected)
      .catch((requestError) => {
        if (
          !(
            requestError instanceof DOMException &&
            requestError.name === "AbortError"
          )
        )
          setDetailError(apiErrorMessage(requestError));
      })
      .finally(() => {
        if (!controller.signal.aborted) setDetailLoading(false);
      });
    return () => controller.abort();
  }, [selectedId]);

  const openOrganization = (organizationId: string) => {
    setSelected(null);
    setDetailError(null);
    setDetailLoading(true);
    setSelectedId(organizationId);
  };
  const closeOrganization = () => {
    setSelectedId(null);
    setSelected(null);
    setDetailError(null);
    setDetailLoading(false);
  };

  return (
    <main className="admin-content">
      <section className="admin-heading">
        <div>
          <p>Tenant management</p>
          <h1>Developer organizations</h1>
          <span>
            Organizations created through Emisell&apos;s invite-only developer
            program.
          </span>
        </div>
        <button onClick={onOpenQueue}>
          <ClipboardCheck size={16} />
          Open review queue
        </button>
      </section>
      <section className="admin-panel admin-organizations-panel">
        <header>
          <div>
            <p>Developer tenants</p>
            <h2>Onboarded organizations</h2>
          </div>
          <span>{organizations.length} organizations</span>
        </header>
        <div className="admin-organization-toolbar">
          <label>
            <Search size={15} />
            <input
              aria-label="Search organizations"
              placeholder="Search name or slug"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
          </label>
          <select
            aria-label="Filter organization status"
            value={status}
            onChange={(event) => setStatus(event.target.value)}
          >
            <option value="">All statuses</option>
            <option value="active">Active</option>
            <option value="suspended">Suspended</option>
          </select>
        </div>
        {loading ? (
          <div className="admin-empty">
            <LoaderCircle className="spin" size={21} />
            <span>Loading organizations…</span>
          </div>
        ) : error ? (
          <div className="admin-empty">
            <Activity size={21} />
            <span>{error}</span>
          </div>
        ) : organizations.length ? (
          <div className="admin-organization-table">
            <div className="admin-table-head">
              <span>Organization</span>
              <span>Usage</span>
              <span>Entitlement</span>
              <span>Status</span>
            </div>
            {organizations.map((organization) => (
              <button
                type="button"
                className="admin-organization-row"
                key={organization.id}
                onClick={() => openOrganization(organization.id)}
              >
                <div>
                  <span className="admin-company-mark">
                    {initials(organization.name)}
                  </span>
                  <span>
                    <strong>{organization.name}</strong>
                    <small>{organization.slug}</small>
                  </span>
                </div>
                <span>
                  <strong>
                    {organization.appCount} of{" "}
                    {organization.entitlement.maxApps} apps
                  </strong>
                  <small>
                    {organization.membershipCount}{" "}
                    {organization.membershipCount === 1 ? "member" : "members"}
                  </small>
                </span>
                <span>
                  <strong>
                    {organization.entitlement.sandboxAccess
                      ? "Sandbox enabled"
                      : "No sandbox access"}
                  </strong>
                  <small>
                    {organization.entitlement.productionAccess
                      ? "Production enabled"
                      : "Production not enabled"}
                  </small>
                </span>
                <span className={`admin-status ${organization.status}`}>
                  {organization.status}
                </span>
              </button>
            ))}
          </div>
        ) : (
          <div className="admin-empty">
            <Building2 size={22} />
            <strong>No matching organizations</strong>
            <span>Approved developer workspaces will appear here.</span>
          </div>
        )}
      </section>
      {selectedId ? (
        <div
          className="admin-drawer-backdrop"
          role="presentation"
          onMouseDown={closeOrganization}
        >
          <aside
            className="admin-organization-drawer"
            role="dialog"
            aria-modal="true"
            aria-label="Developer organization detail"
            onMouseDown={(event) => event.stopPropagation()}
          >
            {detailLoading ? (
              <div className="admin-drawer-loading">
                <LoaderCircle className="spin" size={23} />
                Loading organization detail…
              </div>
            ) : detailError ? (
              <div className="admin-drawer-loading">
                <Activity size={22} />
                {detailError}
                <button onClick={closeOrganization}>Close</button>
              </div>
            ) : selected ? (
              <>
                <header>
                  <div>
                    <span className="admin-company-mark">
                      {initials(selected.name)}
                    </span>
                    <span>
                      <p>Developer organization</p>
                      <h2>{selected.name}</h2>
                      <small>{selected.slug}</small>
                    </span>
                  </div>
                  <button
                    aria-label="Close organization detail"
                    onClick={closeOrganization}
                  >
                    <X size={17} />
                  </button>
                </header>
                <div className="admin-drawer-body">
                  <section className="admin-detail-summary">
                    <div>
                      <small>Status</small>
                      <span className={`admin-status ${selected.status}`}>
                        {selected.status}
                      </span>
                    </div>
                    <div>
                      <small>Created</small>
                      <strong>{formatAdminDate(selected.createdAt)}</strong>
                    </div>
                  </section>
                  <section className="admin-detail-section">
                    <p>Entitlements</p>
                    <div className="admin-entitlement-grid">
                      <div>
                        <span>
                          <ShieldCheck size={16} />
                        </span>
                        <small>Sandbox access</small>
                        <strong>
                          {selected.entitlement.sandboxAccess
                            ? "Enabled"
                            : "Disabled"}
                        </strong>
                      </div>
                      <div>
                        <span>
                          <Network size={16} />
                        </span>
                        <small>Production access</small>
                        <strong>
                          {selected.entitlement.productionAccess
                            ? "Enabled"
                            : "Not enabled"}
                        </strong>
                      </div>
                      <div>
                        <span>
                          <Boxes size={16} />
                        </span>
                        <small>Application usage</small>
                        <strong>
                          {selected.appCount} / {selected.entitlement.maxApps}
                        </strong>
                      </div>
                      <div>
                        <span>
                          <Activity size={16} />
                        </span>
                        <small>Webhook limit</small>
                        <strong>{selected.entitlement.maxWebhooks}</strong>
                      </div>
                    </div>
                  </section>
                  <section className="admin-detail-section">
                    <p>Memberships · {selected.membershipCount}</p>
                    {selected.memberships.length ? (
                      <div className="admin-member-list">
                        {selected.memberships.map((member) => (
                          <div key={member.userId}>
                            <span>{initials(member.displayName)}</span>
                            <div>
                              <strong>{member.displayName}</strong>
                              <small>{member.email}</small>
                            </div>
                            <em>{member.role}</em>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <div className="admin-no-members">
                        <Users size={18} />
                        No accepted organization membership yet.
                      </div>
                    )}
                  </section>
                </div>
              </>
            ) : null}
          </aside>
        </div>
      ) : null}
    </main>
  );
}

function AdminAccessState({ loading = false, error }: { loading?: boolean; error?: string | null }) {
  return (
    <main className="admin-access-state">
      <section>
        <span>
          {loading ? (
            <LoaderCircle className="spin" size={23} />
          ) : (
            <LockKeyhole size={23} />
          )}
        </span>
        <p>Emisell Admin</p>
        <h1>
          {loading ? "Verifying operator access…" : "Admin access required"}
        </h1>
        <small>
          {loading
            ? "Checking your signed platform identity."
            : error ?? "This console is restricted to authorized Emisell platform operators."}
        </small>
        {!loading ? (
          <a href="/admin/login">
            Sign in to Admin Console <ArrowRight size={14} />
          </a>
        ) : null}
      </section>
    </main>
  );
}

function initials(value: string) {
  return (
    value
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((part) => part[0])
      .join("")
      .toUpperCase() || "EA"
  );
}

function formatAdminDate(value: string) {
  return new Intl.DateTimeFormat("en", {
    day: "2-digit",
    month: "short",
    year: "numeric",
  }).format(new Date(value));
}
