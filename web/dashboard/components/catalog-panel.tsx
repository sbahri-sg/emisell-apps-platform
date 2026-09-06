'use client';

import { useState } from 'react';
import type { ScopeDeclaration } from '@/lib/access-scopes';
import { ScopeSummary } from '@/components/scope-summary';
import {
  publicDistributionAllowed,
  paymentBoundaryMessage,
} from '@/lib/distribution';
import {
  ArrowLeft,
  ArrowUpRight,
  Download,
  FileCheck2,
  PackageCheck,
  ShieldCheck,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
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
import {
  PortalAPI,
  type Draft,
  type Session,
  type Submission,
} from '@/lib/portal';

type Manifest = {
  schema: string;
  policy: string;
  appId: string;
  developerId: string;
  version: string;
  name: string;
  summary: string;
  description: string;
  capability: string;
  scopes: string[];
  runtime: string;
  pricing: 'free';
  installable: false;
  sourceSha256: string;
  accessScopes?: ScopeDeclaration;
};
type SignedPackage = {
  manifest: Manifest;
  sha256: string;
  keyId: string;
  signature: string;
};
export type CatalogRelease = {
  id: string;
  submissionId: string;
  package: SignedPackage;
  status: 'signed' | 'published' | 'suspended';
  revision: number;
  createdAt: string;
  updatedAt: string;
};
type Detail = {
  release: CatalogRelease;
  history: {
    id: string;
    action: string;
    reason: string;
    actorId: string;
    occurredAt: string;
  }[];
  trustedPublicKey: string | null;
};
type Perform = (action: () => Promise<void>, message?: string) => Promise<void>;
const state = {
  signed: 'Ditandatangani',
  published: 'Dipublikasikan',
  suspended: 'Ditangguhkan',
};
function download(name: string, content: string) {
  const url = URL.createObjectURL(
    new Blob([content], { type: 'application/json' }),
  );
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = name;
  anchor.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

export function CatalogPanel({
  api,
  session,
  releases,
  submissions,
  busy,
  perform,
  refresh,
}: {
  api: PortalAPI;
  session: Session;
  releases: CatalogRelease[];
  submissions: Submission[];
  busy: boolean;
  perform: Perform;
  refresh: () => Promise<void>;
}) {
  const [detail, setDetail] = useState<Detail | null>(null);
  const [reason, setReason] = useState('');
  const canPublish =
    session.user.surface === 'admin' && session.user.role === 'administrator';
  const eligible = submissions.filter(
    (v) =>
      v.status === 'approved' &&
      publicDistributionAllowed(v.snapshot.capability) &&
      !releases.some((r) => r.submissionId === v.id),
  );
  const open = (id: string) =>
    perform(async () => {
      setDetail(await api.request<Detail>(`/catalog/${id}`));
      setReason('');
    });
  if (detail) {
    const release = detail.release;
    const m = release.package.manifest;
    const reserved = !publicDistributionAllowed(m.capability);
    const action =
      release.status === 'published' || reserved ? 'suspended' : 'published';
    return (
      <>
        <Button variant="ghost" disabled={busy} onClick={() => setDetail(null)}>
          <ArrowLeft />
          Kembali ke rilis
        </Button>
        <div className="page-heading">
          <div>
            <p className="eyebrow">CATALOG RELEASE</p>
            <h1>{m.name}</h1>
            <p>
              v{m.version} · {m.capability}
            </p>
          </div>
          <span className={`status status-${release.status}`}>
            {state[release.status]}
          </span>
        </div>
        <div className="catalog-columns">
          <section className="portal-panel">
            <h2>Integritas metadata</h2>
            {reserved && (
              <p className="field-help">
                {paymentBoundaryMessage} Listing tidak ditampilkan di App Store
                meskipun status historis published.
              </p>
            )}
            <p>
              Signature menjamin metadata tidak berubah. Ini bukan sertifikasi
              keamanan kode aplikasi atau izin eksekusi.
            </p>
            <dl className="catalog-facts">
              <dt>Schema</dt>
              <dd>{m.schema}</dd>
              <dt>Policy</dt>
              <dd>{m.policy}</dd>
              <dt>SHA-256</dt>
              <dd>
                <code>{release.package.sha256}</code>
              </dd>
              <dt>Signing key ID</dt>
              <dd>
                <code>{release.package.keyId}</code>
              </dd>
            </dl>
            <ScopeSummary value={m.accessScopes} />
            <div className="catalog-actions">
              <Button
                variant="outline"
                onClick={() =>
                  download(
                    `${m.appId}-${m.version}-catalog.json`,
                    JSON.stringify(release.package, null, 2),
                  )
                }
              >
                <Download />
                Unduh paket bertanda tangan
              </Button>
              {detail.trustedPublicKey && (
                <Button
                  variant="ghost"
                  onClick={() =>
                    download('catalog-public-key.txt', detail.trustedPublicKey!)
                  }
                >
                  <Download />
                  Public key
                </Button>
              )}
            </div>
            {canPublish && !(reserved && release.status === 'suspended') && (
              <form
                className="catalog-publish"
                onSubmit={(e) => {
                  e.preventDefault();
                  void perform(
                    async () => {
                      await api.request(
                        `/catalog/${release.id}/status`,
                        'POST',
                        { status: action, revision: release.revision, reason },
                        true,
                      );
                      await refresh();
                      setDetail(await api.request(`/catalog/${release.id}`));
                      setReason('');
                    },
                    action === 'published'
                      ? 'Rilis tampil di App Store.'
                      : 'Rilis ditangguhkan dan tidak tampil di App Store.',
                  );
                }}
              >
                <h2>
                  {action === 'published'
                    ? 'Publikasi katalog'
                    : 'Tangguhkan listing'}
                </h2>
                <p>
                  {action === 'published'
                    ? 'Hanya satu versi per aplikasi boleh tampil. Tangguhkan versi lama sebelum menerbitkan versi pengganti.'
                    : 'Listing akan disembunyikan. Data rilis dan audit tetap disimpan.'}
                </p>
                <label htmlFor="catalog-reason">
                  Alasan tindakan
                  <Textarea
                    id="catalog-reason"
                    required
                    maxLength={2000}
                    value={reason}
                    disabled={busy}
                    onChange={(e) => setReason(e.target.value)}
                    placeholder="Catat alasan untuk audit…"
                  />
                </label>
                <Button disabled={busy || !reason.trim()} type="submit">
                  {action === 'published' ? <ArrowUpRight /> : <ShieldCheck />}
                  {action === 'published'
                    ? 'Publikasikan di App Store'
                    : 'Tangguhkan listing'}
                </Button>
              </form>
            )}
          </section>
          <section className="portal-panel">
            <h2>Riwayat rilis</h2>
            <ol className="catalog-history">
              {detail.history.map((h) => (
                <li key={h.id}>
                  <strong>
                    {state[h.action as keyof typeof state] ?? h.action}
                  </strong>
                  <p>{h.reason}</p>
                  <small>
                    {new Date(h.occurredAt).toLocaleString('id-ID')} ·{' '}
                    {h.actorId}
                  </small>
                </li>
              ))}
            </ol>
            <p className="catalog-help">
              Menampilkan maksimal 200 tindakan awal.
            </p>
          </section>
        </div>
      </>
    );
  }
  return (
    <>
      <div className="page-heading">
        <div>
          <p className="eyebrow">RELEASE MANAGEMENT</p>
          <h1>Rilis & publikasi</h1>
          <p>
            Katalog aplikasi gratis, terpisah dari installation dan runtime.
          </p>
        </div>
        <a
          className="catalog-store-link"
          href="http://localhost:4318/"
          target="_blank"
          rel="noreferrer"
        >
          Buka App Store <ArrowUpRight />
        </a>
      </div>
      <div className="portal-callout">
        <ShieldCheck />
        <span>
          Review disetujui → metadata ditandatangani → publikasi oleh
          Administrator. Rilis katalog belum dapat di-install.
        </span>
      </div>
      {eligible.length > 0 && (
        <section className="portal-panel">
          <h2>Metadata disetujui</h2>
          <p>
            {canPublish
              ? 'Validasi dan tandatangani sebelum diterbitkan. Penandatanganan tidak otomatis memublikasikan.'
              : 'Menunggu Administrator menandatangani dan memublikasikan rilis katalog.'}
          </p>
          <div className="catalog-list">
            {eligible.map((s) => (
              <div className="catalog-row" key={s.id}>
                <span>
                  <strong>{s.snapshot.name}</strong>
                  <small>
                    v{s.version} · {s.snapshot.capability}
                  </small>
                </span>
                {canPublish ? (
                  <Button
                    disabled={busy}
                    variant="outline"
                    onClick={() =>
                      void perform(async () => {
                        const value = await api.request<{
                          release: CatalogRelease;
                        }>(`/submissions/${s.id}/catalog`, 'POST', {}, true);
                        await refresh();
                        setDetail(
                          await api.request(`/catalog/${value.release.id}`),
                        );
                      }, 'Metadata tervalidasi dan ditandatangani. Belum dipublikasikan.')
                    }
                  >
                    <FileCheck2 />
                    Validasi & tanda tangani
                  </Button>
                ) : (
                  <span className="status status-approved">Disetujui</span>
                )}
              </div>
            ))}
          </div>
        </section>
      )}
      <section className="portal-panel">
        <h2>Rilis katalog</h2>
        {releases.length ? (
          <div className="catalog-list">
            {releases.map((r) => (
              <button
                className="catalog-row catalog-row-button"
                disabled={busy}
                key={r.id}
                onClick={() => void open(r.id)}
              >
                <span>
                  <strong>{r.package.manifest.name}</strong>
                  <small>
                    v{r.package.manifest.version} ·{' '}
                    {r.package.manifest.capability}
                  </small>
                </span>
                <span className={`status status-${r.status}`}>
                  {state[r.status]}
                </span>
                <ArrowUpRight />
              </button>
            ))}
          </div>
        ) : (
          <Empty>
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <PackageCheck />
              </EmptyMedia>
              <EmptyTitle>Belum ada rilis bertanda tangan</EmptyTitle>
              <EmptyDescription>
                Ajukan metadata melalui Portal Developer. Setelah review
                disetujui, Administrator dapat menyiapkan rilis.
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        )}
        <p className="catalog-help">
          Maksimal 200 rilis terbaru sesuai akses akun.
        </p>
      </section>
    </>
  );
}

export function DeveloperTools({
  api,
  drafts,
  busy,
  perform,
}: {
  api: PortalAPI;
  drafts: Draft[];
  busy: boolean;
  perform: Perform;
}) {
  const [id, setID] = useState('');
  const [tooling, setTooling] = useState<{
    manifest: Manifest;
    valid: boolean;
    checks: string[];
  } | null>(null);
  return (
    <>
      <div className="page-heading">
        <div>
          <p className="eyebrow">DEVELOPER TOOLING</p>
          <h1>Validasi & ekspor metadata</h1>
          <p>Periksa draft tersimpan sebelum diajukan untuk review.</p>
        </div>
      </div>
      <section className="portal-panel">
        <label htmlFor="tooling-app">
          Aplikasi
          <NativeSelect
            id="tooling-app"
            value={id}
            disabled={busy}
            onChange={(e) => {
              setID(e.target.value);
              setTooling(null);
            }}
          >
            <NativeSelectOption value="">Pilih aplikasi…</NativeSelectOption>
            {drafts.map((d) => (
              <NativeSelectOption key={d.id} value={d.id}>
                {d.document.name} · v{d.document.version}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </label>
        <div className="catalog-actions">
          <Button
            disabled={busy || !id}
            onClick={() =>
              void perform(async () =>
                setTooling(await api.request(`/apps/${id}/tooling`)),
              )
            }
          >
            <FileCheck2 />
            Validasi draft
          </Button>
          {tooling?.valid && (
            <Button
              variant="outline"
              onClick={() =>
                download(
                  `${id}-catalog-draft.json`,
                  JSON.stringify(tooling.manifest, null, 2),
                )
              }
            >
              <Download />
              Ekspor metadata
            </Button>
          )}
        </div>
        {tooling && (
          <>
            <h2>
              {tooling.valid
                ? 'Metadata lengkap untuk diajukan'
                : 'Draft perlu dilengkapi'}
            </h2>
            <ul className="tooling-checks">
              {tooling.checks.map((check) => (
                <li key={check}>{check}</li>
              ))}
            </ul>
            <pre className="catalog-json">
              <code>{JSON.stringify(tooling.manifest, null, 2)}</code>
            </pre>
          </>
        )}
        {!drafts.length && (
          <p>
            Belum ada aplikasi. Buat dan simpan draft melalui “Aplikasi saya”.
          </p>
        )}
      </section>
      <section className="portal-panel">
        <h2>Verifikasi dari CLI</h2>
        <p>
          Gunakan public key yang diberikan pengelola platform melalui jalur
          tepercaya.
        </p>
        <pre className="catalog-json">
          <code>
            {
              'go run ./cmd/cli catalog-validate <catalog-draft.json>\ngo run ./cmd/cli catalog-verify <catalog-package.json> <trusted-key.txt>'
            }
          </code>
        </pre>
        <p className="catalog-help">
          Tooling ini mencakup metadata katalog. Untuk konfigurasi endpoint,
          callback dan health, gunakan menu Rilis integrasi setelah metadata
          disetujui. Runtime umum, upload artifact dan security scan app-code
          belum tersedia.
        </p>
      </section>
    </>
  );
}
