'use client';

import { useEffect, useState } from 'react';
import type { Overview } from '@/lib/overview';
import type { PortalAPI } from '@/lib/portal';

import {
  Activity,
  ArrowUpRight,
  Clock3,
  FileCode2,
  Grid2X2,
  Network,
  RefreshCw,
  ShieldCheck,
  Users,
  Download,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '@/components/ui/table';
import { statuses, type Submission } from '@/lib/portal';

export default function AdminOverview({
  api,
  submissions,
  navigate,
  openReview,
  refresh,
  busy,
}: {
  api: PortalAPI;
  submissions: Submission[];
  navigate: (view: string) => void;
  openReview: (id: string) => void;
  refresh: () => void;
  busy: boolean;
}) {
  const [summary, setSummary] = useState<Overview | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [revision, setRevision] = useState(0);
  const chartMax = Math.max(2, ...(summary?.history ?? []).map((p) => p.count));
  const chartTotal = (summary?.history ?? []).reduce((n, p) => n + p.count, 0);
  const chartDate = (date: string) =>
    new Date(`${date}T00:00:00Z`).toLocaleDateString('id-ID', {
      day: 'numeric',
      month: 'short',
      timeZone: 'UTC',
    });
  useEffect(() => {
    let alive = true;
    setLoading(true);
    setError('');
    api
      .request<Overview>('/overview')
      .then((value) => {
        if (alive) setSummary(value);
      })
      .catch(() => {
        if (alive)
          setError(
            'Ringkasan gagal diperbarui. Coba lagi; data sebelumnya tetap ditampilkan.',
          );
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [api, revision]);
  const latest = [...submissions]
    .sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt))
    .slice(0, 3);
  const metrics = [
    {
      title: 'Aplikasi dipublikasikan',
      value: summary?.publishedApps ?? '—',
      note: 'Aplikasi unik berstatus published',
      icon: Grid2X2,
    },
    {
      title: 'Developer',
      value: summary?.developers ?? '—',
      note: 'Akun developer yang diaktifkan',
      icon: Users,
    },
    {
      title: 'Instalasi aktif',
      value: summary?.activeInstallations ?? '—',
      note: 'Instalasi berstatus aktif',
      icon: Download,
    },
    {
      title: 'Menunggu review',
      value: summary?.pendingReviews ?? '—',
      note: 'Seluruh pengajuan menunggu review',
      icon: Clock3,
    },
  ];
  return (
    <section className="admin-overview">
      <div className="overview-heading">
        <div>
          <h1>Ringkasan platform</h1>
          <p>Kelola aplikasi, developer, dan kesehatan integrasi.</p>
        </div>
        <div className="overview-actions">
          <Button
            variant="outline"
            disabled={busy || loading}
            onClick={() => {
              setRevision((v) => v + 1);
              refresh();
            }}
          >
            <RefreshCw size={16} />
            Perbarui data
          </Button>
          <Button disabled={busy} onClick={() => navigate('reviews')}>
            Lihat review
          </Button>
        </div>
      </div>
      {error && <p role="alert">{error}</p>}
      <p aria-live="polite">
        {loading
          ? 'Memperbarui ringkasan…'
          : summary
            ? `Diperiksa ${new Date(summary.checkedAt).toLocaleString('id-ID')}`
            : 'Data belum tersedia'}
      </p>
      <div className="overview-metrics">
        {metrics.map((m) => (
          <article className="overview-card metric" key={m.title}>
            <span className="metric-icon">
              <m.icon size={23} />
            </span>
            <div>
              <h2>{m.title}</h2>
              <strong>{m.value}</strong>
              <p>{m.note}</p>
            </div>
          </article>
        ))}
      </div>
      <div className="overview-columns">
        <section className="overview-card overview-chart">
          <div className="installation-chart-heading">
            <div>
              <h2>Instalasi aplikasi</h2>
              <p>30 hari terakhir · UTC</p>
            </div>
            {summary && (
              <div className="installation-chart-total">
                <strong>{chartTotal}</strong>
                <span>total instalasi</span>
              </div>
            )}
          </div>
          {summary ? (
            <>
              <div className="installation-chart-body">
                <div
                  className="installation-chart-plot"
                  role="img"
                  aria-label={`${chartTotal} instalasi dalam 30 hari. Rincian tersedia di tabel angka harian.`}
                >
                  <div className="installation-chart-scale" aria-hidden="true">
                    <span>{chartMax}</span>
                    <span>{chartMax / 2}</span>
                    <span>0</span>
                  </div>
                  <div className="installation-chart-bars">
                    {summary.history.map((p) => (
                      <div
                        key={p.date}
                        title={`${p.date}: ${p.count} instalasi`}
                        className="installation-chart-bar"
                        style={{
                          height: `${(p.count / chartMax) * 100}%`,
                        }}
                      />
                    ))}
                  </div>
                </div>
                <div className="installation-chart-dates" aria-hidden="true">
                  {summary.history
                    .filter((_, i) => i === 0 || i === 14 || i === 29)
                    .map((p) => (
                      <span key={p.date}>{chartDate(p.date)}</span>
                    ))}
                </div>
                <p className="installation-chart-note">
                  Termasuk instalasi ulang.
                  {chartTotal === 0
                    ? ' Belum ada instalasi dalam periode ini.'
                    : ''}
                </p>
              </div>
              <details className="installation-chart-details">
                <summary>Lihat angka harian</summary>
                <div className="installation-chart-table">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Tanggal (UTC)</TableHead>
                        <TableHead>Instalasi</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {summary.history.map((p) => (
                        <TableRow key={p.date}>
                          <TableCell>{p.date}</TableCell>
                          <TableCell>{p.count}</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              </details>
            </>
          ) : (
            <p className="installation-chart-body">
              Riwayat belum berhasil dimuat.
            </p>
          )}
        </section>
        <section className="overview-card overview-health">
          <h2>Kesehatan platform</h2>
          {[
            {
              name: 'API & database',
              icon: Network,
              detail: 'Pemeriksaan saat ringkasan dimuat',
              value: summary ? 'Terhubung' : 'Belum diukur',
            },
            {
              name: 'Antrean webhook',
              icon: Activity,
              detail: 'Jumlah antrean saat ini; bukan uptime worker',
              value: summary
                ? `${summary.webhookPending} menunggu · ${summary.webhookDead} gagal`
                : 'Belum diukur',
            },
            {
              name: 'Sesi portal',
              icon: ShieldCheck,
              detail: 'Sesi belum kedaluwarsa dari akun aktif',
              value: summary
                ? `${summary.portalSessions} sesi`
                : 'Belum diukur',
            },
          ].map((s) => (
            <div className="health-row" key={s.name}>
              <span className="health-icon">
                <s.icon size={23} />
              </span>
              <div>
                <strong>{s.name}</strong>
                <small>{s.detail}</small>
              </div>
              <span className="health-unavailable">
                {error ? 'Gagal diperbarui' : s.value}
              </span>
            </div>
          ))}
          <div className="health-foot">
            Snapshot database, bukan pemantauan uptime atau sesi embedded.
          </div>
        </section>
        <section className="overview-card overview-reviews">
          <h2>Review terbaru</h2>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Aplikasi</TableHead>
                <TableHead>Versi</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Diajukan</TableHead>
                <TableHead>
                  <span className="sr-only">Aksi</span>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {latest.map((s) => (
                <TableRow key={s.id}>
                  <TableCell>
                    <span className="review-app">
                      <span className="review-icon">
                        <FileCode2 size={19} />
                      </span>
                      {s.snapshot.name}
                    </span>
                  </TableCell>
                  <TableCell>{s.version}</TableCell>
                  <TableCell>
                    <span className={`status status-${s.status}`}>
                      {statuses[s.status] ?? s.status}
                    </span>
                  </TableCell>
                  <TableCell>
                    {new Date(s.createdAt).toLocaleDateString('id-ID', {
                      day: 'numeric',
                      month: 'short',
                    })}
                  </TableCell>
                  <TableCell>
                    <Button
                      variant="ghost"
                      size="icon"
                      disabled={busy}
                      onClick={() => openReview(s.id)}
                      aria-label={`Lihat review ${s.snapshot.name}`}
                    >
                      <ArrowUpRight size={16} />
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
              {!latest.length && (
                <TableRow>
                  <TableCell colSpan={5} className="overview-empty">
                    Belum ada pengajuan review.
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
          <Button variant="link" onClick={() => navigate('reviews')}>
            Lihat semua review
          </Button>
        </section>
        <section className="overview-card overview-activity">
          <h2>Aktivitas pengajuan</h2>
          {latest.map((s) => (
            <div className="overview-event" key={s.id}>
              <span className="event-icon">
                <FileCode2 size={17} />
              </span>
              <div>
                <strong>Pengajuan review dikirim</strong>
                <p>
                  {s.snapshot.name} · {s.version}
                </p>
                <time>
                  {new Date(s.createdAt).toLocaleString('id-ID', {
                    dateStyle: 'medium',
                    timeStyle: 'short',
                  })}
                </time>
              </div>
            </div>
          ))}
          {!latest.length && (
            <p className="overview-empty">Belum ada aktivitas pengajuan.</p>
          )}
          <p className="activity-note">
            Berdasarkan pengajuan termuat, bukan seluruh audit platform.
          </p>
        </section>
      </div>
    </section>
  );
}
