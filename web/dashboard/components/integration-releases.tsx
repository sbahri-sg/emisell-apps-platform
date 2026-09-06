'use client';

import { useEffect, useState } from 'react';
import {
  ArrowLeft,
  ArrowUpRight,
  Download,
  RefreshCw,
  ShieldCheck,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import {
  NativeSelect,
  NativeSelectOption,
} from '@/components/ui/native-select';
import { ScopeSummary } from '@/components/scope-summary';
import { publicDistributionAllowed } from '@/lib/distribution';
import type { PortalAPI, Session, Submission } from '@/lib/portal';
import {
  integrationActions,
  integrationStatus,
  type IntegrationRelease,
  type IntegrationDetail,
  type IntegrationReport,
  type IntegrationStatus,
} from '@/lib/integration-releases';

function download(name: string, value: unknown) {
  const url = URL.createObjectURL(
    new Blob(
      [typeof value === 'string' ? value : JSON.stringify(value, null, 2)],
      { type: 'application/json' },
    ),
  );
  const a = document.createElement('a');
  a.href = url;
  a.download = name;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
function Validation({ report }: { report: IntegrationReport }) {
  return (
    <section className="portal-panel">
      <h2>
        {report.valid
          ? 'Pemeriksaan konfigurasi lulus'
          : 'Konfigurasi perlu diperbaiki'}
      </h2>
      <ul className="tooling-checks">
        {report.checks.map((c) => (
          <li key={c.code}>
            <strong>{c.passed ? 'Lulus' : 'Gagal'}</strong> — {c.message}
          </li>
        ))}
      </ul>
      <h3>Belum dapat di-install</h3>
      <ul className="tooling-checks">
        {report.blockers.map((b) => (
          <li key={b}>{b}</li>
        ))}
      </ul>
      <p className="catalog-help">
        Hasil ini bukan grant akses, sertifikasi keamanan kode, atau verifikasi
        server developer secara langsung.
      </p>
    </section>
  );
}

export default function IntegrationReleases({
  api,
  session,
  submissions,
  busy,
  perform,
}: {
  api: PortalAPI;
  session: Session;
  submissions: Submission[];
  busy: boolean;
  perform: (action: () => Promise<void>, message?: string) => Promise<void>;
}) {
  const [releases, setReleases] = useState<IntegrationRelease[] | null>(null);
  const [loadError, setLoadError] = useState('');
  const [detail, setDetail] = useState<IntegrationDetail | null>(null);
  const [submissionId, setSubmissionId] = useState('');
  const [callbackUrl, setCallbackUrl] = useState('');
  const [healthUrl, setHealthUrl] = useState('');
  const [validation, setValidation] = useState<IntegrationReport | null>(null);
  const [reason, setReason] = useState('');
  const developer = session.user.surface === 'developer';
  useEffect(() => {
    let active = true;
    api
      .request<{ releases: IntegrationRelease[] }>('/integration-releases')
      .then((v) => {
        if (active) setReleases(v.releases);
      })
      .catch((e: Error) => {
        if (active) setLoadError(e.message);
      });
    return () => {
      active = false;
    };
  }, [api]);
  const refresh = async () => {
    const v = await api.request<{ releases: IntegrationRelease[] }>(
      '/integration-releases',
    );
    setReleases(v.releases);
    setLoadError('');
  };
  const open = (id: string) =>
    perform(async () => {
      setDetail(await api.request(`/integration-releases/${id}`));
      setReason('');
    });
  const selected = submissions.find((s) => s.id === submissionId);
  const input = {
    submissionId,
    config: {
      protocol: 'emisell.capability-http/v1',
      endpoint: selected?.snapshot.endpoint ?? '',
      callbackUrl,
      healthUrl,
    },
  };
  const eligible = submissions.filter(
    (s) =>
      s.status === 'approved' &&
      publicDistributionAllowed(s.snapshot.capability) &&
      !releases?.some(
        (r) =>
          r.manifest.submissionId === s.id ||
          (r.manifest.metadata.appId === s.appId &&
            r.manifest.metadata.version === s.version),
      ),
  );
  const labels: Partial<Record<IntegrationStatus, string>> = {
    approved: 'Setujui konfigurasi',
    rejected: 'Tolak konfigurasi',
    signed: 'Tanda tangani konfigurasi',
    suspended: 'Tangguhkan rilis',
  };
  if (detail) {
    const r = detail.release;
    const m = r.manifest;
    const actions = integrationActions(
      session.user.surface,
      session.user.role,
      r.status,
    );
    return (
      <>
        <Button variant="ghost" disabled={busy} onClick={() => setDetail(null)}>
          <ArrowLeft />
          Kembali ke rilis integrasi
        </Button>
        <div className="page-heading">
          <div>
            <p className="eyebrow">APLIKASI INTEGRASI</p>
            <h1>{m.metadata.name}</h1>
            <p>
              v{m.metadata.version} · {m.metadata.capability}
            </p>
          </div>
          <span className={`status status-${r.status}`}>
            {integrationStatus[r.status]}
          </span>
        </div>
        <div className="catalog-columns">
          <div>
            <section className="portal-panel">
              <h2>Snapshot konfigurasi</h2>
              <p>
                Snapshot tidak dapat diedit. Perbaikan setelah diajukan
                memerlukan versi baru dan review metadata baru.
              </p>
              <dl className="catalog-facts">
                <dt>Schema</dt>
                <dd>{m.schema}</dd>
                <dt>Profil</dt>
                <dd>{m.config.protocol}</dd>
                <dt>Endpoint</dt>
                <dd>
                  <code>{m.config.endpoint}</code>
                </dd>
                <dt>Callback OAuth</dt>
                <dd>
                  <code>{m.config.callbackUrl}</code>
                </dd>
                <dt>Health</dt>
                <dd>
                  <code>{m.config.healthUrl}</code>
                </dd>
                <dt>SHA-256</dt>
                <dd>
                  <code>{r.sha256}</code>
                </dd>
                {r.package && (
                  <>
                    <dt>Signing key</dt>
                    <dd>
                      <code>{r.package.keyId}</code>
                    </dd>
                  </>
                )}
              </dl>
              <p>
                Scope capability reference: {m.metadata.scopes.join(', ')}.
                Deklarasi ini belum memberi grant.
              </p>
              <ScopeSummary value={m.metadata.accessScopes} />
              <div className="catalog-actions">
                <Button
                  variant="outline"
                  onClick={() =>
                    download(`${r.id}-configuration.json`, r.package ?? m)
                  }
                >
                  <Download />
                  {r.package ? 'Unduh paket bertanda tangan' : 'Unduh snapshot'}
                </Button>
                {detail.trustedPublicKey && (
                  <Button
                    variant="ghost"
                    onClick={() =>
                      download(
                        'integration-public-key.txt',
                        detail.trustedPublicKey!,
                      )
                    }
                  >
                    Public key integrasi
                  </Button>
                )}
              </div>
            </section>
            <Validation report={detail.validation} />
          </div>
          <section className="portal-panel">
            <h2>Review konfigurasi</h2>
            <p>
              Terpisah dari review metadata dan publikasi App Store. Persetujuan
              tidak mengaktifkan aplikasi.
            </p>
            {actions.length > 0 && (
              <div className="catalog-publish">
                <label htmlFor="integration-reason">
                  Alasan / catatan review
                  <Textarea
                    id="integration-reason"
                    required
                    maxLength={2000}
                    value={reason}
                    disabled={busy}
                    onChange={(e) => setReason(e.target.value)}
                  />
                </label>
                <div className="catalog-actions">
                  {actions.map((action) => (
                    <Button
                      key={action}
                      variant={
                        action === 'rejected' || action === 'suspended'
                          ? 'outline'
                          : 'default'
                      }
                      disabled={
                        busy ||
                        !reason.trim() ||
                        ((action === 'approved' || action === 'signed') &&
                          !detail.validation.valid)
                      }
                      onClick={() =>
                        void perform(async () => {
                          await api.request(
                            `/integration-releases/${r.id}/status`,
                            'POST',
                            { status: action, revision: r.revision, reason },
                            true,
                          );
                          await refresh();
                          setDetail(
                            await api.request(`/integration-releases/${r.id}`),
                          );
                          setReason('');
                        }, 'Status konfigurasi diperbarui. Aplikasi tetap belum dapat di-install.')
                      }
                    >
                      <ShieldCheck />
                      {labels[action]}
                    </Button>
                  ))}
                </div>
              </div>
            )}
            <h3>Riwayat audit</h3>
            <ol className="catalog-history">
              {detail.history.map((h) => (
                <li key={h.id}>
                  <strong>
                    {integrationStatus[h.action as IntegrationStatus] ??
                      h.action}
                  </strong>
                  <p>{h.reason}</p>
                  <small>
                    {new Date(h.occurredAt).toLocaleString('id-ID')} ·{' '}
                    {h.actorId}
                  </small>
                </li>
              ))}
            </ol>
            <p className="catalog-help">Maksimal 200 tindakan awal.</p>
          </section>
        </div>
      </>
    );
  }
  return (
    <>
      <div className="page-heading">
        <div>
          <p className="eyebrow">APLIKASI INTEGRASI</p>
          <h1>Rilis integrasi</h1>
          <p>
            Konfigurasi → validasi → review → signature. Terpisah dari publikasi
            katalog.
          </p>
        </div>
        <Button
          variant="outline"
          disabled={busy}
          onClick={() => void perform(refresh)}
        >
          <RefreshCw />
          Muat ulang
        </Button>
      </div>
      <div className="portal-callout">
        <ShieldCheck />
        <span>
          Release konfigurasi bukan executable. Runtime umum dan
          OAuth/app-client belum tersedia; tidak ada tombol install atau
          penerbitan token dari menu ini.
        </span>
      </div>
      {loadError && <p role="alert">{loadError}</p>}
      {developer && releases !== null && (
        <section className="portal-panel">
          <h2>Ajukan konfigurasi integrasi</h2>
          <p>
            Pilih snapshot metadata yang disetujui. Semua URL wajib HTTPS pada
            origin yang sama, tanpa query, fragment, atau credentials.
          </p>
          {!eligible.length ? (
            <p>
              Belum ada versi yang memenuhi syarat. Buat aplikasi dan ajukan
              review metadata terlebih dahulu; versi yang sudah memiliki rilis
              integrasi tidak dapat diajukan ulang.
            </p>
          ) : (
            <form
              className="catalog-publish"
              onSubmit={(e) => {
                e.preventDefault();
                void perform(async () => {
                  const v = await api.request<{
                    validation: IntegrationReport;
                  }>('/integration-releases/validate', 'POST', input);
                  setValidation(v.validation);
                });
              }}
            >
              <label htmlFor="integration-submission">
                Metadata disetujui
                <NativeSelect
                  id="integration-submission"
                  required
                  value={submissionId}
                  disabled={busy}
                  onChange={(e) => {
                    setSubmissionId(e.target.value);
                    setValidation(null);
                    setCallbackUrl('');
                    setHealthUrl('');
                  }}
                >
                  <NativeSelectOption value="">
                    Pilih aplikasi & versi…
                  </NativeSelectOption>
                  {eligible.map((s) => (
                    <NativeSelectOption key={s.id} value={s.id}>
                      {s.snapshot.name} · v{s.version}
                    </NativeSelectOption>
                  ))}
                </NativeSelect>
              </label>
              {selected && (
                <>
                  <label htmlFor="integration-endpoint">
                    Endpoint dari metadata
                    <Input
                      id="integration-endpoint"
                      value={selected.snapshot.endpoint}
                      readOnly
                    />
                  </label>
                  <label htmlFor="integration-callback">
                    Callback OAuth (deklarasi)
                    <Input
                      id="integration-callback"
                      type="url"
                      required
                      maxLength={2048}
                      value={callbackUrl}
                      disabled={busy}
                      onChange={(e) => {
                        setCallbackUrl(e.target.value);
                        setValidation(null);
                      }}
                      placeholder="https://app.example.com/oauth/callback"
                    />
                  </label>
                  <label htmlFor="integration-health">
                    Health endpoint (belum dipanggil)
                    <Input
                      id="integration-health"
                      type="url"
                      required
                      maxLength={2048}
                      value={healthUrl}
                      disabled={busy}
                      onChange={(e) => {
                        setHealthUrl(e.target.value);
                        setValidation(null);
                      }}
                      placeholder="https://app.example.com/health"
                    />
                  </label>
                </>
              )}
              <div className="catalog-actions">
                <Button type="submit" disabled={busy || !selected}>
                  Validasi konfigurasi
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  disabled={busy || !validation?.valid || !selected}
                  onClick={() =>
                    void perform(async () => {
                      const v = await api.request<{
                        release: IntegrationRelease;
                      }>('/integration-releases', 'POST', input, true);
                      await refresh();
                      setSubmissionId('');
                      setValidation(null);
                      setDetail(
                        await api.request(
                          `/integration-releases/${v.release.id}`,
                        ),
                      );
                    }, 'Konfigurasi diajukan untuk review terpisah.')
                  }
                >
                  <ArrowUpRight />
                  Ajukan review konfigurasi
                </Button>
              </div>
            </form>
          )}
        </section>
      )}
      {validation && <Validation report={validation} />}
      <section className="portal-panel">
        <h2>Daftar rilis integrasi</h2>
        {releases === null ? (
          <p>
            {loadError
              ? 'Data belum tersedia. Coba muat ulang.'
              : 'Memuat rilis…'}
          </p>
        ) : releases.length === 0 ? (
          <p>
            Belum ada rilis integrasi. Pengajuan developer akan tampil di sini.
          </p>
        ) : (
          <div className="catalog-list">
            {releases.map((r) => (
              <button
                type="button"
                className="catalog-row catalog-row-button"
                key={r.id}
                disabled={busy}
                onClick={() => void open(r.id)}
              >
                <span>
                  <strong>{r.manifest.metadata.name}</strong>
                  <small>
                    v{r.manifest.metadata.version} ·{' '}
                    {r.manifest.metadata.capability}
                  </small>
                </span>
                <span className={`status status-${r.status}`}>
                  {integrationStatus[r.status]}
                </span>
                <ArrowUpRight />
              </button>
            ))}
          </div>
        )}
        <p className="catalog-help">
          Maksimal 200 rilis terbaru sesuai akses organisasi/akun.
        </p>
      </section>
    </>
  );
}
