'use client';
import { useEffect, useState } from 'react';
import { Users, RefreshCw, Plus } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '@/components/ui/table';
import type { PortalAPI } from '@/lib/portal';
import { PortalError } from '@/lib/portal';
type Organization = { id: string; name: string; memberCount: number };
export default function AdminDevelopers({
  api,
  role,
}: {
  api: PortalAPI;
  role: string;
}) {
  const [rows, setRows] = useState<Organization[]>([]),
    [next, setNext] = useState(''),
    [query, setQuery] = useState(''),
    [selected, setSelected] = useState<Organization | null>(null),
    [error, setError] = useState(''),
    [busy, setBusy] = useState(true),
    [revision, setRevision] = useState(0);
  const [creating, setCreating] = useState(false);
  const [organizationName, setOrganizationName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [success, setSuccess] = useState('');
  const allowed = api.surface === 'admin' && role === 'administrator';
  useEffect(() => {
    let current = true;
    if (!allowed) return;
    api
      .request<{ organizations: Organization[]; nextAfterId: string }>(
        '/developers',
      )
      .then((d) => {
        if (current) {
          setRows(d.organizations);
          setNext(d.nextAfterId);
        }
      })
      .catch((e) => {
        if (current) setError(e.message);
      })
      .finally(() => {
        if (current) setBusy(false);
      });
    return () => {
      current = false;
    };
  }, [api, allowed, revision]);
  async function perform(fn: () => Promise<void>) {
    if (busy) return;
    setBusy(true);
    setError('');
    try {
      await fn();
    } catch (e) {
      setError(
        creating && e instanceof PortalError && e.status === 409
          ? 'Email sudah digunakan. Gunakan email lain; akun lama tidak diubah.'
          : e instanceof Error
            ? e.message
            : 'Permintaan gagal.',
      );
    } finally {
      setBusy(false);
    }
  }
  if (!allowed)
    return (
      <section className="portal-panel">
        <h1>Developer</h1>
        <p>Daftar organisasi hanya dapat dibaca administrator platform.</p>
      </section>
    );
  const filtered = rows.filter((r) =>
    `${r.name} ${r.id}`.toLowerCase().includes(query.toLowerCase()),
  );
  return (
    <section className="admin-ui-release-list">
      <div className="overview-heading">
        <div>
          <h1>Developer</h1>
          <p>Organisasi pembuat aplikasi di Emisell Apps.</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button
            disabled={busy || creating}
            onClick={() => {
              setCreating(true);
              setError('');
              setSuccess('');
            }}
          >
            <Plus /> Tambah developer
          </Button>
          <Button
            disabled={busy}
            variant="outline"
            onClick={() => {
              setBusy(true);
              setError('');
              setRevision((v) => v + 1);
            }}
          >
            <RefreshCw /> Muat ulang
          </Button>
        </div>
      </div>
      {success && <p role="status">{success}</p>}
      {creating && (
        <form
          className="portal-panel grid gap-4"
          onSubmit={(event) => {
            event.preventDefault();
            void perform(async () => {
              const result = await api.request<{ organization: Organization }>(
                '/developers',
                'POST',
                { organizationName, email, password },
              );
              setPassword('');
              setEmail('');
              setOrganizationName('');
              setCreating(false);
              setSelected(result.organization);
              setSuccess(
                'Akun developer berhasil dibuat dan sudah dapat login. Sampaikan password melalui saluran pribadi; email undangan tidak dikirim otomatis.',
              );
              setRevision((value) => value + 1);
            });
          }}
        >
          <h2>Tambah developer</h2>
          <p>
            Buat organisasi baru beserta akun pemiliknya. Akun ini tidak
            memiliki akses administrator.
          </p>
          <label htmlFor="developer-organization">Nama organisasi</label>
          <Input
            id="developer-organization"
            autoFocus
            required
            maxLength={120}
            value={organizationName}
            disabled={busy}
            onChange={(event) => setOrganizationName(event.target.value)}
          />
          <label htmlFor="developer-email">Email login</label>
          <Input
            id="developer-email"
            type="email"
            autoComplete="off"
            required
            maxLength={254}
            value={email}
            disabled={busy}
            onChange={(event) => setEmail(event.target.value)}
          />
          <label htmlFor="developer-password">Password</label>
          <Input
            id="developer-password"
            type="password"
            autoComplete="new-password"
            required
            minLength={12}
            maxLength={256}
            aria-describedby="developer-password-help"
            value={password}
            disabled={busy}
            onChange={(event) => setPassword(event.target.value)}
          />
          <p id="developer-password-help">
            Minimal 12 karakter. Gunakan password unik dan bagikan hanya kepada
            pemilik akun.
          </p>
          <div className="flex flex-wrap gap-2">
            <Button type="submit" disabled={busy}>
              {busy ? 'Membuat akun…' : 'Buat akun developer'}
            </Button>
            <Button
              type="button"
              variant="outline"
              disabled={busy}
              onClick={() => {
                setCreating(false);
                setPassword('');
                setEmail('');
                setOrganizationName('');
                setError('');
              }}
            >
              Batal
            </Button>
          </div>
        </form>
      )}
      {error && (
        <p role="alert" className="access-error">
          {error}
        </p>
      )}
      {busy && <output>Memuat organisasi…</output>}
      {selected ? (
        <section className="ui-release-table">
          <Button variant="outline" onClick={() => setSelected(null)}>
            Kembali ke developer
          </Button>
          <h2>{selected.name}</h2>
          <p className="break-all">ID organisasi: {selected.id}</p>
          <p>{selected.memberCount} anggota terdaftar</p>
          <p>
            Detail ini hanya-baca. Tidak ada perubahan keanggotaan atau akses.
          </p>
        </section>
      ) : (
        <section className="ui-release-table">
          <h2>
            Organisasi developer <small>· {rows.length} termuat</small>
          </h2>
          <div className="ui-release-filters">
            <Input
              aria-label="Cari developer"
              placeholder="Cari nama organisasi…"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
          </div>
          <Table aria-label="Organisasi developer">
            <TableHeader>
              <TableRow>
                <TableHead>Developer</TableHead>
                <TableHead>ID organisasi</TableHead>
                <TableHead>Anggota</TableHead>
                <TableHead>Aksi</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map((r) => (
                <TableRow key={r.id}>
                  <TableCell>
                    <div className="ui-release-name">
                      <span className="release-icon">
                        <Users />
                      </span>
                      <strong>{r.name}</strong>
                    </div>
                  </TableCell>
                  <TableCell>
                    <code>{r.id}</code>
                  </TableCell>
                  <TableCell>{r.memberCount}</TableCell>
                  <TableCell>
                    <Button
                      disabled={busy}
                      variant="outline"
                      onClick={() =>
                        void perform(async () => {
                          const d = await api.request<{
                            organization: Organization;
                          }>(`/developers/${encodeURIComponent(r.id)}`);
                          setSelected(d.organization);
                        })
                      }
                    >
                      Lihat developer
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
              {!busy && !filtered.length && (
                <TableRow>
                  <TableCell colSpan={4}>
                    {error
                      ? 'Data belum dapat dimuat.'
                      : 'Tidak ada organisasi yang cocok.'}
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
          {next && (
            <Button
              disabled={busy}
              onClick={() =>
                void perform(async () => {
                  const d = await api.request<{
                    organizations: Organization[];
                    nextAfterId: string;
                  }>('/developers', 'GET', undefined, false, { afterId: next });
                  setRows((old) => [...old, ...d.organizations]);
                  setNext(d.nextAfterId);
                })
              }
            >
              Muat berikutnya
            </Button>
          )}
        </section>
      )}
      <div className="scope-explainer">
        <Users />
        <div>
          <h2>Direktori organisasi</h2>
          <p>
            Pencarian berlaku pada daftar termuat. Status verifikasi, jumlah
            aplikasi, dan aktivitas terakhir belum tersedia dalam kontrak ini.
            Setiap akun baru memiliki organisasi sendiri dengan akses pemilik.
          </p>
        </div>
      </div>
    </section>
  );
}
