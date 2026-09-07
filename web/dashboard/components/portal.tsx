'use client';

import { lazy, Suspense, useCallback, useEffect, useState } from 'react';
import {
  ArrowLeft,
  ChevronDown,
  ArrowUpRight,
  BookOpen,
  CheckCircle2,
  Code2,
  FileCode2,
  FlaskConical,
  Inbox,
  KeyRound,
  LogOut,
  Plus,
  RefreshCw,
  Send,
  ShieldCheck,
  LayoutDashboard,
  Search,
  Eye,
  EyeOff,
  Users,
  Puzzle,
} from 'lucide-react';
import AdminOverview from '@/components/admin-overview';
import { adminNavigationGroups } from '@/lib/admin-navigation';
import AdminApps from '@/components/admin-apps';
import AdminDevelopers from '@/components/admin-developers';
import AdminActivity from '@/components/admin-activity';
import AdminStaff from '@/components/admin-staff';
import AdminReviews from '@/components/admin-reviews';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import {
  NativeSelect,
  NativeSelectOption,
} from '@/components/ui/native-select';
import {
  Empty,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
  EmptyDescription,
} from '@/components/ui/empty';
import { Skeleton } from '@/components/ui/skeleton';
import { portalView, portalDocsGroup } from '@/lib/surfaces';
import AccessScopes from '@/components/access-scopes';
import DeveloperApps from '@/components/developer-apps';
import { ScopeSummary } from '@/components/scope-summary';
import {
  publicDistributionAllowed,
  paymentBoundaryMessage,
} from '@/lib/distribution';
import {
  CatalogPanel,
  DeveloperTools,
  type CatalogRelease,
} from '@/components/catalog-panel';
import {
  Sidebar,
  SidebarProvider,
  SidebarHeader,
  SidebarContent,
  SidebarGroup,
  SidebarGroupLabel,
  SidebarFooter,
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuButton,
  SidebarTrigger,
  useSidebar,
} from '@/components/ui/sidebar';
import {
  PortalAPI,
  PortalError,
  blankDocument,
  scopesFor,
  statuses,
  type Surface,
  type Session,
  type Draft,
  type Submission,
  type Audit,
  type AppDocument,
} from '@/lib/portal';

const APIDocs = lazy(() => import('@/components/api-docs'));
const APIKeys = lazy(() => import('@/components/api-keys'));
const AppClients = lazy(() => import('@/components/app-clients'));
const Testing = lazy(() => import('@/components/testing'));
const IntegrationReleases = lazy(
  () => import('@/components/integration-releases'),
);
const UIReleases = lazy(() => import('@/components/ui-releases'));

const date = (value: string) =>
  new Date(value).toLocaleString('id-ID', {
    dateStyle: 'medium',
    timeStyle: 'short',
  });
