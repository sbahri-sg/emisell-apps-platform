'use client';

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
  submissions,
  navigate,
  openReview,
  refresh,
  busy,
}: {
  submissions: Submission[];
  navigate: (view: string) => void;
  openReview: (id: string) => void;
  refresh: () => void;
  busy: boolean;
}) {
  const latest = [...submissions]
    .sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt))
    .slice(0, 3);
  const pending = submissions.filter((s) => s.status === 'submitted').length;
  const metrics = [
    {
      title: 'Aplikasi aktif',
      value: '—',
      note: 'Statistik belum tersedia',
      icon: Grid2X2,
    },
    {
      title: 'Developer',
      value: '—',
      note: 'Statistik belum tersedia',
      icon: Users,
    },
    {
      title: 'Instalasi aktif',
      value: '—',
      note: 'Statistik belum tersedia',
      icon: Download,
    },
    {
      title: 'Menunggu review',
      value: String(pending),
      note: 'Dalam daftar pengajuan termuat',
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
          <Button variant="outline" disabled={busy} onClick={refresh}>
            <RefreshCw size={16} />
            Perbarui data
          </Button>
          <Button disabled={busy} onClick={() => navigate('reviews')}>
            Lihat review
          </Button>
        </div>
      </div>
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
          <h2>Instalasi aplikasi</h2>
          <div className="chart-unavailable">
            <Activity size={30} />
            <strong>Riwayat instalasi belum tersedia</strong>
            <p>
              Grafik akan ditampilkan setelah data statistik platform tersedia.
            </p>
          </div>
        </section>
        <section className="overview-card overview-health">
          <h2>Kesehatan platform</h2>
          {[
            { name: 'API gateway', icon: Network },
            { name: 'Webhook', icon: Activity },
            { name: 'Sesi aplikasi', icon: ShieldCheck },
          ].map((s) => (
            <div className="health-row" key={s.name}>
              <span className="health-icon">
                <s.icon size={23} />
              </span>
              <div>
                <strong>{s.name}</strong>
                <small>Monitoring belum terhubung</small>
              </div>
              <span className="health-unavailable">Belum diukur</span>
            </div>
          ))}
          <div className="health-foot">
            Tidak menyimpulkan status dari koneksi portal.
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
