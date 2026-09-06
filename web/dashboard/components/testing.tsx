'use client';

import { useEffect, useRef, useState } from 'react';
import { FlaskConical, RefreshCw, ArrowLeft } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import {
  NativeSelect,
  NativeSelectOption,
} from '@/components/ui/native-select';
import type { PortalAPI, Session } from '@/lib/portal';
import type { IntegrationRelease } from '@/lib/integration-releases';
import {
  uiReleaseOptions,
  type UIReleaseOption,
} from '@/lib/ui-release-options';
import type { ManagedShippingRelease } from '@/lib/managed-shipping';
import {
  publicDistributionAllowed,
  paymentBoundaryMessage,
} from '@/lib/distribution';
import {
  assignmentActions,
  assignmentStatus,
  testingBlockers,
  type AssignmentDetail,
  type AssignmentPage,
  type AssignmentStatus,
} from '@/lib/testing';

export default function Testing({
  api,
  session,
  busy,
  perform,
}: {
  api: PortalAPI;
  session: Session;
  busy: boolean;
  perform: (action: () => Promise<void>, message?: string) => Promise<void>;
}) {
  const developer = session.user.surface === 'developer';
  const [page, setPage] = useState<AssignmentPage | null>(null);
  const [after, setAfter] = useState('');
  const [attempt, setAttempt] = useState(0);
  const [error, setError] = useState('');
  const [releases, setReleases] = useState<IntegrationRelease[]>([]);
  const [uiReleases, setUIReleases] = useState<UIReleaseOption[]>([]);
  const [managedReleases, setManagedReleases] = useState<
    ManagedShippingRelease[]
  >([]);
  const [releaseId, setReleaseId] = useState('');
  const [merchantId, setMerchantId] = useState('');
  const [reason, setReason] = useState('');
  const [decisionReason, setDecisionReason] = useState('');
  const [detail, setDetail] = useState<AssignmentDetail | null>(null);
  const pending = useRef(false);
  useEffect(() => {
    let current = true;
    Promise.all([
      api.request<AssignmentPage>(
        '/test-assignments',
        'GET',
        undefined,
        false,
        { afterId: after },
      ),
      developer
        ? api.request<{ releases: IntegrationRelease[] }>(
            '/integration-releases',
          )
        : Promise.resolve({ releases: [] }),
      developer
        ? api.request<{ releases: ManagedShippingRelease[] }>(
            '/managed-shipping-releases',
          )
        : Promise.resolve({ releases: [] }),
    ])
      .then(async ([p, r, managed]) => {
        const ui = developer ? await uiReleaseOptions(api) : [];
        if (current) {
          setUIReleases(ui.filter((r) => r.status === 'signed'));
          setPage(p);
          setManagedReleases(
            managed.releases.filter(
              (v) =>
                v.status === 'signed' &&
                v.manifest.capability === 'shipping/v1',
            ),
          );
          setReleases(
            r.releases.filter(
              (v) =>
                v.status === 'signed' &&
                publicDistributionAllowed(v.manifest.metadata.capability),
            ),
          );
        }
      })
      .catch((e: Error) => {
        if (current) setError(e.message);
      });
    return () => {
      current = false;
    };
  }, [api, developer, after, attempt]);
  const refresh = () => {
    setPage(null);
    setError('');
    setDetail(null);
    setAfter('');
    setAttempt((n) => n + 1);
  };
  const run = async (action: () => Promise<void>, message: string) => {
    if (pending.current || busy) return;
    pending.current = true;
    try {
      await perform(action, message);
    } finally {
      pending.current = false;
    }
  };
  const open = (id: string) =>
    run(async () => {
      setDetail(null);
      setDecisionReason('');
      setDetail(
        await api.request<AssignmentDetail>(
          `/test-assignments/${encodeURIComponent(id)}`,
        ),
      );
    }, '');
  const decide = (status: AssignmentStatus) =>
    run(async () => {
      if (!detail) return;
      const a = detail.assignment;
      // Drop stale action controls even after an uncertain response; re-open to retry from current revision.
      setDetail(null);
      await api.request(
        `/test-assignments/${encodeURIComponent(a.id)}/status`,
        'POST',
        { status, revision: a.revision, reason: decisionReason },
        true,
      );
      refresh();
    }, 'Keputusan pengujian disimpan. Tidak ada instalasi atau grant yang dibuat.');
  return (
    <div className="space-y-5">
      <header className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="flex items-center gap-2 text-xl font-semibold">
            <FlaskConical className="size-5" />
            Testing
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Distribusi aplikasi uji · persetujuan dan kesiapan instalasi
            dipisahkan.
          </p>
        </div>
        <Button variant="outline" disabled={busy} onClick={refresh}>
          <RefreshCw />
          Perbarui
        </Button>
      </header>
      <section className="portal-panel">
        <h2>Izin pengujian bukan izin akses data</h2>
        <p>
          Developer mengajukan versi ke merchant tertentu, lalu Administrator
          menyetujui distribusinya. Merchant melihat assignment yang disetujui.
          Status kesiapan menjelaskan langkah yang masih diperlukan sebelum
          aplikasi dapat di-install. Persetujuan distribusi tidak memberi akses
          toko.
        </p>
        <p className="catalog-help">
          Mencabut assignment menutup distribusi pengujian, bukan uninstall.
          Versi atau merchant tidak dapat diubah setelah pengajuan.
        </p>
      </section>
      {detail ? (
        <>
          <Button
            variant="ghost"
            onClick={() => setDetail(null)}
            disabled={busy}
          >
            <ArrowLeft />
            Kembali ke daftar
          </Button>
          <section className="portal-panel space-y-3">
            <h2>
              {detail.assignment.app.appName} · v{detail.assignment.app.version}
            </h2>
            {!publicDistributionAllowed(detail.assignment.app.capability) && (
              <p className="field-help">{paymentBoundaryMessage}</p>
            )}
            <dl className="grid gap-2 text-sm">
              <div>
                Merchant ID:{' '}
                <code className="break-all">
                  {detail.assignment.merchantId}
                </code>
              </div>
              <div>
                Release:{' '}
                <code className="break-all">{detail.assignment.releaseId}</code>
              </div>
              <div>
                Jenis:{' '}
                {detail.assignment.releaseKind === 'managed_shipping'
                  ? 'Provider terkelola · API-Kurir'
                  : detail.assignment.releaseKind === 'ui' ? 'Aplikasi dengan UI'
                  : 'Aplikasi Integrasi'}
              </div>
              <div>
                Status: {assignmentStatus[detail.assignment.status]} · revisi{' '}
                {detail.assignment.revision}
              </div>
            </dl>
            <h3>
              Kesiapan instalasi:{' '}
              {detail.assignment.app.readiness.installable
                ? 'Siap diuji lokal'
                : 'belum tersedia'}
            </h3>
            <p>
              Konfigurasi:{' '}
              {detail.assignment.app.readiness.configurationReady
                ? 'terverifikasi'
                : 'perlu ditinjau'}
              . Scope wajib:{' '}
              {detail.assignment.app.readiness.requiredScopesReady
                ? 'lulus pemeriksaan konfigurasi'
                : 'belum siap'}
              .
            </p>
            <ul className="list-disc space-y-1 pl-5 text-sm">
              {detail.assignment.app.readiness.blockers.map((b) => (
                <li key={b}>
                  {testingBlockers[b] || 'Pemeriksaan tambahan diperlukan.'}
                </li>
              ))}
            </ul>
            {assignmentActions(
              session.user.surface,
              session.user.role,
              detail.assignment.status,
            ).length > 0 && (
              <div className="space-y-3 border-t pt-3">
                <label className="block text-sm" htmlFor="testing-decision">
                  Alasan keputusan
                </label>
                <Textarea
                  id="testing-decision"
                  value={decisionReason}
                  onChange={(e) => setDecisionReason(e.target.value)}
                  maxLength={2000}
                  disabled={busy}
                />
                <div className="flex flex-wrap gap-2">
                  {assignmentActions(
                    session.user.surface,
                    session.user.role,
                    detail.assignment.status,
                  ).map((status) => (
                    <Button
                      key={status}
                      variant={status === 'approved' ? 'default' : 'outline'}
                      disabled={
                        busy ||
                        !decisionReason.trim() ||
                        (status === 'approved' &&
                          !publicDistributionAllowed(
                            detail.assignment.app.capability,
                          ))
                      }
                      onClick={() => void decide(status)}
                    >
                      {status === 'approved'
                        ? 'Setujui pengujian'
                        : status === 'rejected'
                          ? 'Tolak pengajuan'
                          : 'Cabut assignment'}
                    </Button>
                  ))}
                </div>
              </div>
            )}
          </section>
          <section className="portal-panel">
            <h2>Riwayat keputusan</h2>
            <ol className="space-y-4">
              {detail.history.map((h) => (
                <li key={h.id} className="border-b pb-3 text-sm">
                  <strong>
                    {assignmentStatus[h.action as AssignmentStatus] || h.action}
                  </strong>
                  <p className="whitespace-pre-wrap break-words">{h.reason}</p>
                  <p className="text-muted-foreground">
                    {new Date(h.occurredAt).toLocaleString('id-ID')} ·{' '}
                    {h.actorId}
                  </p>
                </li>
              ))}
            </ol>
          </section>
        </>
      ) : (
        <>
          {developer && (
            <form
              className="portal-panel space-y-3"
              onSubmit={(e) => {
                e.preventDefault();
                void run(async () => {
                  await api.request(
                    '/test-assignments',
                    'POST',
                    {
                      releaseId,
                      ...(uiReleases.some((r) => r.id === releaseId)
                        ? { releaseKind: 'ui' }
                        : {}),
                      ...(managedReleases.some((r) => r.id === releaseId)
                        ? { releaseKind: 'managed_shipping' }
                        : {}),
                      merchantId: merchantId.trim(),
                      reason: reason.trim(),
                    },
                    true,
                  );
                  setMerchantId('');
                  setReason('');
                  refresh();
                }, 'Pengajuan tersimpan. Menunggu persetujuan Administrator.');
              }}
            >
              <h2>Ajukan aplikasi uji</h2>
              <label className="block text-sm" htmlFor="testing-release">
                Aplikasi dan versi signed
              </label>
              <NativeSelect
                id="testing-release"
                value={releaseId}
                onChange={(e) => setReleaseId(e.target.value)}
                required
                disabled={busy || !page}
              >
                <NativeSelectOption value="">Pilih release</NativeSelectOption>
                {releases.map((r) => (
                  <NativeSelectOption key={r.id} value={r.id}>
                    {r.manifest.metadata.name} · v{r.manifest.metadata.version}
                  </NativeSelectOption>
                ))}
                {managedReleases.map((r) => (
                  <NativeSelectOption key={r.id} value={r.id}>
                    {r.manifest.name} · v{r.manifest.version} · Provider
                    terkelola
                  </NativeSelectOption>
                ))}
                {uiReleases.map((r) => (
                  <NativeSelectOption key={r.id} value={r.id}>
                    {r.manifest.name} · v{r.manifest.version} · UI{' '}
                    {r.manifest.mode}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
              {page &&
                releases.length === 0 &&
                uiReleases.length === 0 &&
                managedReleases.length === 0 && (
                  <p className="text-sm text-muted-foreground">
                    Belum ada release signed. Selesaikan review dan
                    penandatanganan rilis integrasi atau provider terkelola
                    terlebih dahulu.
                  </p>
                )}
              <label className="block text-sm" htmlFor="testing-merchant">
                Merchant ID tujuan
              </label>
              <Input
                id="testing-merchant"
                value={merchantId}
                onChange={(e) => setMerchantId(e.target.value)}
                required
                pattern="[A-Za-z0-9_-]{1,100}"
                maxLength={100}
                disabled={busy}
                autoComplete="off"
              />
              <p className="text-sm text-muted-foreground">
                Gunakan ID yang diberikan pemilik toko. Portal tidak menyediakan
                pencarian daftar merchant.
              </p>
              <label className="block text-sm" htmlFor="testing-reason">
                Tujuan pengujian
              </label>
              <Textarea
                id="testing-reason"
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                required
                maxLength={2000}
                disabled={busy}
              />
              <Button
                type="submit"
                disabled={
                  busy ||
                  !page ||
                  !releaseId ||
                  !merchantId.trim() ||
                  !reason.trim()
                }
              >
                Ajukan pengujian
              </Button>
            </form>
          )}
          <section className="portal-panel">
            <h2>Assignment pengujian</h2>
            {error ? (
              <p role="alert">{error}</p>
            ) : !page ? (
              <output>Memuat assignment…</output>
            ) : page.assignments.length === 0 ? (
              <div className="py-6 text-center">
                <FlaskConical className="mx-auto mb-3 size-7 text-muted-foreground" />
                <h3>Belum ada assignment pengujian</h3>
                <p className="text-sm text-muted-foreground">
                  {developer
                    ? 'Ajukan aplikasi signed ke merchant yang akan menguji.'
                    : 'Pengajuan developer akan muncul di sini untuk diperiksa.'}
                </p>
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-sm">
                  <caption className="sr-only">Assignment aplikasi uji</caption>
                  <thead>
                    <tr className="border-b text-muted-foreground">
                      {[
                        'Aplikasi / versi',
                        'Merchant ID',
                        'Persetujuan',
                        'Instalasi',
                        'Tindakan',
                      ].map((h) => (
                        <th className="p-3 font-medium" key={h}>
                          {h}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {page.assignments.map((a) => (
                      <tr key={a.id} className="border-b">
                        <td className="min-w-44 p-3">
                          <strong>{a.app.appName}</strong>
                          <p>v{a.app.version}</p>
                        </td>
                        <td className="max-w-56 break-all p-3 font-mono text-xs">
                          {a.merchantId}
                        </td>
                        <td className="p-3">{assignmentStatus[a.status]}</td>
                        <td className="p-3 text-muted-foreground">
                          {a.app.readiness.installable
                            ? 'Siap diuji lokal'
                            : 'Belum tersedia'}
                        </td>
                        <td className="p-3">
                          <Button
                            variant="outline"
                            size="sm"
                            disabled={busy}
                            onClick={() => void open(a.id)}
                          >
                            Detail
                            <span className="sr-only"> {a.app.appName}</span>
                          </Button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            {page && (after || page.nextAfterId) && (
              <div className="mt-4 flex flex-wrap justify-between gap-2">
                <Button variant="ghost" onClick={refresh} disabled={busy}>
                  Kembali ke awal
                </Button>
                {page.nextAfterId && (
                  <Button
                    variant="outline"
                    disabled={busy}
                    onClick={() => {
                      setError('');
                      setPage(null);
                      setAfter(page.nextAfterId);
                    }}
                  >
                    Halaman berikutnya
                  </Button>
                )}
              </div>
            )}
          </section>
        </>
      )}
    </div>
  );
}
