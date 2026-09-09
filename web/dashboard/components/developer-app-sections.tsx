'use client';

import { useState } from 'react';
import { Activity, ArrowUpRight, RefreshCw, Tag, Terminal } from 'lucide-react';
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
import { developerActivity } from '@/lib/developer-activity';
import { forApp } from '@/lib/developer-navigation';

type Props = {
  app: Draft;
  submissions: Submission[];
  busy: boolean;
  navigate: (view: string) => void;
  edit: () => void;
  openReview: (id: string) => void;
  refresh: () => void;
};
const date = (value: string) => new Date(value).toLocaleString('id-ID');
const label = (status: string) =>
  statuses[status] ?? (status === 'draft' ? 'Draft' : status);

export function DeveloperVersions({
  app,
  submissions,
  busy,
  navigate,
  edit,
  openReview,
  refresh,
}: Props) {
  const reviews = forApp(submissions, app.id, (row) => row.appId)
    .slice()
    .sort((a, b) => b.createdAt.localeCompare(a.createdAt));
  return (
    <section className="dev-app-section">
      <div className="page-heading">
        <div>
          <h1>Versions</h1>
          <p>{app.document.name} · Versi aktif dan konfigurasi berikutnya.</p>
        </div>
        <Button disabled={busy} onClick={edit}>
          <Tag />
          {app.activeVersion
            ? 'New version'
            : reviews.length
              ? 'Siapkan versi berikutnya'
              : 'Lengkapi versi pertama'}
        </Button>
      </div>
      {app.activeVersion && (
        <section className="dev-card dev-version-draft">
          <div>
            <h2>
              {app.activeVersion.document.version}{' '}
              <span className="status status-approved">Active</span>
            </h2>
            <p>Diaktifkan {date(app.activeVersion.activatedAt)}</p>
          </div>
        </section>
      )}
      {(!app.activeVersion || app.revision !== app.activeVersion.revision) && (
        <section className="dev-card dev-version-draft">
          <div>
            <h2>
              v{app.document.version}{' '}
              <span className="status status-draft">Draft r{app.revision}</span>
            </h2>
            <p>Terakhir disimpan {date(app.updatedAt)}</p>
          </div>
          <Button variant="outline" disabled={busy} onClick={edit}>
            Edit draft <ArrowUpRight />
          </Button>
        </section>
      )}
      {(!app.activeVersion || reviews.length > 0) && (
        <>
          <div className="dev-section-heading">
            <h2>Riwayat pengajuan versi</h2>
            <Button variant="ghost" disabled={busy} onClick={refresh}>
              <RefreshCw />
              Muat ulang
            </Button>
          </div>
          {reviews.length ? (
            <div className="dev-review-list">
              {reviews.map((row) => (
                <button
                  className="dev-review-row"
                  type="button"
                  key={row.id}
                  disabled={busy}
                  onClick={() => openReview(row.id)}
                >
                  <span className="dev-review-main">
                    <strong>v{row.version}</strong>
                    <span>Snapshot draft r{row.draftRevision}</span>
                  </span>
                  <span className={`status status-${row.status}`}>
                    {label(row.status)}
                  </span>
                  <time dateTime={row.createdAt}>{date(row.createdAt)}</time>
                  <ArrowUpRight />
                </button>
              ))}
            </div>
          ) : (
            <Empty className="portal-empty">
              <EmptyHeader>
                <EmptyMedia variant="icon">
                  <Tag />
                </EmptyMedia>
                <EmptyTitle>Belum ada versi yang diajukan</EmptyTitle>
                <EmptyDescription>
                  Lengkapi endpoint, informasi aplikasi, dan izin di draft.
                  Simpan lalu ajukan untuk review.
                </EmptyDescription>
              </EmptyHeader>
              <Button variant="outline" disabled={busy} onClick={edit}>
                Lengkapi draft
              </Button>
            </Empty>
          )}
        </>
      )}
      <p className="dev-footnote">
        Versi awal aktif setelah aplikasi dibuat. Mengedit konfigurasi
        berikutnya tidak mengubah versi aktif. Instalasi dan izin toko tetap
        terpisah.
      </p>
      <section className="dev-card">
        <h2>Rilis & pengujian</h2>
        <p>
          Kelola tahap lanjutan melalui alat organisasi. Pilih aplikasi ini di
          halaman tujuan.
        </p>
        <div className="dev-section-actions">
          {[
            ['reviews', 'Semua pengajuan aplikasi'],
            ['integration-releases', 'Rilis integrasi'],
            ['ui-releases', 'Rilis UI'],
            ['testing', 'Toko pengujian'],
            ['catalog', 'Katalog'],
          ].map(([view, text]) => (
            <Button
              key={view}
              variant="outline"
              disabled={busy}
              onClick={() => navigate(view)}
            >
              {text}
              <ArrowUpRight />
            </Button>
          ))}
        </div>
      </section>
    </section>
  );
}

