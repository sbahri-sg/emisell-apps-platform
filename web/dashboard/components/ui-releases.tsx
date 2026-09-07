'use client';
import { useCallback, useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '@/components/ui/table';
import {
  Files,
  ShieldCheck,
  Clock3,
  Ban,
  RefreshCw,
  Monitor,
  ChevronLeft,
  ChevronRight,
} from 'lucide-react';
import {
  NativeSelect,
  NativeSelectOption,
} from '@/components/ui/native-select';
import type { PortalAPI, Session } from '@/lib/portal';
import ResourceReleases from './resource-releases';

type Release = {
  id: string;
  revision: number;
  status: string;
  manifest: {
    appId: string;
    name: string;
    summary: string;
    version: string;
    url: string;
    mode: string;
  };
  package?: { sha256: string };
};
const labels: Record<string, string> = {
  submitted: 'Diajukan',
  approved: 'Disetujui',
  signed: 'Ditandatangani',
  rejected: 'Ditolak',
  suspended: 'Ditangguhkan',
};
const blank = {
  appId: '',
  name: '',
  summary: '',
  version: '1.0.0',
  url: '',
  mode: 'embedded',
  reason: '',
};
export default function UIReleases({
  api,
  session,
}: {
  api: PortalAPI;
  session: Session;
}) {
  const [rows, setRows] = useState<Release[]>([]),
    [selected, setSelected] = useState<Release | null>(null);
  const [form, setForm] = useState(blank),
    [reason, setReason] = useState(''),
    [next, setNext] = useState('');
  const [busy, setBusy] = useState(true),
    [error, setError] = useState(''),
    [loaded, setLoaded] = useState(false);
  const developer = session.user.surface === 'developer';
  const [query, setQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState('all');
  const [modeFilter, setModeFilter] = useState('all');
  const [page, setPage] = useState(0);
  const filtered = rows.filter(
    (r) =>
      (statusFilter === 'all' || r.status === statusFilter) &&
      (modeFilter === 'all' || r.manifest.mode === modeFilter) &&
      `${r.manifest.name} ${r.manifest.version}`
        .toLowerCase()
        .includes(query.toLowerCase()),
  );
  const pages = Math.max(1, Math.ceil(filtered.length / 6));
  const currentPage = Math.min(page, pages - 1);
  const load = useCallback(
    async (after = '') => {
      const data = await api.request<{
        releases: Release[];
        nextAfterId: string;
      }>('/ui-releases', 'GET', undefined, false, { afterId: after });
      setRows((old) => (after ? [...old, ...data.releases] : data.releases));
      setNext(data.nextAfterId);
      setLoaded(true);
    },
    [api],
  );
  const perform = async (fn: () => Promise<void>) => {
    setBusy(true);
    setError('');
    try {
      await fn();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Permintaan gagal.');
    } finally {
      setBusy(false);
    }
  };
  useEffect(() => {
    let active = true;
    Promise.resolve()
      .then(() => load())
      .catch((e) => {
        if (active) setError(e.message);
      })
      .finally(() => {
        if (active) setBusy(false);
      });
    return () => {
      active = false;
    };
  }, [load]);
  const open = async (id: string) => {
    const d = await api.request<{ release: Release }>(`/ui-releases/${id}`);
    setSelected(d.release);
    setReason('');
  };
  const actions =
    selected?.status === 'submitted'
      ? ['approved', 'rejected']
      : selected?.status === 'approved'
        ? ['signed', 'suspended']
        : selected?.status === 'signed'
          ? ['suspended']
          : [];
  return (
    <>
      <ResourceReleases api={api} session={session} />
      <div className="page-heading">
        <div>
          {developer && <p className="eyebrow">APLIKASI DEVELOPER</p>}
          <h1>{developer ? 'Aplikasi dengan UI' : 'Rilis UI'}</h1>
          <p>
            Tinjau versi antarmuka, status rilis, dan tujuan pembukaan aplikasi.
          </p>
        </div>
        <Button
          variant="outline"
          disabled={busy}
          onClick={() => void perform(() => load())}
        >
          <RefreshCw size={16} /> Muat ulang
        </Button>
      </div>
      <div className="portal-callout">
        Review dan signing belum memberikan izin install. Release signed dapat
        dipilih di App clients dan Testing; akses seller belum tersedia untuk
        release UI ini. Aplikasi tanpa dashboard tetap dikelola di Settings.
      </div>
      {error && (
        <p role="alert" className="portal-callout">
          {error}
        </p>
      )}
      {busy && <output>Memproses…</output>}
      {selected ? (
        <section className="catalog-card">
          <Button
            variant="outline"
            disabled={busy}
            onClick={() => setSelected(null)}
          >
            Kembali ke daftar
          </Button>
          <h2>
            {selected.manifest.name} · {selected.manifest.version}
          </h2>
          <p>{labels[selected.status]}</p>
          <p>{selected.manifest.summary}</p>
          <p>Mode: {selected.manifest.mode}</p>
          <p className="break-all">URL: {selected.manifest.url}</p>
          <p className="break-all">App ID: {selected.manifest.appId}</p>
          {selected.package && (
            <p className="break-all">Checksum: {selected.package.sha256}</p>
          )}
          {developer && (
            <Button
              disabled={busy}
              variant="outline"
              onClick={() => {
                setForm({
                  ...blank,
                  appId: selected.manifest.appId,
                  name: selected.manifest.name,
                  summary: selected.manifest.summary,
                  url: selected.manifest.url,
                  mode: selected.manifest.mode,
                  version: '',
                  reason: '',
                });
                setSelected(null);
              }}
            >
              Siapkan versi berikutnya
            </Button>
          )}
          {!developer &&
            ['administrator', 'reviewer'].includes(session.user.role) &&
            actions.length > 0 && (
              <div>
                <label htmlFor="ui-decision-reason">
                  Alasan keputusan
                  <Input
                    id="ui-decision-reason"
                    value={reason}
                    maxLength={2000}
                    onChange={(e) => setReason(e.target.value)}
                  />
                </label>
                {actions
                  .filter(
                    (action) =>
                      !['signed', 'suspended'].includes(action) ||
                      session.user.role === 'administrator',
                  )
                  .map((action) => (
                    <Button
                      key={action}
                      disabled={busy || !reason.trim()}
                      onClick={() =>
                        void perform(async () => {
                          const d = await api.request<{ release: Release }>(
                            `/ui-releases/${selected.id}/status`,
                            'POST',
                            {
                              revision: selected.revision,
                              status: action,
                              reason,
                            },
                            true,
                          );
                          setSelected(d.release);
                          setReason('');
                          await load();
                        })
                      }
                    >
                      {
                        (
                          {
                            approved: 'Setujui',
                            rejected: 'Tolak',
                            signed: 'Tandatangani',
                            suspended: 'Tangguhkan',
                          } as Record<string, string>
                        )[action]
                      }
                    </Button>
                  ))}
              </div>
            )}
        </section>
      ) : (
        <>
          {!developer ? (
            <section className="admin-ui-release-list">
              <div className="ui-release-metrics">
                {[
                  { title: 'Total rilis', status: 'all', icon: Files },
                  {
                    title: 'Ditandatangani',
                    status: 'signed',
                    icon: ShieldCheck,
                  },
                  {
                    title: 'Menunggu review',
                    status: 'submitted',
                    icon: Clock3,
                  },
                  { title: 'Ditangguhkan', status: 'suspended', icon: Ban },
                ].map((card) => (
                  <article key={card.status}>
                    <span className={`release-icon ${card.status}`}>
                      <card.icon />
                    </span>
                    <div>
                      <p>{card.title}</p>
                      <strong>
                        {loaded
                          ? rows.filter(
                              (r) =>
                                card.status === 'all' ||
                                r.status === card.status,
                            ).length
                          : '—'}
                      </strong>
                      <small>Dalam daftar termuat</small>
                    </div>
                  </article>
                ))}
              </div>
              <div className="ui-release-table">
                <Tabs
                  value={statusFilter}
                  onValueChange={(v) => {
                    setStatusFilter(v);
                    setPage(0);
                  }}
                >
                  <TabsList variant="line">
                    <TabsTrigger value="all">Semua rilis</TabsTrigger>
                    {Object.entries(labels).map(([key, label]) => (
                      <TabsTrigger key={key} value={key}>
                        {label}
                      </TabsTrigger>
                    ))}
                  </TabsList>
                </Tabs>
                <div className="ui-release-filters">
                  <Input
                    aria-label="Cari aplikasi atau versi rilis UI"
                    placeholder="Cari aplikasi atau versi…"
                    value={query}
                    onChange={(e) => {
                      setQuery(e.target.value);
                      setPage(0);
                    }}
                  />
                  <NativeSelect
                    aria-label="Mode rilis UI"
                    value={modeFilter}
                    onChange={(e) => {
                      setModeFilter(e.target.value);
                      setPage(0);
                    }}
                  >
                    <NativeSelectOption value="all">
                      Semua mode
                    </NativeSelectOption>
                    <NativeSelectOption value="embedded">
                      Embedded
                    </NativeSelectOption>
                    <NativeSelectOption value="external">
                      External
                    </NativeSelectOption>
                  </NativeSelect>
                </div>
                <Table aria-label="Daftar rilis UI">
                  <TableHeader>
                    <TableRow>
                      <TableHead>Aplikasi</TableHead>
                      <TableHead>Versi</TableHead>
                      <TableHead>Mode</TableHead>
                      <TableHead>Status rilis</TableHead>
                      <TableHead>Revisi</TableHead>
                      <TableHead>Aksi</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {filtered
                      .slice(currentPage * 6, currentPage * 6 + 6)
                      .map((row) => (
                        <TableRow key={row.id}>
                          <TableCell>
                            <div className="ui-release-name">
                              <span className="release-icon">
                                <Monitor size={20} />
                              </span>
                              <div>
                                <strong>{row.manifest.name}</strong>
                                <small>{row.manifest.summary}</small>
                              </div>
                            </div>
                          </TableCell>
                          <TableCell>{row.manifest.version}</TableCell>
                          <TableCell>
                            {row.manifest.mode === 'embedded'
                              ? 'Embedded'
                              : 'External'}
                          </TableCell>
                          <TableCell>
                            <span className={`release-status ${row.status}`}>
                              {labels[row.status] ?? row.status}
                            </span>
                          </TableCell>
                          <TableCell>r{row.revision}</TableCell>
                          <TableCell>
                            <Button
                              disabled={busy}
                              variant="outline"
                              onClick={() => void perform(() => open(row.id))}
                            >
                              Lihat rilis
                            </Button>
                          </TableCell>
                        </TableRow>
                      ))}
                    {loaded && !filtered.length && (
                      <TableRow>
                        <TableCell colSpan={6}>
                          {rows.length
                            ? 'Tidak ada rilis yang cocok dengan filter.'
                            : 'Belum ada release UI yang diajukan.'}
                        </TableCell>
                      </TableRow>
                    )}
                  </TableBody>
                </Table>
                <div className="scope-pagination">
                  <span>
                    Menampilkan {filtered.length ? currentPage * 6 + 1 : 0}–
                    {Math.min((currentPage + 1) * 6, filtered.length)} dari{' '}
                    {filtered.length} rilis termuat
                  </span>
                  <div>
                    <Button
                      aria-label="Halaman rilis sebelumnya"
                      variant="outline"
                      disabled={currentPage === 0}
                      onClick={() => setPage(currentPage - 1)}
                    >
                      <ChevronLeft />
                    </Button>
                    <span>
                      {currentPage + 1} / {pages}
                    </span>
                    <Button
                      aria-label="Halaman rilis berikutnya"
                      variant="outline"
                      disabled={currentPage + 1 >= pages}
                      onClick={() => setPage(currentPage + 1)}
                    >
                      <ChevronRight />
                    </Button>
                  </div>
                </div>
                {next && (
                  <Button
                    disabled={busy}
                    variant="outline"
                    onClick={() => void perform(() => load(next))}
                  >
                    Muat berikutnya
                  </Button>
                )}
              </div>
              <div className="scope-explainer">
                <ShieldCheck />
                <div>
                  <h2>Pemeriksaan rilis</h2>
                  <p>
                    URL aplikasi, signature, dan digest diperiksa melalui proses
                    rilis yang berlaku. Status ditandatangani bukan izin
                    instalasi; App clients, assignment Testing, dan persetujuan
                    toko tetap terpisah.
                  </p>
                </div>
              </div>
            </section>
          ) : (
            <section className="catalog-card">
              <h2>Release</h2>
              {loaded && !rows.length && (
                <p>Belum ada release UI yang diajukan.</p>
              )}
              {rows.map((row) => (
                <div
                  key={row.id}
                  className="flex flex-wrap items-center justify-between gap-3 border-b py-4"
                >
                  <div>
                    <strong>{row.manifest.name}</strong>
                    <p>
                      {row.manifest.version} · {row.manifest.mode} ·{' '}
                      {labels[row.status]}
                    </p>
                  </div>
                  <Button
                    disabled={busy}
                    variant="outline"
                    onClick={() => void perform(() => open(row.id))}
                  >
                    Detail
                  </Button>
                </div>
              ))}
              {next && (
                <Button
                  disabled={busy}
                  onClick={() => void perform(() => load(next))}
                >
                  Muat berikutnya
                </Button>
              )}
            </section>
          )}
          {developer && (
            <form
              className="catalog-card space-y-4"
              onSubmit={(e) => {
                e.preventDefault();
                void perform(async () => {
                  const d = await api.request<{ release: Release }>(
                    '/ui-releases',
                    'POST',
                    form,
                    true,
                  );
                  setForm(blank);
                  setSelected(d.release);
                  await load();
                });
              }}
            >
              <h2>
                {form.appId ? 'Ajukan versi baru' : 'Ajukan aplikasi UI baru'}
              </h2>
              {form.appId && <p className="break-all">App ID: {form.appId}</p>}
              {(['name', 'summary', 'version', 'url', 'reason'] as const).map(
                (field) => (
                  <label key={field} className="block">
                    {
                      {
                        name: 'Nama aplikasi',
                        summary: 'Ringkasan',
                        version: 'Versi (contoh 1.0.0)',
                        url: 'URL aplikasi HTTPS',
                        reason: 'Alasan pengajuan',
                      }[field]
                    }
                    <Input
                      required
                      disabled={busy}
                      value={form[field]}
                      maxLength={
                        field === 'name'
                          ? 100
                          : field === 'summary'
                            ? 180
                            : field === 'reason'
                              ? 2000
                              : 2048
                      }
                      type={field === 'url' ? 'url' : 'text'}
                      onChange={(e) =>
                        setForm({ ...form, [field]: e.target.value })
                      }
                    />
                  </label>
                ),
              )}
              <label className="block" htmlFor="ui-mode">
                Cara membuka aplikasi
                <NativeSelect
                  id="ui-mode"
                  disabled={busy}
                  value={form.mode}
                  onChange={(e) => setForm({ ...form, mode: e.target.value })}
                >
                  <NativeSelectOption value="embedded">
                    Di dalam Dashboard (embedded)
                  </NativeSelectOption>
                  <NativeSelectOption value="external">
                    Dashboard eksternal
                  </NativeSelectOption>
                </NativeSelect>
              </label>
              <p>
                Gratis, tanpa izin data bisnis. Pengajuan menjadi snapshot
                tetap; perubahan selanjutnya memakai versi baru.
              </p>
              <Button disabled={busy} type="submit">
                Ajukan review
              </Button>
            </form>
          )}
        </>
      )}
    </>
  );
}