function Status({ value }: { value: string }) {
  return (
    <span className={`status status-${value}`}>{statuses[value] ?? value}</span>
  );
}
function Brand() {
  return (
    <div className="portal-brand">
      <span>e</span>emisell
      <span className="brand-divider" />
      apps
    </div>
  );
}
function Navigation({
  developer,
  view,
  navigate,
  session,
  logout,
  busy,
}: {
  developer: boolean;
  view: string;
  navigate: (view: string) => void;
  session: Session;
  logout: () => void;
  busy: boolean;
}) {
  const { setOpenMobile } = useSidebar();
  const [technicalOpen, setTechnicalOpen] = useState(false);
  useEffect(() => {
    if (adminNavigationGroups.some(group => group.collapsible && group.ids.includes(view))) {
      setTechnicalOpen(true);
    }
  }, [view]);
  const items = developer
    ? [
        { id: 'apps', label: 'Aplikasi saya', icon: FileCode2 },
        { id: 'testing', label: 'Testing', icon: FlaskConical },
        { id: 'app-clients', label: 'App clients', icon: KeyRound },
        { id: 'integration-releases', label: 'Rilis integrasi', icon: Code2 },
        { id: 'ui-releases', label: 'Aplikasi dengan UI', icon: FileCode2 },
        { id: 'reviews', label: 'Pengajuan review', icon: Inbox },
        { id: 'catalog', label: 'Rilis & publikasi', icon: CheckCircle2 },
        { id: 'tooling', label: 'Developer tools', icon: Code2 },
        { id: 'scopes', label: 'Katalog scope', icon: ShieldCheck },
      ]
    : [
        { id: 'overview', label: 'Ringkasan', icon: LayoutDashboard },
        { id: 'apps', label: 'Aplikasi', icon: FileCode2 },
        { id: 'developers', label: 'Developer', icon: Users },
        { id: 'activity', label: 'Aktivitas', icon: FileCode2 },
        { id: 'staff', label: 'Kelola staf', icon: Users },
        { id: 'reviews', label: 'Pengajuan review', icon: ShieldCheck },
        { id: 'testing', label: 'Pengujian', icon: FlaskConical },
        { id: 'app-clients', label: 'App clients', icon: KeyRound },
        { id: 'integration-releases', label: 'Rilis integrasi', icon: Code2 },
        { id: 'ui-releases', label: 'Rilis UI', icon: FileCode2 },
        { id: 'catalog', label: 'Rilis & publikasi', icon: CheckCircle2 },
        { id: 'api-docs', label: 'Dokumentasi API', icon: BookOpen },
        { id: 'api-keys', label: 'API key', icon: KeyRound },
        { id: 'scopes', label: 'Katalog scope', icon: ShieldCheck },
      ];
  const renderItems = (groupItems: typeof items) => (
    <SidebarMenu>
      {groupItems.map(item => (
        <SidebarMenuItem key={item.id}>
          <SidebarMenuButton
            isActive={view === item.id}
            aria-current={view === item.id ? 'page' : undefined}
            disabled={busy}
            onClick={() => { navigate(item.id); setOpenMobile(false); }}
          >
            <item.icon />
            <span>{item.label}</span>
          </SidebarMenuButton>
        </SidebarMenuItem>
      ))}
    </SidebarMenu>
  );
  return (
    <Sidebar collapsible="offcanvas">
      <SidebarHeader>
        <Brand />
        <div className="surface-label">
          {developer ? 'DEVELOPER PORTAL' : 'ADMIN PLATFORM'}
        </div>
      </SidebarHeader>
      <SidebarContent>
        {developer ? renderItems(items) : adminNavigationGroups.map(group => {
          const groupItems = group.ids.flatMap(id => items.filter(item => item.id === id));
          return (
            <SidebarGroup key={group.label}>
              {group.collapsible ? (
                <>
                  <button
                    type="button"
                    className="flex min-h-9 items-center justify-between rounded-md px-2 text-xs font-medium text-muted-foreground hover:bg-muted focus-visible:outline-2 focus-visible:outline-offset-2"
                    aria-expanded={technicalOpen}
                    aria-controls="admin-technical-navigation"
                    onClick={() => setTechnicalOpen(open => !open)}
                  >
                    {group.label}
                    <ChevronDown aria-hidden="true" className={`size-3.5 transition-transform ${technicalOpen ? '' : '-rotate-90'}`} />
                  </button>
                  <div id="admin-technical-navigation" hidden={!technicalOpen}>
                    {renderItems(groupItems)}
                  </div>
                </>
              ) : <><SidebarGroupLabel>{group.label}</SidebarGroupLabel>{renderItems(groupItems)}</>}
            </SidebarGroup>
          );
        })}
        <div className="sidebar-note">
          <span className="local-dot" />
          Emisell Apps<p>Platform aplikasi dan integrasi.</p>
        </div>
      </SidebarContent>
      <SidebarFooter>
        <div className="portal-account">
          <strong>{session.organization?.name ?? 'Tim Emisell'}</strong>
          <span>{session.user.email}</span>
          <small>{session.user.role}</small>
        </div>
        <Button variant="ghost" disabled={busy} onClick={logout}>
          <LogOut />
          Keluar
        </Button>
      </SidebarFooter>
    </Sidebar>
  );
}
export default function Portal({
  surface,
  unifiedLogin,
  onSignedOut,
}: {
  surface: Surface;
  unifiedLogin?: (email: string, password: string) => Promise<void>;
  onSignedOut?: () => void;
}) {
  const developer = surface === 'developer';
  const [api] = useState(() => new PortalAPI(surface));
  const [session, setSession] = useState<Session | null>(null);
  const [loading, setLoading] = useState(true);
  const [dataReady, setDataReady] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [view, setView] = useState(developer ? 'apps' : 'overview');
  const [drafts, setDrafts] = useState<Draft[]>([]);
  const [submissions, setSubmissions] = useState<Submission[]>([]);
  const [releases, setReleases] = useState<CatalogRelease[]>([]);
  const [editor, setEditor] = useState<Draft | 'new' | null>(null);
  const [detail, setDetail] = useState<{
    submission: Submission;
    history: Audit[];
  } | null>(null);
  const [search, setSearch] = useState('');
  const [filter, setFilter] = useState('all');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [docsGroup, setDocsGroup] = useState('admin');
  const [docsOperation, setDocsOperation] = useState('');
  const [scopeSelection, setScopeSelection] = useState('');
  useEffect(() => {
    const restore = () => {
      setView(portalView(surface, window.location.search));
      const params = new URLSearchParams(window.location.search);
      setDocsOperation((params.get('api_operation') ?? '').slice(0, 512));
      setScopeSelection((params.get('scope') ?? '').slice(0, 128));
      setDocsGroup(portalDocsGroup(window.location.search));
      setEditor(null);
      setDetail(null);
    };
    restore();
    window.addEventListener('popstate', restore);
    return () => window.removeEventListener('popstate', restore);
  }, [surface]);
  const refresh = useCallback(async () => {
    const [reviews, apps, catalog] = await Promise.all([
      api.request<{ submissions: Submission[] }>('/submissions'),
      developer
        ? api.request<{ apps: Draft[] }>('/apps')
        : Promise.resolve({ apps: [] }),
      api.request<{ releases: CatalogRelease[] }>('/catalog'),
    ]);
    setSubmissions(reviews.submissions);
    setDrafts(apps.apps);
    setReleases(catalog.releases);
    setDataReady(true);
  }, [api, developer]);
  useEffect(() => {
    let alive = true;
    api
      .request<Session>('/session')
      .then(async (value) => {
        if (!alive) return;
        setSession(value);
        await refresh();
      })
      .catch((err) => {
        if (alive && !(err instanceof PortalError && err.status === 401))
          setError(err.message);
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [api, refresh]);
  const perform = async (action: () => Promise<void>, message = '') => {
    if (busy) return;
    setBusy(true);
    setError('');
    setNotice('');
    try {
      await action();
      setNotice(message);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Permintaan gagal.');
      if (err instanceof PortalError && err.status === 401 && session) {
        setSession(null);
        setDataReady(false);
        setDrafts([]);
        setSubmissions([]);
        setReleases([]);
        setEditor(null);
        setDetail(null);
      }
    } finally {
      setBusy(false);
    }
  };
  const navigate = (next: string) => {
    const url = new URL(window.location.href);
    url.searchParams.set('view', next);
    window.history.pushState(null, '', url);
    setEditor(null);
    setDetail(null);
    setView(next);
    setSearch('');
    setFilter('all');
    setNotice('');
    setError('');
  };
  const openSubmission = (id: string) =>
    perform(async () => {
      const value = await api.request<{
        submission: Submission;
        history: Audit[];
      }>(`/submissions/${id}`);
      setDetail(value);
      setEditor(null);
      setView('reviews');
    });
  const openDraft = (id: string) =>
    perform(async () => {
      const value = await api.request<{ app: Draft }>(`/apps/${id}`);
      setEditor(value.app);
      setDetail(null);
    });
  const banners = (
    <>
      {error && (
        <div className="portal-message error" role="alert">
          {error}
        </div>
      )}
      {notice && <output className="portal-message success">{notice}</output>}
    </>
  );
  if (loading)
    return (
      <main className="portal-loading" aria-label="Memuat portal">
        <Brand />
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-40 w-full" />
      </main>
    );
  if (!session)
    return (
      <main className={`portal-login ${developer ? '' : 'admin-login'}`}>
        <div className="login-intro">
          <Brand />
          <div className="login-icon" hidden={!developer}>
            {developer ? <Code2 /> : <ShieldCheck />}
          </div>
          <p className="eyebrow">
            {developer ? 'DEVELOPER PORTAL' : 'PLATFORM ADMIN'}
          </p>
          <h1>
            {developer
              ? 'Tempat aplikasi Anda dimulai.'
              : 'Kelola ekosistem aplikasi Emisell.'}
          </h1>
          <p>
            {developer
              ? 'Siapkan aplikasi, simpan draft, dan ajukan versi untuk review.'
              : 'Satu platform untuk aplikasi, developer, dan integrasi.'}
          </p>
          {!developer && (
            <div className="login-features">
              <div>
                <LayoutDashboard />
                <section>
                  <h3>Aplikasi</h3>
                  <p>Kelola aplikasi dan pengajuan dalam satu platform.</p>
                </section>
              </div>
              <div>
                <Users />
                <section>
                  <h3>Developer</h3>
                  <p>Pantau organisasi pembuat aplikasi.</p>
                </section>
              </div>
              <div>
                <Puzzle />
                <section>
                  <h3>Integrasi</h3>
                  <p>
                    Hubungkan API dan layanan untuk alur kerja yang efisien.
                  </p>
                </section>
              </div>
            </div>
          )}
          <span className="local-chip">
            <span className="local-dot" />
            Emisell Apps Platform
          </span>
        </div>
        <section className="login-panel">
          {!developer && (
            <p className="eyebrow">
              {unifiedLogin ? 'EMISELL APPS' : 'ADMIN PLATFORM'}
            </p>
          )}
          <h2>
            {developer ? 'Masuk sebagai developer' : 'Masuk ke dashboard'}
          </h2>
          <p>
            {unifiedLogin
              ? 'Masuk dengan akun Admin, staf, atau Developer Anda.'
              : `Gunakan akun khusus ${developer ? 'developer' : 'administrasi'} platform.`}
          </p>
          {banners}
          <form
            onSubmit={(event) => {
              event.preventDefault();
              void perform(async () => {
                if (unifiedLogin) {
                  await unifiedLogin(email, password);
                  setPassword('');
                  return;
                }
                await api.request('/login', 'POST', { email, password });
                setPassword('');
                setShowPassword(false);
                setSession(await api.request<Session>('/session'));
                await refresh();
              });
            }}
          >
            <label htmlFor="login-email">
              Email
              <Input
                id="login-email"
                required
                type="email"
                autoComplete="username"
                placeholder="nama@emisell.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                disabled={busy}
              />
            </label>
            <label htmlFor="login-password">
              Kata sandi
              <span className="login-password-field">
                <Input
                  id="login-password"
                  required
                  type={showPassword ? 'text' : 'password'}
                  autoComplete="current-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  disabled={busy}
                />
                <Button
                  type="button"
                  variant="ghost"
                  disabled={busy}
                  aria-label={
                    showPassword
                      ? 'Sembunyikan kata sandi'
                      : 'Tampilkan kata sandi'
                  }
                  aria-pressed={showPassword}
                  onClick={() => setShowPassword(!showPassword)}
                >
                  {showPassword ? <EyeOff /> : <Eye />}
                </Button>
              </span>
            </label>
            <Button type="submit" disabled={busy}>
              {busy ? 'Memeriksa…' : 'Masuk'}
              {developer && <ArrowUpRight />}
            </Button>
          </form>
          <p className="login-help">
            Akun disediakan oleh pengelola platform. Akun toko Emisell tidak
            memiliki akses ke portal ini.
          </p>
        </section>
      </main>
    );
  const visibleDrafts = drafts.filter((d) =>
    `${d.document.name} ${d.document.capability}`
      .toLowerCase()
      .includes(search.toLowerCase()),
  );
  const visibleReviews = submissions.filter(
    (s) =>
      (filter === 'all' || s.status === filter) &&
      `${s.snapshot.name} ${s.version}`
        .toLowerCase()
        .includes(search.toLowerCase()),
  );
  const latest = (id: string) => submissions.find((s) => s.appId === id);
  return (
    <SidebarProvider className={`portal ${developer ? '' : 'admin-redesign'}`}>
      <Navigation
        developer={developer}
        view={view}
        navigate={navigate}
        session={session}
        busy={busy}
        logout={() =>
          void perform(async () => {
            await api.request('/logout', 'POST', {});
            onSignedOut?.();
            setSession(null);
            setDataReady(false);
            setDrafts([]);
            setSubmissions([]);
            setReleases([]);
            setEditor(null);
            setDetail(null);
          })
        }
      />
      <div className="portal-body">
        <header className="portal-topbar">
          <SidebarTrigger aria-label="Buka navigasi" />
          <span>
            {developer
              ? 'Portal Developer'
              : view === 'staff'
                ? 'Platform / Pengaturan / Staf'
                : view === 'activity'
                  ? 'Platform / Aktivitas'
                  : view === 'developers'
                    ? 'Platform / Developer'
                    : view === 'apps'
                      ? 'Platform / Aplikasi'
                      : `Platform / ${view === 'overview' ? 'Ringkasan' : view === 'reviews' ? 'Pengajuan review' : view === 'ui-releases' ? 'Rilis UI' : view === 'scopes' ? 'Katalog scope' : view === 'api-docs' ? 'Dokumentasi API' : view === 'api-keys' ? 'API keys' : view === 'catalog' ? 'Rilis & publikasi' : view === 'testing' ? 'Testing' : view === 'app-clients' ? 'App clients' : 'Rilis integrasi'}`}
          </span>
          {!developer && (
            <form
              className="admin-header-search"
              onSubmit={(event) => {
                event.preventDefault();
                const term = search;
                navigate('reviews');
                setSearch(term);
              }}
            >
              <Search size={18} />
              <Input
                aria-label="Cari pengajuan aplikasi"
                placeholder="Cari pengajuan aplikasi…"
                value={search}
                onChange={(event) => setSearch(event.target.value)}
              />
            </form>
          )}
          <span className="topbar-environment">
            <span className="local-dot" />
            Lokal
          </span>
        </header>
        <main className="portal-main">
          {banners}
          {editor ? (
            <DraftEditor
              api={api}
              key={editor === 'new' ? 'new' : `${editor.id}-${editor.revision}`}
              draft={editor}
              busy={busy}
              back={() => setEditor(null)}
              save={(document) =>
                void perform(async () => {
                  const value = await api.request<{ app: Draft }>(
                    editor === 'new' ? '/apps' : `/apps/${editor.id}`,
                    editor === 'new' ? 'POST' : 'PUT',
                    {
                      revision: editor === 'new' ? 0 : editor.revision,
                      document,
                    },
                    true,
                  );
                  setEditor(value.app);
                  await refresh();
                }, 'Draft tersimpan.')
              }
              submit={() => {
                if (editor !== 'new')
                  void perform(async () => {
                    const value = await api.request<{ submission: Submission }>(
                      `/apps/${editor.id}/submissions`,
                      'POST',
                      { revision: editor.revision },
                      true,
                    );
                    setEditor(null);
                    setView('reviews');
                    setDetail(
                      await api.request(`/submissions/${value.submission.id}`),
                    );
                    await refresh();
                  }, 'Versi diajukan. Menunggu keputusan reviewer.');
              }}
              latest={editor === 'new' ? undefined : latest(editor.id)}
            />
          ) : detail ? (
            <SubmissionDetail
              value={detail}
              busy={busy}
              canReview={
                !developer &&
                ['administrator', 'reviewer'].includes(session.user.role)
              }
              back={() => setDetail(null)}
              decide={(status, feedback) =>
                void perform(async () => {
                  await api.request(
                    `/submissions/${detail.submission.id}/decision`,
                    'POST',
                    { status, feedback },
                    true,
                  );
                  setDetail(
                    await api.request(`/submissions/${detail.submission.id}`),
                  );
                  await refresh();
                }, 'Keputusan review tersimpan.')
              }
              edit={
                developer
                  ? () => void openDraft(detail.submission.appId)
                  : undefined
              }
            />
          ) : view === 'scopes' ? (
            <AccessScopes
              key={scopeSelection}
              api={api}
              initialScope={scopeSelection}
            />
          ) : view === 'api-keys' && !developer ? (
            <Suspense fallback={<Skeleton className="h-40 w-full" />}>
              <APIKeys
                api={api}
                role={session.user.role}
                onDocumentation={() => navigate('api-docs')}
              />
            </Suspense>
          ) : view === 'api-docs' && !developer ? (
            <Suspense
              fallback={
                <section aria-label="Memuat dokumentasi API">
                  <Skeleton className="h-40 w-full" />
                </section>
              }
            >
              <APIDocs
                key={docsGroup + docsOperation}
                api={api}
                initialGroup={docsGroup}
                initialOperation={docsOperation}
              />
            </Suspense>
          ) : !dataReady ? (
            <section className="portal-panel">
              <h1>Data belum berhasil dimuat</h1>
              <p>Periksa layanan platform, lalu coba lagi.</p>
              <Button disabled={busy} onClick={() => void perform(refresh)}>
                <RefreshCw />
                Muat ulang
              </Button>
            </section>
          ) : view === 'staff' && !developer ? (
            <AdminStaff api={api} session={session} />
          ) : view === 'activity' && !developer ? (
            <AdminActivity api={api} submissions={submissions} />
          ) : view === 'developers' && !developer ? (
            <AdminDevelopers api={api} role={session.user.role} />
          ) : view === 'apps' && !developer ? (
            <AdminApps
              submissions={submissions}
              releases={releases}
              busy={busy}
              refresh={() => void perform(refresh)}
              navigate={navigate}
              openReview={(id) =>
                void perform(async () => {
                  setDetail(await api.request(`/submissions/${id}`));
                })
              }
            />
          ) : view === 'overview' && !developer ? (
            <AdminOverview
              submissions={submissions}
              navigate={navigate}
              busy={busy}
              refresh={() => void perform(refresh)}
              openReview={(id) =>
                void perform(async () => {
                  setDetail(await api.request(`/submissions/${id}`));
                })
              }
            />
          ) : view === 'testing' ? (
            <Suspense fallback={<Skeleton className="h-40 w-full" />}>
              <Testing
                api={api}
                session={session}
                busy={busy}
                perform={perform}
              />
            </Suspense>
          ) : view === 'app-clients' ? (
            <Suspense fallback={<Skeleton className="h-40 w-full" />}>
              <AppClients
                api={api}
                session={session}
                busy={busy}
                perform={perform}
              />
            </Suspense>
          ) : view === 'ui-releases' ? (
            <Suspense fallback={<Skeleton className="h-40 w-full" />}>
              <UIReleases api={api} session={session} />
            </Suspense>
          ) : view === 'integration-releases' ? (
            <Suspense fallback={<Skeleton className="h-40 w-full" />}>
              <IntegrationReleases
                api={api}
                session={session}
                submissions={submissions}
                busy={busy}
                perform={perform}
              />
            </Suspense>
          ) : view === 'catalog' ? (
            <CatalogPanel
              api={api}
              session={session}
              releases={releases}
              submissions={submissions}
              busy={busy}
              perform={perform}
              refresh={refresh}
            />
          ) : view === 'reviews' && !developer ? (
            <AdminReviews
              submissions={submissions}
              busy={busy}
              search={search}
              setSearch={setSearch}
              open={(id) => void openSubmission(id)}
              refresh={() => void perform(refresh)}
            />
          ) : view === 'tooling' && developer ? (
            <DeveloperTools
              api={api}
              drafts={drafts}
              busy={busy}
              perform={perform}
            />
          ) : (
            <>
              <div className="page-heading">
                <div>
                  <p className="eyebrow">
                    {view === 'apps' ? 'BUILD & MANAGE' : 'APPLICATION REVIEW'}
                  </p>
                  <h1>
                    {view === 'apps'
                      ? 'Aplikasi saya'
                      : developer
                        ? 'Pengajuan review'
                        : 'Review aplikasi'}
                  </h1>
                  <p>
                    {view === 'apps'
                      ? 'Aplikasi yang dikembangkan oleh organisasi Anda.'
                      : developer
                        ? 'Ikuti keputusan dan feedback untuk setiap versi.'
                        : 'Periksa versi yang diajukan, lalu berikan keputusan.'}
                  </p>
                </div>
                {view === 'apps' ? (
                  <Button
                    disabled={busy}
                    onClick={() => {
                      setEditor('new');
                      setNotice('');
                      setError('');
                    }}
                  >
                    <Plus />
                    Buat aplikasi
                  </Button>
                ) : (
                  <Button
                    variant="outline"
                    disabled={busy}
                    onClick={() => void perform(refresh)}
                  >
                    <RefreshCw />
                    Muat ulang
                  </Button>
                )}
              </div>
              <div className="portal-callout">
                <ShieldCheck />
                <span>
                  Persetujuan review belum memublikasikan aplikasi. Kelola
                  penandatanganan metadata dan publikasi melalui Rilis &
                  publikasi.
                </span>
              </div>
              <div className="list-toolbar">
                <Input
                  aria-label="Cari aplikasi"
                  placeholder="Cari nama aplikasi…"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
                {view === 'reviews' && (
                  <NativeSelect
                    aria-label="Filter status"
                    value={filter}
                    onChange={(e) => setFilter(e.target.value)}
                  >
                    <NativeSelectOption value="all">
                      Semua status
                    </NativeSelectOption>
                    {Object.entries(statuses).map(([value, label]) => (
                      <NativeSelectOption key={value} value={value}>
                        {label}
                      </NativeSelectOption>
                    ))}
                  </NativeSelect>
                )}
                <span>
                  Maks. 200 {view === 'apps' ? 'aplikasi' : 'pengajuan'} terbaru
                </span>
              </div>
              {(
                view === 'apps'
                  ? visibleDrafts.length === 0
                  : visibleReviews.length === 0
              ) ? (
                <Empty className="portal-empty">
                  <EmptyHeader>
                    <EmptyMedia variant="icon">
                      {view === 'apps' ? <FileCode2 /> : <Inbox />}
                    </EmptyMedia>
                    <EmptyTitle>
                      {search || filter !== 'all'
                        ? 'Tidak ada hasil yang cocok'
                        : view === 'apps'
                          ? 'Mulai dengan aplikasi pertama'
                          : 'Belum ada pengajuan'}
                    </EmptyTitle>
                    <EmptyDescription>
                      {view === 'apps'
                        ? 'Buat draft Aplikasi Integrasi untuk capability payment atau shipping.'
                        : 'Pengajuan developer akan muncul di sini setelah dikirim.'}
                    </EmptyDescription>
                  </EmptyHeader>
                </Empty>
              ) : (
                <div className={view === 'apps' ? undefined : 'app-list'}>
                  {view === 'apps' ? (
                    <DeveloperApps
                      key={session.user.id}
                      api={api}
                      drafts={visibleDrafts}
                      busy={busy}
                      openDraft={(id) => void openDraft(id)}
                    />
                  ) : (
                    visibleReviews.map((s) => (
                      <button
                        className="app-row"
                        key={s.id}
                        disabled={busy}
                        onClick={() => void openSubmission(s.id)}
                      >
                        <span className="app-glyph">
                          <FileCode2 />
                        </span>
                        <span className="app-row-main">
                          <strong>{s.snapshot.name}</strong>
                          <span>{date(s.createdAt)}</span>
                        </span>
                        <span className="row-version">
                          v{s.version}
                          <small>Draft r{s.draftRevision}</small>
                        </span>
                        <Status value={s.status} />
                        <ArrowUpRight className="row-arrow" />
                      </button>
                    ))
                  )}
                </div>
              )}
            </>
          )}
        </main>
      </div>
    </SidebarProvider>
  );
}

function DraftEditor({
  api,
  draft,
  busy,
  back,
  save,
  submit,
  latest,
}: {
  api: PortalAPI;
  draft: Draft | 'new';
  busy: boolean;
  back: () => void;
  save: (doc: AppDocument) => void;
  submit: () => void;
  latest?: Submission;
}) {
  const [document, setDocument] = useState<AppDocument>(
    draft === 'new' ? blankDocument() : draft.document,
  );
  const [scopeValid, setScopeValid] = useState(true);
  const reserved =
    draft !== 'new' && !publicDistributionAllowed(draft.document.capability);
  const dirty =
    draft === 'new' ||
    JSON.stringify(document) !== JSON.stringify(draft.document);
  const complete =
    document.summary.trim() && document.description.trim() && document.endpoint;
  const sameRevision =
    draft !== 'new' && latest?.draftRevision === draft.revision;
  const blockedVersion =
    latest &&
    ['approved', 'rejected'].includes(latest.status) &&
    latest.version === document.version;
  const field = (name: keyof AppDocument, value: string) =>
    setDocument((previous) => ({ ...previous, [name]: value }));
  return (
    <>
      <Button variant="ghost" disabled={busy} onClick={back}>
        <ArrowLeft />
        Kembali
      </Button>
      <div className="page-heading">
        <div>
          <p className="eyebrow">APLIKASI INTEGRASI</p>
          <h1>{draft === 'new' ? 'Buat aplikasi' : draft.document.name}</h1>
          <p>
            {draft === 'new'
              ? 'Mulai dari identitas dan kontrak aplikasi.'
              : `Draft revisi ${draft.revision} · ${date(draft.updatedAt)}`}
          </p>
        </div>
        <span className="status status-draft">Draft</span>
      </div>
      <div className="editor-grid">
        <form
          className="portal-panel app-form"
          onSubmit={(e) => {
            e.preventDefault();
            save(document);
          }}
        >
          <div className="panel-title">
            <FileCode2 />
            <h2>Informasi aplikasi</h2>
          </div>
          <p className="field-help">{paymentBoundaryMessage}</p>
          <fieldset disabled={busy || reserved}>
            <label htmlFor="app-name">
              Nama aplikasi
              <Input
                id="app-name"
                required
                maxLength={100}
                value={document.name}
                onChange={(e) => field('name', e.target.value)}
                placeholder="Contoh: Nusa Shipping"
              />
            </label>
            <label htmlFor="app-summary">
              Ringkasan
              <Input
                id="app-summary"
                maxLength={180}
                value={document.summary}
                onChange={(e) => field('summary', e.target.value)}
                placeholder="Jelaskan kegunaan aplikasi dalam satu kalimat"
              />
            </label>
            <label htmlFor="app-description">
              Deskripsi
              <Textarea
                id="app-description"
                rows={4}
                maxLength={5000}
                value={document.description}
                onChange={(e) => field('description', e.target.value)}
              />
            </label>
            <div className="form-columns">
              <label htmlFor="app-version">
                Versi
                <Input
                  id="app-version"
                  required
                  pattern="(0|[1-9][0-9]{0,5})\.(0|[1-9][0-9]{0,5})\.(0|[1-9][0-9]{0,5})"
                  value={document.version}
                  onChange={(e) => field('version', e.target.value)}
                />
                <small>Format: 1.0.0</small>
              </label>
              <label htmlFor="app-capability">
                Capability
                <NativeSelect
                  id="app-capability"
                  value={document.capability}
                  onChange={(e) =>
                    setDocument((previous) => ({
                      ...previous,
                      capability: e.target.value,
                      scopes: scopesFor(e.target.value),
                    }))
                  }
                >
                  {reserved && (
                    <NativeSelectOption value={document.capability} disabled>
                      {document.capability} · historis, hanya-baca
                    </NativeSelectOption>
                  )}
                  <NativeSelectOption value="shipping/v1">
                    shipping/v1
                  </NativeSelectOption>
                </NativeSelect>
              </label>
            </div>
            <label htmlFor="app-endpoint">
              Endpoint Aplikasi Integrasi
              <Input
                id="app-endpoint"
                type="url"
                pattern="https://.*"
                placeholder="https://app.example.com/emisell/v1"
                value={document.endpoint}
                onChange={(e) => field('endpoint', e.target.value)}
                maxLength={2048}
              />
              <small>
                HTTPS. Disimpan sebagai konfigurasi review; belum dipanggil oleh
                runtime.
              </small>
            </label>
            <p className="field-help">
              Scope fixture capability (terpisah dari akses data)
            </p>
            <div className="scope-list">
              {document.scopes.map((scope) => (
                <code key={scope}>{scope}</code>
              ))}
            </div>
            <p className="field-help">
              Reference shipping adalah kontrak layanan, bukan provider kurir
              tertentu. Scope ini bukan scope resource Shopify dan tidak
              diterjemahkan otomatis.
            </p>
            <AccessScopes
              api={api}
              value={document.accessScopes}
              disabled={busy}
              onValidity={setScopeValid}
              onChange={(accessScopes) =>
                setDocument((previous) => ({ ...previous, accessScopes }))
              }
            />
          </fieldset>
          <Button
            type="submit"
            disabled={busy || reserved || !dirty || !scopeValid}
          >
            {busy ? 'Memproses…' : 'Simpan draft'}
          </Button>
        </form>
        <aside className="portal-panel review-guide">
          <p className="eyebrow">LANGKAH BERIKUTNYA</p>
          <h2>Siap untuk review?</h2>
          <p>
            Lengkapi ringkasan, deskripsi, versi, dan endpoint. Simpan perubahan
            sebelum mengajukan.
          </p>
          <ul>
            <li>
              <CheckCircle2 />
              Snapshot versi tidak dapat diubah.
            </li>
            <li>
              <CheckCircle2 />
              Feedback tersimpan per pengajuan.
            </li>
            <li>
              <CheckCircle2 />
              Tidak langsung tampil di App Store.
            </li>
          </ul>
          {latest && (
            <div className="latest-review">
              <Status value={latest.status} />
              <p>{latest.feedback || 'Reviewer belum memberikan keputusan.'}</p>
            </div>
          )}
          <Button
            disabled={
              busy ||
              dirty ||
              !complete ||
              reserved ||
              !scopeValid ||
              sameRevision ||
              latest?.status === 'submitted' ||
              Boolean(blockedVersion)
            }
            onClick={submit}
          >
            <Send />
            Ajukan review
          </Button>
          {dirty && <small>Simpan draft terlebih dahulu.</small>}
          {sameRevision && (
            <small>
              Revisi ini sudah pernah diajukan. Simpan perbaikan untuk membuat
              revisi baru.
            </small>
          )}
          {blockedVersion && (
            <small>Versi ini telah diputuskan. Gunakan nomor versi baru.</small>
          )}
        </aside>
      </div>
    </>
  );
}
function SubmissionDetail({
  value,
  busy,
  canReview,
  back,
  decide,
  edit,
}: {
  value: { submission: Submission; history: Audit[] };
  busy: boolean;
  canReview: boolean;
  back: () => void;
  decide: (status: string, feedback: string) => void;
  edit?: () => void;
}) {
  const { submission: s, history } = value;
  const [feedback, setFeedback] = useState('');
  const [status, setStatus] = useState('changes_requested');
  return (
    <>
      <Button variant="ghost" disabled={busy} onClick={back}>
        <ArrowLeft />
        Kembali ke pengajuan
      </Button>
      <div className="page-heading">
        <div>
          <p className="eyebrow">SNAPSHOT PENGAJUAN · v{s.version}</p>
          <h1>{s.snapshot.name}</h1>
          <p>
            Diajukan {date(s.createdAt)} · Draft r{s.draftRevision}
          </p>
        </div>
        <Status value={s.status} />
      </div>
      <div className="editor-grid">
        <section className="portal-panel snapshot">
          <div className="panel-title">
            <FileCode2 />
            <h2>Versi yang direview</h2>
          </div>
          <p>{s.snapshot.summary}</p>
          <p className="preserve-lines">{s.snapshot.description}</p>
          <dl>
            <dt>App ID</dt>
            <dd>
              <code>{s.appId}</code>
            </dd>
            <dt>Organisasi developer</dt>
            <dd>
              <code>{s.organizationId}</code>
            </dd>
            <dt>Capability</dt>
            <dd>{s.snapshot.capability}</dd>
            <dt>Endpoint</dt>
            <dd>
              <code>{s.snapshot.endpoint}</code>
            </dd>
            <dt>Scope fixture capability</dt>
            <dd>{s.snapshot.scopes.join(', ')}</dd>
          </dl>
          <ScopeSummary value={s.snapshot.accessScopes} />
          {!publicDistributionAllowed(s.snapshot.capability) && (
            <p className="field-help">{paymentBoundaryMessage}</p>
          )}
          <p className="field-help">
            Snapshot hanya-baca. Endpoint belum dijalankan; keamanan runtime dan
            artifact belum disertifikasi.
          </p>
          {edit && (
            <Button variant="outline" disabled={busy} onClick={edit}>
              Buka draft aplikasi
            </Button>
          )}
        </section>
        <aside className="portal-panel">
          <h2>
            {s.status === 'submitted' ? 'Keputusan review' : 'Hasil review'}
          </h2>
          {canReview && s.status === 'submitted' ? (
            <form
              className="decision-form"
              onSubmit={(e) => {
                e.preventDefault();
                decide(status, feedback);
              }}
            >
              <label htmlFor="review-decision">
                Keputusan
                <NativeSelect
                  id="review-decision"
                  value={status}
                  disabled={busy}
                  onChange={(e) => setStatus(e.target.value)}
                >
                  <NativeSelectOption value="changes_requested">
                    Minta perbaikan
                  </NativeSelectOption>
                  <NativeSelectOption
                    value="approved"
                    disabled={!publicDistributionAllowed(s.snapshot.capability)}
                  >
                    Setujui review
                  </NativeSelectOption>
                  <NativeSelectOption value="rejected">
                    Tolak versi
                  </NativeSelectOption>
                </NativeSelect>
              </label>
              <label htmlFor="review-feedback">
                Catatan untuk developer
                <Textarea
                  id="review-feedback"
                  required
                  minLength={1}
                  maxLength={4000}
                  rows={5}
                  value={feedback}
                  disabled={busy}
                  onChange={(e) => setFeedback(e.target.value)}
                />
              </label>
              <p className="field-help">
                Keputusan bersifat final untuk pengajuan ini. Persetujuan bukan
                publikasi.
              </p>
              <Button type="submit" disabled={busy || !feedback.trim()}>
                Simpan keputusan
              </Button>
            </form>
          ) : (
            <p className="preserve-lines">
              {s.feedback ||
                (canReview
                  ? 'Belum ada keputusan.'
                  : 'Menunggu keputusan reviewer. Akses Anda hanya-baca.')}
            </p>
          )}
          <div className="review-history">
            <h3>Riwayat pengajuan</h3>
            {history.map((item) => (
              <article key={item.id}>
                <strong>{statuses[item.action] ?? item.action}</strong>
                <time>{date(item.occurredAt)}</time>
                {item.feedback && (
                  <p className="preserve-lines">{item.feedback}</p>
                )}
                <small>Oleh {item.actorId}</small>
              </article>
            ))}
          </div>
        </aside>
      </div>
    </>
  );
}