export function DeveloperMonitoring({
  app,
  submissions,
  busy,
  navigate,
  refresh,
}: Props) {
  const latest = forApp(submissions, app.id, (row) => row.appId)
    .slice()
    .sort((a, b) => b.createdAt.localeCompare(a.createdAt))[0];
  return (
    <section className="dev-app-section">
      <div className="page-heading">
        <div>
          <h1>Monitoring</h1>
          <p>
            {app.document.name} · Status konfigurasi dan ketersediaan metrik.
          </p>
        </div>
        <Button variant="outline" disabled={busy} onClick={refresh}>
          <RefreshCw />
          Muat ulang
        </Button>
      </div>
      <section className="dev-card">
        <h2>
          <Activity />
          Status konfigurasi
        </h2>
        <dl className="dev-app-details">
          <div>
            <dt>Versi draft</dt>
            <dd>{app.document.version}</dd>
          </div>
          <div>
            <dt>Endpoint</dt>
            <dd>
              {app.document.endpoint
                ? 'Tersimpan dalam draft'
                : 'Belum dikonfigurasi'}
            </dd>
          </div>
          <div>
            <dt>Review terakhir</dt>
            <dd>{latest ? label(latest.status) : 'Belum diajukan'}</dd>
          </div>
        </dl>
        <p className="dev-caption">
          Status ini berasal dari konfigurasi dan pengajuan, bukan pemeriksaan
          kesehatan endpoint runtime.
        </p>
        <Button
          variant="outline"
          disabled={busy}
          onClick={() => navigate('versions')}
        >
          Periksa versi <ArrowUpRight />
        </Button>
      </section>
      <div className="dev-monitoring-grid">
        {[
          [
            'API requests',
            'Jumlah request, latensi, dan tingkat error belum tersedia dari API portal.',
          ],
          [
            'Embedded performance',
            'Pengukuran performa antarmuka aplikasi belum terhubung ke portal.',
          ],
          [
            'Webhooks',
            'Statistik pengiriman dan kegagalan webhook belum tersedia untuk aplikasi ini di portal.',
          ],
        ].map(([title, text]) => (
          <section className="dev-card" key={title}>
            <h2>{title}</h2>
            <span className="status status-draft">Belum tersedia</span>
            <p>{text}</p>
          </section>
        ))}
      </div>
      <p className="dev-footnote">
        Belum tersedia bukan berarti tidak ada error. Lihat log server aplikasi
        untuk memeriksa runtime.
      </p>
    </section>
  );
}

export function DeveloperLogs({
  app,
  submissions,
  busy,
  openReview,
  refresh,
}: Props) {
  const [search, setSearch] = useState('');
  const [kind, setKind] = useState('all');
  const [range, setRange] = useState('all');
  const [now, setNow] = useState(() => Date.now());
  const events = developerActivity(app, submissions).filter(
    (event) =>
      (kind === 'all' || event.type === kind) &&
      (range === 'all' ||
        Date.parse(event.at) >= now - Number(range) * 86400000) &&
      `${event.title} ${event.feedback ?? ''} ${label(event.status)}`
        .toLowerCase()
        .includes(search.trim().toLowerCase()),
  );
  return (
    <section className="dev-app-section">
      <div className="page-heading">
        <div>
          <h1>Logs</h1>
          <p>{app.document.name} · Aktivitas versi dan review.</p>
        </div>
        <Button
          variant="outline"
          disabled={busy}
          onClick={() => {
            setNow(Date.now());
            refresh();
          }}
        >
          <RefreshCw />
          Muat ulang
        </Button>
      </div>
      <div className="portal-callout">
        <Terminal />
        <span>
          Yang tersedia saat ini adalah aktivitas draft dan pengajuan. Log
          request API, webhook, dan runtime aplikasi belum terhubung.
        </span>
      </div>
      <div className="dev-log-toolbar">
        <Input
          aria-label="Cari aktivitas"
          placeholder="Cari aktivitas atau feedback"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <NativeSelect
          aria-label="Rentang aktivitas"
          value={range}
          onChange={(e) => {
            setRange(e.target.value);
            setNow(Date.now());
          }}
        >
          <NativeSelectOption value="all">Semua waktu</NativeSelectOption>
          <NativeSelectOption value="1">24 jam terakhir</NativeSelectOption>
          <NativeSelectOption value="7">7 hari terakhir</NativeSelectOption>
          <NativeSelectOption value="30">30 hari terakhir</NativeSelectOption>
        </NativeSelect>
        <NativeSelect
          aria-label="Jenis aktivitas"
          value={kind}
          onChange={(e) => setKind(e.target.value)}
        >
          <NativeSelectOption value="all">Semua jenis</NativeSelectOption>
          <NativeSelectOption value="draft">Draft</NativeSelectOption>
          <NativeSelectOption value="submission">Pengajuan</NativeSelectOption>
          <NativeSelectOption value="decision">
            Keputusan review
          </NativeSelectOption>
        </NativeSelect>
      </div>
      <output className="dev-caption">
        {events.length} aktivitas ditampilkan
      </output>
      <div className="dev-review-list">
        {events.map((event) => (
          <div className="dev-log-row" key={event.id}>
            <div className="dev-review-main">
              <strong>{event.title}</strong>
              {event.feedback && <span>{event.feedback}</span>}
              <time dateTime={event.at}>{date(event.at)}</time>
            </div>
            <span className={`status status-${event.status}`}>
              {label(event.status)}
            </span>
            {event.submissionId && (
              <Button
                size="icon"
                variant="ghost"
                aria-label={`Detail ${event.title}`}
                disabled={busy}
                onClick={() => openReview(event.submissionId!)}
              >
                <ArrowUpRight />
              </Button>
            )}
          </div>
        ))}
      </div>
      {!events.length && (
        <Empty>
          <EmptyHeader>
            <EmptyTitle>Tidak ada aktivitas yang cocok</EmptyTitle>
            <EmptyDescription>
              Coba rentang waktu atau jenis aktivitas lainnya.
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      )}
      <p className="dev-footnote">
        Berdasarkan draft terakhir dan maksimal 200 pengajuan terbaru
        organisasi, bukan seluruh audit historis.
      </p>
    </section>
  );
}
