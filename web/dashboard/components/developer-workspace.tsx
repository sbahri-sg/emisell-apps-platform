'use client';

import { useEffect, useState } from 'react';
import {
  ArrowLeft,
  ArrowRight,
  ArrowUpRight,
  Box,
  CalendarDays,
  Check,
  Copy,
  FileText,
  Info,
  Plus,
  Search,
  Terminal,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import DeveloperApps from '@/components/developer-apps';
import DeveloperInstallPicker from '@/components/developer-install-picker';
import {
  appLifecycleStatus,
  loadAppLifecycle,
  type AppLifecycle,
} from '@/lib/app-lifecycle';
import {
  developerOverview,
  loadApplicationIdentity,
} from '@/lib/developer-overview';
import {
  statuses,
  type Draft,
  type PortalAPI,
  type Submission,
} from '@/lib/portal';

export function CLICommand({ command }: { command: string }) {
  const [message, setMessage] = useState('');
  useEffect(() => {
    if (!message) return;
    const timer = setTimeout(() => setMessage(''), 2500);
    return () => clearTimeout(timer);
  }, [message]);
  return (
    <div className="dev-command">
      <code>{command}</code>
      <Button
        size="icon"
        variant="ghost"
        aria-label="Salin perintah"
        onClick={() => {
          if (!navigator.clipboard) {
            setMessage('Pilih dan salin teks perintah.');
            return;
          }
          void navigator.clipboard
            .writeText(command)
            .then(() => setMessage('Tersalin'))
            .catch(() => setMessage('Pilih dan salin teks perintah.'));
        }}
      >
        {message === 'Tersalin' ? <Check /> : <Copy />}
      </Button>
      <output className="dev-copy-status">{message}</output>
    </div>
  );
}

export default function DeveloperWorkspace({
  api,
  drafts,
  submissions,
  busy,
  search,
  setSearch,
  creating,
  setCreating,
  selected,
  select,
  edit,
  create,
  navigate,
  openReview,
}: {
  api: PortalAPI;
  drafts: Draft[];
  submissions: Submission[];
  busy: boolean;
  search: string;
  setSearch: (value: string) => void;
  creating: boolean;
  setCreating: (value: boolean) => void;
  selected: Draft | null;
  select: (value: Draft | null) => void;
  edit: (id: string) => void;
  create: (name: string) => void;
  navigate: (view: string) => void;
  refresh: () => void;
  openReview: (id: string) => void;
}) {
  const [name, setName] = useState('');
  if (creating)
    return (
      <section className="dev-create">
        <div className="dev-title">
          <Button
            variant="ghost"
            size="icon"
            aria-label="Kembali ke aplikasi"
            onClick={() => setCreating(false)}
            disabled={busy}
          >
            <ArrowLeft />
          </Button>
          <h1>Buat aplikasi</h1>
        </div>
        <div className="dev-create-grid">
          <section className="dev-card dev-cli-card">
            <span className="dev-feature-icon">
              <Terminal />
            </span>
            <h2>Mulai dengan Emisell CLI</h2>
            <p>Siapkan project React, lalu jalankan secara lokal.</p>
            <CLICommand command="npx @emisell/cli@latest app init" />
            <Button
              className="dev-text-link"
              variant="link"
              onClick={() => navigate('tooling')}
            >
              Baca dokumentasi <ArrowUpRight />
            </Button>
          </section>
          <form
            className="dev-card"
            onSubmit={(event) => {
              event.preventDefault();
              if (name.trim()) create(name.trim());
            }}
          >
            <span className="dev-feature-icon">
              <FileText />
            </span>
            <h2>Mulai dari dashboard</h2>
            <p>
              Buat aplikasi dan credential, lalu lengkapi konfigurasi versi.
            </p>
            <label htmlFor="dev-app-name">Nama aplikasi</label>
            <Input
              id="dev-app-name"
              required
              maxLength={100}
              placeholder="Contoh: Product Sync"
              value={name}
              onChange={(event) => setName(event.target.value)}
              disabled={busy}
            />
            <Button type="submit" disabled={busy || !name.trim()}>
              {busy ? 'Membuat aplikasi…' : 'Create app'}
            </Button>
          </form>
        </div>
        <p className="dev-footnote">
          <Info /> Credential tersedia setelah aplikasi dibuat. Akses data toko
          tetap memerlukan instalasi dan persetujuan izin oleh merchant.
        </p>
      </section>
    );
  if (selected)
    return (
      <DeveloperOverview
        key={selected.id}
        api={api}
        draft={selected}
        submissions={submissions}
        busy={busy}
        back={() => select(null)}
        edit={() => edit(selected.id)}
        navigate={navigate}
        openReview={openReview}
      />
    );
  return (
    <section className="dev-applications">
      <div className="page-heading">
        <div>
          <h1>Apps</h1>
        </div>
        <Button
          disabled={busy}
          onClick={() => {
            setName('');
            setCreating(true);
          }}
        >
          Create app
        </Button>
      </div>
      <div className="dev-search">
        <Search />
        <Input
          aria-label="Cari aplikasi"
          placeholder="Search by name or app ID"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
      </div>
      <DeveloperApps
        api={api}
        drafts={drafts.filter((draft) =>
          `${draft.document.name} ${draft.id}`
            .toLowerCase()
            .includes(search.toLowerCase()),
        )}
        busy={busy}
        compact
        openDraft={(id) =>
          select(drafts.find((draft) => draft.id === id) ?? null)
        }
      />
      {drafts.length > 0 &&
        !drafts.some((draft) =>
          `${draft.document.name} ${draft.id}`
            .toLowerCase()
            .includes(search.toLowerCase()),
        ) && (
          <p className="dev-empty-filter">
            Tidak ada aplikasi yang cocok dengan pencarian.
          </p>
        )}
      {!drafts.length && (
        <div className="dev-empty">
          <Box />
          <h2>Mulai dengan aplikasi pertama</h2>
          <p>Buat draft di dashboard atau mulai project dari CLI.</p>
          <Button
            variant="outline"
            onClick={() => setCreating(true)}
            disabled={busy}
          >
            <Plus />
            Buat aplikasi
          </Button>
        </div>
      )}
    </section>
  );
}

function DeveloperOverview({
  api,
  draft,
  submissions,
  busy,
  back,
  edit,
  navigate,
  openReview,
}: {
  api: PortalAPI;
  draft: Draft;
  submissions: Submission[];
  busy: boolean;
  back: () => void;
  edit: () => void;
  navigate: (view: string) => void;
  openReview: (id: string) => void;
}) {
  const [selectingMerchant, setSelectingMerchant] = useState(false);
  const [state, setState] = useState<{
    data: AppLifecycle | null;
    failed: boolean;
  }>({ data: null, failed: false });
  const [identity, setIdentity] = useState<
    'loading' | 'active' | 'unavailable'
  >('loading');
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let alive = true;
    if (!draft.activeVersion)
      loadAppLifecycle(api)
        .then((data) => {
          if (alive) setState({ data, failed: false });
        })
        .catch(() => {
          if (alive) setState({ data: null, failed: true });
        });
    loadApplicationIdentity(api, draft.id)
      .then(() => {
        if (alive) setIdentity('active');
      })
      .catch(() => {
        if (alive) setIdentity('unavailable');
      });
    return () => {
      alive = false;
    };
  }, [api, draft.id, draft.activeVersion, attempt]);
  const summary = developerOverview(draft, state.data, submissions);
  let versionStatus = {
    label: state.failed ? 'Belum terverifikasi' : 'Memuat…',
    tone: 'draft',
  };
  try {
    if (state.data) versionStatus = appLifecycleStatus(draft, state.data);
  } catch {
    versionStatus = { label: 'Belum terverifikasi', tone: 'draft' };
  }
  if (draft.activeVersion)
    versionStatus = { label: 'Active', tone: 'approved' };
  if (selectingMerchant)
    return (
      <DeveloperInstallPicker
        api={api}
        appId={draft.id}
        back={() => setSelectingMerchant(false)}
      />
    );
  return (
    <section className="dev-overview dev-overview-compact">
      <div className="dev-overview-heading">
        <h1>Overview</h1>
        <div className="dev-overview-heading-actions">
          <output
            className={
              identity === 'active'
                ? 'dev-identity-active'
                : 'dev-identity-pending'
            }
            title="Status identitas dan credential aplikasi, bukan status instalasi atau versi."
          >
            {identity === 'active'
              ? 'App active'
              : identity === 'loading'
                ? 'Memuat aplikasi…'
                : 'Status belum tersedia'}
          </output>
          <Button
            variant="ghost"
            size="icon"
            aria-label="Kembali ke Apps"
            onClick={back}
            disabled={busy}
          >
            <ArrowLeft />
          </Button>
        </div>
      </div>
      {(state.failed || identity === 'unavailable') && (
        <div className="portal-message error" role="alert">
          Sebagian status aplikasi belum dapat dimuat.
          <Button
            variant="outline"
            disabled={busy}
            onClick={() => {
              setState({ data: null, failed: false });
              setIdentity('loading');
              setAttempt((value) => value + 1);
            }}
          >
            Coba lagi
          </Button>
        </div>
      )}
      <div className="dev-overview-grid">
        <div className="dev-stack">
          <section className="dev-card dev-health-card">
            <h2>API health</h2>
            <button
              className="dev-health-summary"
              disabled={busy}
              onClick={() => navigate('monitoring')}
            >
              <span className="dev-health-symbol">
                <Info />
              </span>
              <span>
                <strong>No data</strong>
                <span>
                  Data kesehatan API belum tersedia untuk aplikasi ini.
                </span>
              </span>
              <ArrowRight />
            </button>
          </section>
          <section
            className="dev-card dev-overview-metrics"
            aria-label="Metrik 7 hari terakhir"
          >
            <div className="dev-metric-period">
              <CalendarDays /> <span>7 days</span>
            </div>
            {[
              'Webhook failure rate',
              'Removed subscriptions',
              'Function error rate',
            ].map((label) => (
              <div className="dev-metric" key={label}>
                <h2>{label}</h2>
                <span>No data</span>
              </div>
            ))}
          </section>
          <section className="dev-card dev-activity-card">
            <h2>Activity</h2>
            <ol className="dev-overview-timeline">
              <li className="dev-activity-date">
                <time dateTime={draft.updatedAt}>
                  {new Date(draft.updatedAt).toLocaleDateString('en-US', {
                    month: 'long',
                    day: 'numeric',
                    year: 'numeric',
                  })}
                </time>
              </li>
              <li>
                <button disabled={busy} onClick={edit}>
                  <code>{draft.document.version}</code> ·{' '}
                  {draft.activeVersion?.revision === draft.revision
                    ? 'Version activated'
                    : `Draft revisi ${draft.revision} disimpan`}
                </button>
                <time dateTime={draft.updatedAt}>
                  {' '}
                  ·{' '}
                  {new Date(draft.updatedAt).toLocaleTimeString('id-ID', {
                    hour: '2-digit',
                    minute: '2-digit',
                  })}
                </time>
              </li>
              {summary.reviews.slice(0, 3).map((row) => (
                <li key={row.id}>
                  <button disabled={busy} onClick={() => openReview(row.id)}>
                    <code>{row.version}</code> ·{' '}
                    {statuses[row.status] ?? row.status}
                  </button>
                  <time dateTime={row.createdAt}>
                    {' '}
                    · {new Date(row.createdAt).toLocaleString('id-ID')}
                  </time>
                </li>
              ))}
              <li>
                <button
                  className="dev-overview-link"
                  disabled={busy}
                  onClick={() => navigate('versions')}
                >
                  View all versions
                </button>
              </li>
            </ol>
          </section>
        </div>
        <aside className="dev-stack">
          <section className="dev-card">
            <div className="dev-overview-card-heading">
              <h2>Installs</h2>
              <Button
                disabled={busy}
                onClick={() => setSelectingMerchant(true)}
                title="Pilih toko Anda untuk melanjutkan instalasi"
              >
                Install app
              </Button>
            </div>
            <button
              className="dev-overview-row"
              disabled={busy}
              onClick={() => navigate('stores')}
              aria-label="Buka toko saya"
            >
              <span>No data</span>
              <ArrowRight />
            </button>
            <p className="dev-caption">
              Pilih toko Anda. Instalasi memerlukan persetujuan izin di toko;
              jumlah instalasi belum tersedia.
            </p>
          </section>
          <section className="dev-card">
            <div className="dev-overview-card-heading">
              <h2>Versions</h2>
              <Button
                variant="secondary"
                disabled={busy}
                onClick={() => navigate('versions')}
              >
                New version
              </Button>
            </div>
            <p className="dev-overview-description">
              Update your configuration by creating a new version.
            </p>
            <button
              className="dev-overview-row"
              disabled={busy}
              onClick={() => navigate('versions')}
            >
              <span className="dev-overview-version">
                <code>
                  {draft.activeVersion?.document.version ??
                    draft.document.version}
                </code>
                <span className={`status status-${versionStatus.tone}`}>
                  {versionStatus.label}
                </span>
              </span>
              <ArrowRight />
            </button>
          </section>
          <section className="dev-card">
            <div className="dev-overview-card-heading">
              <h2>Preview app with Emisell CLI</h2>
              <button
                className="dev-overview-link"
                onClick={() => navigate('tooling')}
              >
                Docs <ArrowUpRight />
              </button>
            </div>
            <CLICommand command="emisell app dev" />
          </section>
        </aside>
      </div>
    </section>
  );
}
