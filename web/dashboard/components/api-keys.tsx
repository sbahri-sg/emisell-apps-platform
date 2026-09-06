'use client';

import { useCallback, useEffect, useId, useRef, useState } from 'react';
import Link from 'next/link';
import {
  KeyRound,
  Copy,
  RefreshCw,
  Plus,
  ShieldCheck,
  Search,
  Code2,
  ArrowRight,
} from 'lucide-react';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
  TableCaption,
} from '@/components/ui/table';
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
  AlertDialogAction,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import { Skeleton } from '@/components/ui/skeleton';
import { PortalAPI } from '@/lib/portal';

type APIKey = {
  id: string;
  name: string;
  access: 'platform_full';
  createdAt: string;
  status: 'active' | 'revoked';
};
type KeyList = {
  keys: APIKey[];
  limit: number;
};
const keyStatus = {
  active: 'Aktif',
  revoked: 'Dicabut',
};

export default function APIKeys({
  api,
  role,
  onDocumentation,
}: {
  api: PortalAPI;
  role: string;
  onDocumentation: () => void;
}) {
  const [data, setData] = useState<KeyList | null>(null);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);
  const [name, setName] = useState('');
  const [creating, setCreating] = useState(false);
  const [search, setSearch] = useState('');
  const [status, setStatus] = useState('all');
  const [issued, setIssued] = useState<{ key: APIKey; secret: string } | null>(
    null,
  );
  const [confirm, setConfirm] = useState<string | null>(null);
  const mounted = useRef(false);
  const formId = useId();
  const inFlight = useRef(false);
  const allowed = api.surface === 'admin' && role === 'administrator';
  const visibleKeys = (data?.keys ?? []).filter(
    (key) =>
      (status === 'all' || key.status === status) &&
      `${key.name} ${key.id}`.toLowerCase().includes(search.toLowerCase()),
  );
  const reload = useCallback(async () => {
    const next = await api.request<KeyList>('/platform-keys');
    if (mounted.current) setData(next);
  }, [api]);
  useEffect(() => {
    mounted.current = true;
    if (allowed)
      void reload()
        .catch((e: Error) => {
          if (mounted.current) setError(e.message);
        })
        .finally(() => {
          if (mounted.current) setLoading(false);
        });
    return () => {
      mounted.current = false;
    };
  }, [allowed, reload]);
  useEffect(() => {
    if (!issued) return;
    const timeout = window.setTimeout(() => setIssued(null), 5 * 60 * 1000);
    return () => window.clearTimeout(timeout);
  }, [issued]);
  async function perform(action: () => Promise<void>) {
    if (inFlight.current) return;
    inFlight.current = true;
    setBusy(true);
    setError('');
    setNotice('');
    try {
      await action();
    } catch (e) {
      if (mounted.current)
        setError(e instanceof Error ? e.message : 'Permintaan gagal.');
    } finally {
      inFlight.current = false;
      if (mounted.current) setBusy(false);
    }
  }
  if (!allowed)
    return (
      <section className="portal-panel">
        <h2>API Key</h2>
        <p>Hanya administrator platform dapat mengelola key integrasi Core.</p>
      </section>
    );
  return (
    <section className="api-keys">
      <div className="overview-heading">
        <div>
          <h1>API keys</h1>
          <p>Hubungkan backend Emisell ke Apps Platform dengan akses penuh.</p>
        </div>
        <Button
          disabled={busy || loading || Boolean(issued) || !data}
          aria-expanded={creating}
          aria-controls={`${formId}-create`}
          onClick={() => setCreating(!creating)}
        >
          <Plus /> Buat API key
        </Button>
      </div>
      <div className="key-security-banner">
        <span className="key-feature-icon">
          <ShieldCheck />
        </span>
        <div>
          <strong>Khusus komunikasi backend</strong>
          <p>
            API key ini memiliki full access. Simpan hanya di server, jangan di
            browser atau aplikasi developer.
          </p>
        </div>
      </div>
      {error && (
        <p role="alert" className="access-error">
          {error}
        </p>
      )}
      {notice && <output>{notice}</output>}
      {issued && (
        <div className="api-key-secret">
          <h2>Simpan secret sekarang</h2>
          <p>
            {issued.key.name} · <code>{issued.key.id}</code>. Secret hanya
            diberikan pada respons pertama, lalu disembunyikan setelah 5 menit
            atau saat meninggalkan halaman.
          </p>
          <Input
            aria-label="Secret API key baru"
            value={issued.secret}
            readOnly
            autoComplete="off"
          />
          <div>
            <Button
              variant="outline"
              onClick={() =>
                void navigator.clipboard
                  .writeText(issued.secret)
                  .then(() =>
                    setNotice(
                      'Secret disalin. Simpan di secret manager backend.',
                    ),
                  )
                  .catch(() =>
                    setError('Gagal menyalin. Salin secret secara manual.'),
                  )
              }
            >
              <Copy /> Salin secret
            </Button>
            <Button variant="outline" onClick={() => setIssued(null)}>
              Sudah disimpan · tutup
            </Button>
          </div>
        </div>
      )}
      {loading ? (
        <Skeleton className="h-40 w-full" />
      ) : (
        data && (
          <>
            {creating && (
              <form
                id={`${formId}-create`}
                className="portal-panel api-key-form"
                onSubmit={(e) => {
                  e.preventDefault();
                  void perform(async () => {
                    const result = await api.request<{
                      key: APIKey;
                      secret: string;
                      secretAvailable: boolean;
                    }>('/platform-keys', 'POST', { name }, true);
                    if (!mounted.current) return;
                    if (result.secretAvailable && result.secret) {
                      setIssued({ key: result.key, secret: result.secret });
                      setNotice(
                        'Key dibuat. Secret tidak dapat dilihat kembali.',
                      );
                    } else {
                      setNotice(
                        `Permintaan sebelumnya sudah diproses (${result.key.id}). Secret tidak dikirim ulang. Jika respons pertama hilang, cabut key ini lalu buat key baru.`,
                      );
                    }
                    setName('');
                    setCreating(false);
                    await reload();
                  });
                }}
              >
                <h2>
                  <KeyRound size={18} /> Generate API key
                </h2>
                <div className="api-key-fields">
                  <label htmlFor={`${formId}-name`}>
                    Nama koneksi
                    <Input
                      id={`${formId}-name`}
                      required
                      maxLength={80}
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                      placeholder="Emisell Core staging"
                      disabled={busy || Boolean(issued)}
                    />
                  </label>
                </div>
                <p className="access-meta">
                  Full access otomatis. Tidak perlu memilih merchant, masa
                  berlaku, atau izin layanan. Secret hanya ditampilkan sekali.
                </p>
                <Button type="submit" disabled={busy || Boolean(issued)}>
                  <KeyRound /> {busy ? 'Memproses…' : 'Generate API key'}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  disabled={busy}
                  onClick={() => setCreating(false)}
                >
                  Batal
                </Button>
              </form>
            )}
            <div className="key-table-card">
              <div className="key-table-heading">
                <h2>
                  Kunci akses backend <span>{data.keys.length} kunci</span>
                </h2>
                <Button
                  variant="ghost"
                  disabled={busy}
                  aria-label="Muat ulang kunci"
                  onClick={() => void perform(reload)}
                >
                  <RefreshCw size={16} />
                </Button>
              </div>
              <div className="key-table-toolbar">
                <div className="key-search">
                  <Search size={18} />
                  <Input
                    aria-label="Cari nama kunci"
                    placeholder="Cari nama kunci…"
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                  />
                </div>
                <Tabs value={status} onValueChange={setStatus}>
                  <TabsList variant="line">
                    <TabsTrigger value="all">Semua</TabsTrigger>
                    <TabsTrigger value="active">Aktif</TabsTrigger>
                    <TabsTrigger value="revoked">Dicabut</TabsTrigger>
                  </TabsList>
                </Tabs>
              </div>
              <Table>
                <TableCaption>
                  Maksimal {data.limit} key platform terbaru. Key merchant lama
                  tidak otomatis menjadi full access dan tetap dapat dikelola
                  melalui API legacy.
                </TableCaption>
                <TableHeader>
                  <TableRow>
                    <TableHead>Nama</TableHead>
                    <TableHead>API key</TableHead>
                    <TableHead>Akses</TableHead>
                    <TableHead>Status key</TableHead>
                    <TableHead>Tindakan</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {visibleKeys.map((key) => (
                    <TableRow key={key.id}>
                      <TableCell>
                        <strong>{key.name}</strong>
                        <small>
                          Dibuat{' '}
                          {new Date(key.createdAt).toLocaleDateString('id-ID', {
                            day: 'numeric',
                            month: 'short',
                            year: 'numeric',
                          })}
                        </small>
                      </TableCell>
                      <TableCell>
                        <span
                          className="key-masked"
                          aria-label="Secret disembunyikan"
                        >
                          •••• •••• ••••
                        </span>
                        <small className="key-public-id">ID: {key.id}</small>
                      </TableCell>
                      <TableCell>
                        <span className="key-access-badge">Full access</span>
                        <small>Berlaku sampai dicabut</small>
                      </TableCell>
                      <TableCell>
                        <span className={`key-status-badge ${key.status}`}>
                          {keyStatus[key.status]}
                        </span>
                      </TableCell>
                      <TableCell>
                        <AlertDialog
                          open={confirm === key.id}
                          onOpenChange={(open) => {
                            if (!busy) setConfirm(open ? key.id : null);
                          }}
                        >
                          <AlertDialogTrigger
                            render={
                              <Button
                                variant="outline"
                                disabled={busy || key.status === 'revoked'}
                              />
                            }
                          >
                            Cabut key
                          </AlertDialogTrigger>
                          <AlertDialogContent>
                            <AlertDialogHeader>
                              <AlertDialogTitle>
                                Cabut {key.name}?
                              </AlertDialogTitle>
                              <AlertDialogDescription>
                                Koneksi yang memakai key ini akan ditolak untuk
                                permintaan berikutnya. Tindakan tidak dapat
                                dibatalkan; buat key baru jika masih diperlukan.
                              </AlertDialogDescription>
                            </AlertDialogHeader>
                            <AlertDialogFooter>
                              <AlertDialogCancel disabled={busy}>
                                Batal
                              </AlertDialogCancel>
                              <AlertDialogAction
                                disabled={busy}
                                onClick={() =>
                                  void perform(async () => {
                                    await api.request(
                                      `/platform-keys/${key.id}/revoke`,
                                      'POST',
                                      {},
                                    );
                                    if (!mounted.current) return;
                                    if (issued?.key.id === key.id)
                                      setIssued(null);
                                    setConfirm(null);
                                    setNotice('Key telah dicabut.');
                                    await reload();
                                  })
                                }
                              >
                                Ya, cabut key
                              </AlertDialogAction>
                            </AlertDialogFooter>
                          </AlertDialogContent>
                        </AlertDialog>
                      </TableCell>
                    </TableRow>
                  ))}
                  {visibleKeys.length === 0 && (
                    <TableRow>
                      <TableCell colSpan={5}>
                        {data.keys.length === 0
                          ? 'Belum ada API key. Buat kunci untuk menghubungkan backend Emisell.'
                          : 'Tidak ada kunci yang sesuai dengan pencarian atau status ini.'}
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            </div>
          </>
        )
      )}
      <div className="key-guidance-grid">
        <article>
          <span className="key-feature-icon">
            <KeyRound />
          </span>
          <div>
            <h2>Ditampilkan sekali</h2>
            <p>
              Kunci lengkap hanya ditampilkan setelah dibuat. Simpan di secret
              manager backend.
            </p>
          </div>
        </article>
        <article>
          <span className="key-feature-icon">
            <RefreshCw />
          </span>
          <div>
            <h2>Rotasi dengan aman</h2>
            <p>Buat kunci baru, perbarui backend, lalu cabut kunci lama.</p>
          </div>
        </article>
      </div>
      <div className="key-docs-banner">
        <span className="key-feature-icon">
          <Code2 />
        </span>
        <div>
          <h2>Butuh panduan integrasi?</h2>
          <p>Pelajari autentikasi dan contoh request backend.</p>
        </div>
        <Link
          href="?view=api-docs&api_group=core"
          onClick={(event) => {
            if (
              !event.metaKey &&
              !event.ctrlKey &&
              !event.shiftKey &&
              !event.altKey
            ) {
              event.preventDefault();
              onDocumentation();
            }
          }}
        >
          Buka dokumentasi API <ArrowRight size={18} />
        </Link>
      </div>
      <details className="portal-panel">
        <summary>Cara menguji koneksi</summary>
        <p>
          Key tidak terikat satu merchant dan berlaku sampai dicabut. Key ini
          tidak menggantikan consent aplikasi atau mengaktifkan gateway resource
          yang masih berstatus Plan.
        </p>
        <p>
          Kirim dari backend ke{' '}
          <code>
            http://127.0.0.1:8088/emisell.integration.v1.ConnectionService/Check
          </code>{' '}
          memakai POST, Content-Type application/json, Connect-Protocol-Version:
          1, Authorization: Bearer YOUR_SECRET, dan body <code>{'{}'}</code>.
          Jangan kirim cookie atau Origin browser.
        </p>
        <p>
          Respons mengonfirmasi identitas key dan platformFullAccess: true;
          bukan bukti dukungan scope resource. Untuk operasi data toko, backend
          Emisell mengirim merchantId per request setelah memeriksa otoritas
          penggunanya. Endpoint Check ini tidak melakukan transaksi atau
          instalasi. Lingkungan saat ini localhost; deployment nyata wajib HTTPS
          dan pengamanan internal.
        </p>
      </details>
    </section>
  );
}
