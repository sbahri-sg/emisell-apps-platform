'use client';

import { useEffect, useState } from 'react';
import {
  ArrowLeft,
  ArrowUpRight,
  Copy,
  Download,
  KeyRound,
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  AlertDialog,
  AlertDialogTrigger,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
  AlertDialogAction,
} from '@/components/ui/alert-dialog';
import type { PortalAPI, Session } from '@/lib/portal';
import type { IntegrationRelease } from '@/lib/integration-releases';
import {
  uiReleaseOptions,
  type UIReleaseOption,
} from '@/lib/ui-release-options';
import AppLaunch from './app-launch';
import {
  publicDistributionAllowed,
  paymentBoundaryMessage,
} from '@/lib/distribution';
import {
  clientActions,
  clientActionLabels,
  proofResult,
  type ClientView,
  type ClientDetail,
  type ClientAction,
} from '@/lib/app-clients';

export default function AppClients({
  api,
  session,
  busy,
  perform,
}: {
  api: PortalAPI;
  session: Session;
  busy: boolean;
  perform: (fn: () => Promise<void>, notice?: string) => Promise<void>;
}) {
  const [clients, setClients] = useState<ClientView[] | null>(null);
  const [releases, setReleases] = useState<IntegrationRelease[]>([]);
  const [uiReleases, setUIReleases] = useState<UIReleaseOption[]>([]);
  const [detail, setDetail] = useState<ClientDetail | null>(null);
  const [releaseId, setReleaseId] = useState('');
  const [reason, setReason] = useState('');
  const [error, setError] = useState('');
  const [secret, setSecret] = useState('');
  const developer = session.user.surface === 'developer';
  useEffect(() => {
    let active = true;
    Promise.all([
      api.request<{ clients: ClientView[] }>('/app-clients'),
      api.request<{ releases: IntegrationRelease[] }>('/integration-releases'),
      uiReleaseOptions(api),
    ])
      .then(([c, r, ui]) => {
        if (active) {
          setClients(c.clients);
          setReleases(r.releases);
          setUIReleases(ui);
        }
      })
      .catch((e: Error) => {
        if (active) setError(e.message);
      });
    return () => {
      active = false;
    };
  }, [api]);
  useEffect(() => {
    if (!secret) return;
    const timer = window.setTimeout(() => setSecret(''), 5 * 60 * 1000);
    return () => window.clearTimeout(timer);
  }, [secret]);
  const reload = async () => {
    const [c, r, ui] = await Promise.all([
      api.request<{ clients: ClientView[] }>('/app-clients'),
      api.request<{ releases: IntegrationRelease[] }>('/integration-releases'),
      uiReleaseOptions(api),
    ]);
    setClients(c.clients);
    setReleases(r.releases);
    setUIReleases(ui);
    setError('');
  };
  const open = (id: string) =>
    perform(async () => {
      setSecret('');
      setReason('');
      setDetail(await api.request(`/app-clients/${id}`));
    });
  const act = (action: ClientAction) =>
    perform(async () => {
      if (!detail) return;
      setSecret('');
      const r = await api.request<{
        view: ClientView;
        secret: string;
        secretAvailable: boolean;
      }>(
        `/app-clients/${detail.view.client.id}/actions`,
        'POST',
        { action, revision: detail.view.client.revision, reason },
        true,
      );
      // Capture one-time secret before any follow-up request can fail.
      setSecret(r.secretAvailable ? r.secret : '');
      setDetail({ ...detail, view: r.view });
      setReason('');
      await reload();
      setDetail(await api.request(`/app-clients/${r.view.client.id}`));
    }, 'Tindakan tercatat. Periksa status terbaru; client tidak memberi akses tenant atau mengaktifkan OAuth.');
  const eligible = releases.filter(
    (r) =>
      r.status === 'signed' &&
      publicDistributionAllowed(r.manifest.metadata.capability) &&
      !clients?.some((c) => c.client.binding.releaseId === r.id),
  );
  const status = (v: ClientView) =>
    v.client.status === 'revoked'
      ? 'Dicabut'
      : v.clientReady
        ? 'Identitas client siap'
        : v.client.status === 'verified'
          ? 'Bukti tersimpan — periksa kesiapan'
          : 'Menunggu bukti endpoint';
  if (detail) {
    const v = detail.view,
      c = v.client;
    const sourceRelease = releases.find((r) => r.id === c.binding.releaseId);
    const distributionAllowed =
      (sourceRelease &&
        publicDistributionAllowed(
          sourceRelease.manifest.metadata.capability,
        )) ||
      uiReleases.some(
        (r) => r.id === c.binding.releaseId && r.status === 'signed',
      );
    const actions = clientActions(
      session.user.surface,
      session.user.role,
      v,
    ).filter((action) => action === 'revoke' || distributionAllowed);
    return (
      <>
        <div className="catalog-actions">
          <Button
            variant="ghost"
            disabled={busy}
            onClick={() => {
              setDetail(null);
              setSecret('');
            }}
          >
            <ArrowLeft />
            Kembali ke app clients
          </Button>
          <Button
            variant="outline"
            disabled={busy}
            onClick={() => void open(c.id)}
          >
            <RefreshCw />
            Periksa status
          </Button>
        </div>
        <div className="page-heading">
          <div>
            <p className="eyebrow">APP CLIENT · PERSIAPAN OAUTH</p>
            <h1>{c.binding.name}</h1>
            <p>
              v{c.binding.version} · {status(v)}
            </p>
          </div>
        </div>
        {sourceRelease && !distributionAllowed && (
          <p className="field-help">{paymentBoundaryMessage}</p>
        )}
        <AppLaunch
          key={c.id}
          api={api}
          session={session}
          clientId={c.id}
          ready={v.clientReady}
        />
        {secret && (
          <section className="portal-panel" aria-label="Client secret baru">
            <h2>Client secret — tampil sekali</h2>
            <p>
              Simpan di secret manager backend aplikasi. Bukan API key Core,
              bukan access token. Tampilan ditutup setelah 5 menit atau
              meninggalkan halaman.
            </p>
            <label htmlFor="client-secret">
              Secret baru
              <Input
                id="client-secret"
                value={secret}
                readOnly
                autoComplete="off"
                spellCheck={false}
              />
            </label>
            <div className="catalog-actions">
              <Button
                onClick={() =>
                  void perform(
                    () => navigator.clipboard.writeText(secret),
                    'Secret disalin. Jangan bagikan ke frontend/log.',
                  )
                }
              >
                <Copy />
                Salin secret
              </Button>
              <Button variant="outline" onClick={() => setSecret('')}>
                Sudah disimpan
              </Button>
            </div>
          </section>
        )}
        <div className="catalog-columns">
          <div>
            <section className="portal-panel">
              <h2>Identitas & binding release</h2>
              <dl className="catalog-facts">
                <dt>Client ID</dt>
                <dd>
                  <code>{c.id}</code>
                </dd>
                <dt>Release</dt>
                <dd>
                  <code>{c.binding.releaseId}</code>
                </dd>
                <dt>SHA-256 release</dt>
                <dd>
                  <code>{c.binding.digest}</code>
                </dd>
                <dt>Callback OAuth</dt>
                <dd>
                  <code>{c.binding.redirectUri}</code>
                </dd>
                <dt>Hasil pemeriksaan</dt>
                <dd>{proofResult[c.lastResult] ?? c.lastResult}</dd>
                <dt>Bukti valid sampai</dt>
                <dd>
                  {c.verifiedUntil
                    ? new Date(c.verifiedUntil).toLocaleString('id-ID')
                    : 'Belum terverifikasi'}
                </dd>
                <dt>Versi secret</dt>
                <dd>{c.secretVersion}</dd>
              </dl>
              <h3>Gate yang masih berlaku</h3>
              <ul className="tooling-checks">
                {v.blockers.map((b) => (
                  <li key={b}>{b}</li>
                ))}
              </ul>
            </section>
            <section className="portal-panel">
              <h2>Buktikan kendali endpoint</h2>
              <p>
                Layani JSON berikut dengan HTTP 200 dan Content-Type
                application/json pada URL ini. Platform mengambil file tanpa
                mengirim nonce, secret, cookie atau token. Tidak menerima
                redirect.
              </p>
              <pre className="catalog-json">
                <code>{v.proofUrl}</code>
              </pre>
              <pre className="catalog-json">
                <code>{JSON.stringify(v.expected, null, 2)}</code>
              </pre>
              <p>
                Challenge kedaluwarsa:{' '}
                {new Date(c.challengeExpiresAt).toLocaleString('id-ID')}. Bukti
                hanya memverifikasi kendali origin; bukan uji callback OAuth
                atau fungsi bisnis. Koneksi pemeriksaan v1 menggunakan HTTPS
                port 443 dan IPv4 publik.
              </p>
              <Button
                variant="outline"
                onClick={() => {
                  const url = URL.createObjectURL(
                    new Blob([JSON.stringify(v.expected, null, 2)], {
                      type: 'application/json',
                    }),
                  );
                  const a = document.createElement('a');
                  a.href = url;
                  a.download = 'emisell-endpoint-proof.json';
                  a.click();
                  setTimeout(() => URL.revokeObjectURL(url), 1000);
                }}
              >
                <Download />
                Unduh JSON bukti
              </Button>
            </section>
          </div>
          <section className="portal-panel">
            <h2>Kelola app-client</h2>
            {actions.length > 0 ? (
              <>
                <label htmlFor="client-reason">
                  Alasan tindakan — tanpa secret
                  <Textarea
                    id="client-reason"
                    value={reason}
                    maxLength={2000}
                    disabled={busy}
                    onChange={(e) => setReason(e.target.value)}
                  />
                </label>
                <div className="catalog-actions">
                  {actions.map((action) =>
                    action === 'verify' ? (
                      <Button
                        key={action}
                        disabled={busy || !reason.trim()}
                        onClick={() => void act(action)}
                      >
                        <ShieldCheck />
                        Verifikasi endpoint
                      </Button>
                    ) : (
                      <AlertDialog key={action}>
                        <AlertDialogTrigger
                          render={
                            <Button
                              variant="outline"
                              disabled={busy || !reason.trim()}
                            />
                          }
                        >
                          {clientActionLabels[action]}
                        </AlertDialogTrigger>
                        <AlertDialogContent>
                          <AlertDialogHeader>
                            <AlertDialogTitle>
                              {clientActionLabels[action]}?
                            </AlertDialogTitle>
                            <AlertDialogDescription>
                              {action === 'revoke'
                                ? 'Client dicabut permanen untuk release ini; credential berhenti berlaku.'
                                : action === 'challenge'
                                  ? 'Challenge lama dan secret lama akan dibatalkan. Layani bukti baru, verifikasi, lalu terbitkan secret pengganti.'
                                  : 'Secret lama langsung berhenti berlaku. Secret baru hanya ditampilkan sekali; pindahkan ke backend aplikasi.'}
                            </AlertDialogDescription>
                          </AlertDialogHeader>
                          <AlertDialogFooter>
                            <AlertDialogCancel>Batal</AlertDialogCancel>
                            <AlertDialogAction onClick={() => void act(action)}>
                              Lanjutkan
                            </AlertDialogAction>
                          </AlertDialogFooter>
                        </AlertDialogContent>
                      </AlertDialog>
                    ),
                  )}
                </div>
                <p className="catalog-help">
                  Verifikasi dibatasi sekali per menit. Kegagalan atau proses
                  terputus tidak dianggap berhasil; muat ulang lalu coba lagi.
                  Penerbitan secret memerlukan bukti valid dan release tetap
                  signed.
                </p>
              </>
            ) : (
              <p>
                {c.status === 'revoked'
                  ? 'Client sudah dicabut. Gunakan release versi baru untuk registrasi pengganti.'
                  : 'Akun ini hanya dapat membaca status client.'}
              </p>
            )}
            <h3>Riwayat audit</h3>
            <ol className="catalog-history">
              {detail.history.map((h) => (
                <li key={h.id}>
                  <strong>{h.action}</strong>
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
          <p className="eyebrow">DEVELOPER INTEGRATION</p>
          <h1>App clients</h1>
          <p>
            Identitas aplikasi, bukti kendali endpoint, dan pengelolaan
            credential.
          </p>
        </div>
        <Button
          variant="outline"
          disabled={busy}
          onClick={() => void perform(reload)}
        >
          <RefreshCw />
          Muat ulang
        </Button>
      </div>
      <div className="portal-callout">
        <KeyRound />
        <span>
          Terpisah dari API key full-access Emisell Core. Registrasi client dan
          proof belum mengaktifkan OAuth token exchange, runtime umum, grant
          resource atau install.
        </span>
      </div>
      {error && <p role="alert">{error}</p>}
      {developer && clients !== null && (
        <section className="portal-panel">
          <h2>Daftarkan client aplikasi</h2>
          <p>
            Satu client per rilis signed. URL dan konfigurasi
            mengikuti snapshot immutable.
          </p>
          {eligible.length ||
          uiReleases.some(
            (r) =>
              r.status === 'signed' &&
              !clients.some((c) => c.client.binding.releaseId === r.id),
          ) ? (
            <form
              className="catalog-publish"
              onSubmit={(e) => {
                e.preventDefault();
                void perform(async () => {
                  const r = await api.request<{ view: ClientView }>(
                    '/app-clients',
                    'POST',
                    { releaseId },
                    true,
                  );
                  await reload();
                  setReleaseId('');
                  setDetail(
                    await api.request(`/app-clients/${r.view.client.id}`),
                  );
                });
              }}
            >
              <label htmlFor="client-release">
                Rilis konfigurasi
                <NativeSelect
                  id="client-release"
                  required
                  value={releaseId}
                  disabled={busy}
                  onChange={(e) => setReleaseId(e.target.value)}
                >
                  <NativeSelectOption value="">
                    Pilih release signed…
                  </NativeSelectOption>
                  {eligible.map((r) => (
                    <NativeSelectOption key={r.id} value={r.id}>
                      {r.manifest.metadata.name} · v
                      {r.manifest.metadata.version}
                    </NativeSelectOption>
                  ))}
                  {uiReleases
                    .filter(
                      (r) =>
                        r.status === 'signed' &&
                        !clients.some(
                          (c) => c.client.binding.releaseId === r.id,
                        ),
                    )
                    .map((r) => (
                      <NativeSelectOption key={r.id} value={r.id}>
                        {r.manifest.name} · v{r.manifest.version} · UI
                      </NativeSelectOption>
                    ))}
                </NativeSelect>
              </label>
              <Button type="submit" disabled={busy || !releaseId}>
                <KeyRound />
                Daftarkan app-client
              </Button>
            </form>
          ) : (
            <p>
              Belum ada rilis yang dapat didaftarkan. Selesaikan review dan
              signing pada menu Rilis integrasi atau Aplikasi dengan UI.
            </p>
          )}
        </section>
      )}
      <section className="portal-panel">
        <h2>Client aplikasi</h2>
        {clients === null ? (
          <p>
            {error
              ? 'Data belum tersedia. Coba muat ulang.'
              : 'Memuat app clients…'}
          </p>
        ) : clients.length === 0 ? (
          <p>
            Belum ada client. Client aplikasi akan tampil setelah developer
            mendaftarkan rilis signed.
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Aplikasi</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Bukti endpoint</TableHead>
                <TableHead>
                  <span className="sr-only">Detail</span>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {clients.map((v) => (
                <TableRow key={v.client.id}>
                  <TableCell>
                    <strong>{v.client.binding.name}</strong>
                    <div>v{v.client.binding.version}</div>
                  </TableCell>
                  <TableCell>{status(v)}</TableCell>
                  <TableCell>
                    {proofResult[v.client.lastResult] ?? v.client.lastResult}
                  </TableCell>
                  <TableCell>
                    <Button
                      variant="ghost"
                      disabled={busy}
                      onClick={() => void open(v.client.id)}
                      aria-label={`Detail ${v.client.binding.name}`}
                    >
                      <ArrowUpRight />
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
        <p className="catalog-help">
          Maksimal 200 client terbaru sesuai akses akun.
        </p>
      </section>
    </>
  );
}
