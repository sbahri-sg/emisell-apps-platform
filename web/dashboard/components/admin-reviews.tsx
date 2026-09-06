'use client';
import { useState } from 'react';
import {
  Clock3,
  CheckCircle2,
  AlertCircle,
  XCircle,
  FileCode2,
  RefreshCw,
  Search,
  ChevronLeft,
  ChevronRight,
  Info,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  NativeSelect,
  NativeSelectOption,
} from '@/components/ui/native-select';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import {
  Table,
  TableHeader,
  TableBody,
  TableHead,
  TableRow,
  TableCell,
} from '@/components/ui/table';
import { statuses, type Submission } from '@/lib/portal';

export default function AdminReviews({
  submissions,
  busy,
  search,
  setSearch,
  open,
  refresh,
}: {
  submissions: Submission[];
  busy: boolean;
  search: string;
  setSearch: (s: string) => void;
  open: (id: string) => void;
  refresh: () => void;
}) {
  const [filter, setFilter] = useState('all');
  const [sort, setSort] = useState('newest');
  const [page, setPage] = useState(0);
  const filtered = submissions
    .filter(
      (s) =>
        (filter === 'all' || s.status === filter) &&
        `${s.snapshot.name} ${s.version}`
          .toLowerCase()
          .includes(search.toLowerCase()),
    )
    .sort(
      (a, b) =>
        (Date.parse(b.createdAt) - Date.parse(a.createdAt)) *
        (sort === 'newest' ? 1 : -1),
    );
  const totalPages = Math.max(1, Math.ceil(filtered.length / 6));
  const current = Math.min(page, totalPages - 1);
  const rows = filtered.slice(current * 6, current * 6 + 6);
  const cards = [
    { state: 'submitted', label: 'Menunggu review', icon: Clock3 },
    { state: 'changes_requested', label: 'Perlu perbaikan', icon: AlertCircle },
    { state: 'approved', label: 'Disetujui', icon: CheckCircle2 },
    { state: 'rejected', label: 'Ditolak', icon: XCircle },
  ];
  const tabs = [
    ['all', 'Semua pengajuan'],
    ['submitted', 'Antrean'],
    ['changes_requested', 'Perlu perbaikan'],
    ['approved', 'Disetujui'],
    ['rejected', 'Ditolak'],
  ];
  return (
    <section className="admin-review-page">
      <div className="overview-heading">
        <div>
          <h1>Pengajuan review</h1>
          <p>Periksa kelayakan aplikasi sebelum proses rilis dan publikasi.</p>
        </div>
        <Button variant="outline" disabled={busy} onClick={refresh}>
          <RefreshCw size={16} />
          Perbarui data
        </Button>
      </div>
      <div className="overview-metrics">
        {cards.map((c) => (
          <article
            className={`overview-card metric review-metric-${c.state}`}
            key={c.state}
          >
            <span className="metric-icon">
              <c.icon size={24} />
            </span>
            <div>
              <h2>{c.label}</h2>
              <strong>
                {submissions.filter((s) => s.status === c.state).length}
              </strong>
              <p>Dalam daftar termuat</p>
            </div>
          </article>
        ))}
      </div>
      <Tabs
        value={filter}
        onValueChange={(v) => {
          setFilter(String(v));
          setPage(0);
        }}
        className="overview-card review-table-card"
      >
        <TabsList variant="line" aria-label="Status pengajuan">
          {tabs.map(([value, label]) => (
            <TabsTrigger key={value} value={value}>
              {label}
            </TabsTrigger>
          ))}
        </TabsList>
        <div className="review-filters">
          <label className="review-search" htmlFor="admin-review-search">
            <Search size={18} />
            <Input
              id="admin-review-search"
              aria-label="Cari aplikasi atau versi"
              placeholder="Cari aplikasi atau versi…"
              value={search}
              onChange={(e) => {
                setSearch(e.target.value);
                setPage(0);
              }}
            />
          </label>
          <NativeSelect
            aria-label="Urutan pengajuan"
            value={sort}
            onChange={(e) => {
              setSort(e.target.value);
              setPage(0);
            }}
          >
            <NativeSelectOption value="newest">Terbaru</NativeSelectOption>
            <NativeSelectOption value="oldest">Terlama</NativeSelectOption>
          </NativeSelect>
        </div>
        {tabs.map(([value]) => (
          <TabsContent value={value} key={value}>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Aplikasi</TableHead>
                  <TableHead>Versi</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Diajukan</TableHead>
                  <TableHead>Aksi</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.map((s) => (
                  <TableRow key={s.id}>
                    <TableCell>
                      <span className="review-app">
                        <span className="review-icon">
                          <FileCode2 size={21} />
                        </span>
                        <span>
                          {s.snapshot.name}
                          <small>Revisi {s.draftRevision}</small>
                        </span>
                      </span>
                    </TableCell>
                    <TableCell>v{s.version}</TableCell>
                    <TableCell>
                      <span className={`status status-${s.status}`}>
                        {statuses[s.status] ?? s.status}
                      </span>
                    </TableCell>
                    <TableCell>
                      {new Date(s.createdAt).toLocaleString('id-ID', {
                        dateStyle: 'medium',
                        timeStyle: 'short',
                      })}
                    </TableCell>
                    <TableCell>
                      <Button
                        className="review-open"
                        variant="outline"
                        disabled={busy}
                        onClick={() => open(s.id)}
                      >
                        Lihat detail
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
                {!rows.length && (
                  <TableRow>
                    <TableCell colSpan={5} className="overview-empty">
                      {submissions.length
                        ? 'Tidak ada pengajuan yang cocok dengan filter.'
                        : 'Belum ada pengajuan review.'}
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </TabsContent>
        ))}
        <div className="review-pagination">
          <span>
            {filtered.length
              ? `Menampilkan ${current * 6 + 1}–${Math.min((current + 1) * 6, filtered.length)} dari ${filtered.length} pengajuan`
              : '0 pengajuan'}
            <small>Maksimal 200 pengajuan terbaru dari server</small>
          </span>
          <div>
            <Button
              variant="outline"
              size="icon"
              aria-label="Halaman sebelumnya"
              disabled={current === 0}
              onClick={() => setPage(current - 1)}
            >
              <ChevronLeft size={16} />
            </Button>
            <span aria-live="polite">
              {current + 1} / {totalPages}
            </span>
            <Button
              variant="outline"
              size="icon"
              aria-label="Halaman berikutnya"
              disabled={current + 1 >= totalPages}
              onClick={() => setPage(current + 1)}
            >
              <ChevronRight size={16} />
            </Button>
          </div>
        </div>
      </Tabs>
      <div className="review-info">
        <Info size={20} />
        <p>
          Persetujuan review tidak otomatis memublikasikan aplikasi. Lanjutkan
          melalui proses rilis.
        </p>
      </div>
    </section>
  );
}
