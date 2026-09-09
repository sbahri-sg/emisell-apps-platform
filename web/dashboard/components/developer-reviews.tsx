'use client';

import { ArrowUpRight, Inbox, RefreshCw, Search } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
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
import { statuses, type Draft, type Submission } from '@/lib/portal';
import { forApp } from '@/lib/developer-navigation';

export default function DeveloperReviews({
  submissions,
  app,
  busy,
  search,
  setSearch,
  filter,
  setFilter,
  open,
  refresh,
  back,
}: {
  submissions: Submission[];
  app: Draft | null;
  busy: boolean;
  search: string;
  setSearch: (value: string) => void;
  filter: string;
  setFilter: (value: string) => void;
  open: (id: string) => void;
  refresh: () => void;
  back: () => void;
}) {
  const rows = forApp(submissions, app?.id, (row) => row.appId);
  const visible = rows
    .filter(
      (row) =>
        (filter === 'all' || row.status === filter) &&
        `${row.snapshot.name} ${row.version}`
          .toLowerCase()
          .includes(search.trim().toLowerCase()),
    )
    .sort((a, b) => b.createdAt.localeCompare(a.createdAt));
  return (
    <section className="dev-reviews">
      <div className="page-heading">
        <div>
          <h1>Pengajuan review</h1>
          <p>
            {app ? app.document.name : 'Semua aplikasi organisasi'} · Keputusan
            dan masukan untuk setiap versi.
          </p>
        </div>
        <Button variant="outline" disabled={busy} onClick={refresh}>
          <RefreshCw />
          Muat ulang
        </Button>
      </div>
      <div className="dev-review-toolbar">
        <div className="dev-review-search">
          <Search aria-hidden="true" />
          <Input
            aria-label="Cari pengajuan"
            placeholder="Cari nama aplikasi atau versi"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
        <NativeSelect
          aria-label="Status pengajuan"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
        >
          <NativeSelectOption value="all">Semua status</NativeSelectOption>
          {Object.entries(statuses).map(([value, label]) => (
            <NativeSelectOption key={value} value={value}>
              {label}
            </NativeSelectOption>
          ))}
        </NativeSelect>
      </div>
      <output className="dev-caption">
        {visible.length} pengajuan ditampilkan · Maksimal 200 pengajuan terbaru
        organisasi.
      </output>
      {!visible.length ? (
        <Empty className="portal-empty">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <Inbox />
            </EmptyMedia>
            <EmptyTitle>
              {rows.length
                ? 'Tidak ada pengajuan yang cocok'
                : 'Belum ada pengajuan'}
            </EmptyTitle>
            <EmptyDescription>
              {rows.length
                ? 'Coba kata pencarian atau status lainnya.'
                : 'Lengkapi konfigurasi aplikasi, simpan draft, lalu ajukan versi untuk ditinjau.'}
            </EmptyDescription>
          </EmptyHeader>
          <Button
            variant="outline"
            disabled={busy}
            onClick={
              rows.length
                ? () => {
                    setSearch('');
                    setFilter('all');
                  }
                : back
            }
          >
            {rows.length
              ? 'Hapus filter'
              : app
                ? 'Buka aplikasi'
                : 'Lihat aplikasi'}
          </Button>
        </Empty>
      ) : (
        <div className="dev-review-list">
          {visible.map((row) => (
            <button
              type="button"
              className="dev-review-row"
              key={row.id}
              disabled={busy}
              onClick={() => open(row.id)}
            >
              <span className="dev-review-main">
                <strong>{row.snapshot.name}</strong>
                <span>
                  Versi {row.version} · Draft r{row.draftRevision}
                </span>
                {row.feedback && (
                  <span className="dev-review-feedback">{row.feedback}</span>
                )}
              </span>
              <span className={`status status-${row.status}`}>
                {statuses[row.status] ?? row.status}
              </span>
              <time dateTime={row.createdAt}>
                {new Date(row.createdAt).toLocaleDateString('id-ID', {
                  day: 'numeric',
                  month: 'short',
                  year: 'numeric',
                })}
              </time>
              <ArrowUpRight aria-hidden="true" />
            </button>
          ))}
        </div>
      )}
      <p className="dev-footnote">
        Review yang disetujui belum otomatis memublikasikan aplikasi atau
        memberi akses ke data toko.
      </p>
    </section>
  );
}
