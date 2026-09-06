'use client';
import { useEffect, useState } from 'react';
import { ShieldCheck, RefreshCw } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  NativeSelect,
  NativeSelectOption,
} from '@/components/ui/native-select';
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '@/components/ui/table';
import type { PortalAPI, Session } from '@/lib/portal';
type Account = { id: string; email: string; role: string; enabled: boolean };
const roles: Record<string, string> = {
  administrator: 'Administrator',
  reviewer: 'Reviewer',
  operator: 'Operator',
};
export default function AdminStaff({
  api,
  session,
}: {
  api: PortalAPI;
  session: Session;
}) {
  const [rows, setRows] = useState<Account[]>([]);
  const [next, setNext] = useState('');
  const [cursor, setCursor] = useState('');
  const [revision, setRevision] = useState(0);
  const [busy, setBusy] = useState(true);
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');
  const [role, setRole] = useState('all');
  const [status, setStatus] = useState('all');
  const allowed =
    api.surface === 'admin' && session.user.role === 'administrator';
  useEffect(() => {
    if (!allowed) return;
    let alive = true;
    api
      .request<{ accounts: Account[]; nextAfterId: string }>(
        '/staff',
        'GET',
        undefined,
        false,
        { afterId: cursor },
      )
      .then((data) => {
        if (!alive) return;
        setRows((previous) =>
          cursor
            ? [
                ...previous.filter(
                  (r) => !data.accounts.some((n) => n.id === r.id),
                ),
                ...data.accounts,
              ]
            : data.accounts,
        );
        setNext(data.nextAfterId);
      })
      .catch((e) => {
        if (alive)
          setError(
            e instanceof Error ? e.message : 'Daftar staf tidak dapat dimuat.',
          );
      })
      .finally(() => {
        if (alive) setBusy(false);
      });
    return () => {
      alive = false;
    };
  }, [api, allowed, cursor, revision]);
  if (!allowed)
    return (
      <section className="portal-panel">
        <h1>Kelola staf</h1>
        <p>Daftar staf hanya dapat dibaca administrator platform.</p>
      </section>
    );
  const filtered = rows.filter(
    (r) =>
      `${r.email} ${r.id}`.toLowerCase().includes(query.toLowerCase()) &&
      (role === 'all' || r.role === role) &&
      (status === 'all' || r.enabled === (status === 'enabled')),
  );
  return (
    <section className="admin-ui-release-list admin-staff-page">
      <div className="overview-heading">
        <div>
          <h1>Kelola staf</h1>
          <p>Akun tim internal yang memiliki akses ke dashboard admin.</p>
        </div>
        <Button
          variant="outline"
          disabled={busy}
          onClick={() => {
            setBusy(true);
            setError('');
            setCursor('');
            setRevision((v) => v + 1);
          }}
        >
          <RefreshCw />
          Muat ulang
        </Button>
      </div>
      <section className="ui-release-table">
        <div className="ui-release-filters">
          <Input
            aria-label="Cari email atau ID staf"
            placeholder="Cari email atau ID staf…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          <NativeSelect
            aria-label="Filter peran staf"
            value={role}
            onChange={(e) => setRole(e.target.value)}
          >
            <NativeSelectOption value="all">Semua peran</NativeSelectOption>
            {Object.entries(roles).map(([key, label]) => (
              <NativeSelectOption key={key} value={key}>
                {label}
              </NativeSelectOption>
            ))}
          </NativeSelect>
          <NativeSelect
            aria-label="Filter status staf"
            value={status}
            onChange={(e) => setStatus(e.target.value)}
          >
            <NativeSelectOption value="all">Semua status</NativeSelectOption>
            <NativeSelectOption value="enabled">Aktif</NativeSelectOption>
            <NativeSelectOption value="disabled">Nonaktif</NativeSelectOption>
          </NativeSelect>
        </div>
        {error && (
          <p role="alert" className="access-error">
            {error}
          </p>
        )}
        {busy && <output>Memuat staf…</output>}
        <Table aria-label="Daftar staf platform">
          <TableHeader>
            <TableRow>
              <TableHead>Email / ID</TableHead>
              <TableHead>Peran</TableHead>
              <TableHead>Status akun</TableHead>
              <TableHead>Akses</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {filtered.map((r) => (
              <TableRow key={r.id}>
                <TableCell>
                  <strong>{r.email}</strong>
                  <br />
                  <small>{r.id}</small>
                </TableCell>
                <TableCell>{roles[r.role] ?? r.role}</TableCell>
                <TableCell>
                  <span
                    className={`status ${r.enabled ? 'status-approved' : ''}`}
                  >
                    {r.enabled ? 'Aktif' : 'Nonaktif'}
                  </span>
                </TableCell>
                <TableCell>
                  {r.id === session.user.id ? 'Akun Anda' : 'Hanya-baca'}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        {!busy && !error && !filtered.length && (
          <p>Tidak ada staf yang sesuai filter.</p>
        )}
        <div className="ui-release-pagination">
          <span>
            {filtered.length} dari {rows.length} akun yang dimuat
          </span>
          {next && (
            <Button
              variant="outline"
              disabled={busy}
              onClick={() => {
                setBusy(true);
                setError('');
                setCursor(next);
              }}
            >
              Muat berikutnya
            </Button>
          )}
        </div>
      </section>
      <div className="portal-panel">
        <ShieldCheck />
        <h2>Daftar akses tim</h2>
        <p>
          Status aktif menunjukkan akun diizinkan masuk, bukan sedang online.
          Waktu terakhir aktif belum tersedia. Undangan staf dan perubahan peran
          belum tersedia dari halaman ini.
        </p>
      </div>
    </section>
  );
}
