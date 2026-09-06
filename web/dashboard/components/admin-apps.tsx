'use client';
import { useState } from 'react';
import { Boxes, Globe, Clock3, FileText, RefreshCw } from 'lucide-react';
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
import { statuses, type Submission } from '@/lib/portal';
import type { CatalogRelease } from '@/components/catalog-panel';

export default function AdminApps({
  submissions,
  releases,
  busy,
  refresh,
  openReview,
  navigate,
}: {
  submissions: Submission[];
  releases: CatalogRelease[];
  busy: boolean;
  refresh: () => void;
  openReview: (id: string) => void;
  navigate: (view: string) => void;
}) {
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState('all');
  const [page, setPage] = useState(0);
  const [selected, setSelected] = useState<string | null>(null);
  const ids = [
    ...new Set([
      ...submissions.map((s) => s.appId),
      ...releases.map((r) => r.package.manifest.appId),
    ]),
  ];
  const apps = ids.map((id) => {
    const submission = submissions
      .filter((s) => s.appId === id)
      .sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt))[0];
    const versions = releases
      .filter((r) => r.package.manifest.appId === id)
      .sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt));
    const release = versions[0];
    const published = versions.some((r) => r.status === 'published');
    return {
      id,
      submission,
      release,
      published,
      name: submission?.snapshot.name ?? release.package.manifest.name,
      version: submission?.version ?? release.package.manifest.version,
    };
  });
  const filtered = apps.filter(
    (a) =>
      `${a.name} ${a.id}`.toLowerCase().includes(query.toLowerCase()) &&
      (filter === 'all' ||
        (filter === 'published' && a.published) ||
        (filter === 'review' && a.submission?.status === 'submitted') ||
        (filter === 'suspended' && a.release?.status === 'suspended')),
  );
  const pages = Math.max(1, Math.ceil(filtered.length / 6));
  const current = Math.min(page, pages - 1);
  const detail = apps.find((a) => a.id === selected);
  const publication = (a: (typeof apps)[number]) =>
    a.published
      ? 'Dipublikasikan'
      : a.release?.status === 'suspended'
        ? 'Ditangguhkan'
        : a.release
          ? 'Ditandatangani'
          : 'Belum dipublikasikan';
  return (
    <section className="admin-ui-release-list">
      <div className="overview-heading">
        <div>
          <h1>Aplikasi</h1>
          <p>
            Ringkasan aplikasi dari pengajuan dan rilis katalog yang termuat.
          </p>
        </div>
        <Button variant="outline" disabled={busy} onClick={refresh}>
          <RefreshCw /> Muat ulang
        </Button>
      </div>
      <div className="ui-release-metrics">
        {[
          { label: 'Aplikasi termuat', value: apps.length, icon: Boxes },
          {
            label: 'Dipublikasikan',
            value: apps.filter((a) => a.published).length,
            icon: Globe,
          },
          {
            label: 'Dalam review',
            value: apps.filter((a) => a.submission?.status === 'submitted')
              .length,
            icon: Clock3,
          },
          { label: 'Draft developer', value: '—', icon: FileText },
        ].map((c) => (
          <article key={c.label}>
            <span className="release-icon">
              <c.icon />
            </span>
            <div>
              <p>{c.label}</p>
              <strong>{c.value}</strong>
              <small>
                {c.value === '—'
                  ? 'Data belum tersedia'
                  : 'Dalam daftar termuat'}
              </small>
            </div>
          </article>
        ))}
      </div>
      {detail ? (
        <section className="ui-release-table">
          <Button variant="outline" onClick={() => setSelected(null)}>
            Kembali ke aplikasi
          </Button>
          <h2>{detail.name}</h2>
          <p>
            {detail.submission?.snapshot.summary ??
              detail.release?.package.manifest.summary}
          </p>
          <p className="break-all">App ID: {detail.id}</p>
          <p>Versi pengajuan terbaru: {detail.version}</p>
          <p>Publikasi: {publication(detail)}</p>
          <p>
            Review terbaru:{' '}
            {detail.submission
              ? (statuses[detail.submission.status] ?? detail.submission.status)
              : 'Tidak tersedia dalam daftar termuat'}
          </p>
          <div className="ui-release-filters">
            {detail.submission && (
              <Button
                disabled={busy}
                onClick={() => openReview(detail.submission.id)}
              >
                Lihat pengajuan
              </Button>
            )}
            {detail.release && (
              <Button variant="outline" onClick={() => navigate('catalog')}>
                Kelola rilis & publikasi
              </Button>
            )}
          </div>
        </section>
      ) : (
        <div className="ui-release-table">
          <Tabs
            value={filter}
            onValueChange={(v) => {
              setFilter(v);
              setPage(0);
            }}
          >
            <TabsList variant="line">
              <TabsTrigger value="all">Semua aplikasi</TabsTrigger>
              <TabsTrigger value="published">Dipublikasikan</TabsTrigger>
              <TabsTrigger value="review">Dalam review</TabsTrigger>
              <TabsTrigger value="suspended">Ditangguhkan</TabsTrigger>
            </TabsList>
          </Tabs>
          <div className="ui-release-filters">
            <Input
              aria-label="Cari nama aplikasi"
              placeholder="Cari nama aplikasi…"
              value={query}
              onChange={(e) => {
                setQuery(e.target.value);
                setPage(0);
              }}
            />
          </div>
          <Table aria-label="Daftar aplikasi Admin">
            <TableHeader>
              <TableRow>
                <TableHead>Aplikasi</TableHead>
                <TableHead>Publikasi</TableHead>
                <TableHead>Review terbaru</TableHead>
                <TableHead>Aksi</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.slice(current * 6, current * 6 + 6).map((a) => (
                <TableRow key={a.id}>
                  <TableCell>
                    <div className="ui-release-name">
                      <span className="release-icon">
                        <Boxes />
                      </span>
                      <div>
                        <strong>{a.name}</strong>
                        <small>{a.version}</small>
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>{publication(a)}</TableCell>
                  <TableCell>
                    {a.submission
                      ? (statuses[a.submission.status] ?? a.submission.status)
                      : '—'}
                  </TableCell>
                  <TableCell>
                    <Button variant="outline" onClick={() => setSelected(a.id)}>
                      Lihat aplikasi
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
              {!filtered.length && (
                <TableRow>
                  <TableCell colSpan={4}>
                    Tidak ada aplikasi yang cocok dengan daftar atau filter ini.
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
          <div className="scope-pagination">
            <span>
              {filtered.length} aplikasi termuat · halaman {current + 1} /{' '}
              {pages}
            </span>
            <div>
              <Button
                variant="outline"
                disabled={!current}
                onClick={() => setPage(current - 1)}
              >
                Sebelumnya
              </Button>
              <Button
                variant="outline"
                disabled={current + 1 >= pages}
                onClick={() => setPage(current + 1)}
              >
                Berikutnya
              </Button>
            </div>
          </div>
        </div>
      )}
      <div className="scope-explainer">
        <Boxes />
        <div>
          <h2>Status publikasi bukan status instalasi toko</h2>
          <p>
            Daftar ini terbatas pada pengajuan dan katalog yang tersedia untuk
            Admin. Nama developer, mode UI, draft yang belum diajukan, dan
            jumlah instalasi belum disediakan oleh sumber ini. Rilis antarmuka
            dikelola terpisah melalui Rilis UI.
          </p>
          <Button variant="outline" onClick={() => navigate('ui-releases')}>
            Buka Rilis UI
          </Button>
        </div>
      </div>
    </section>
  );
}
