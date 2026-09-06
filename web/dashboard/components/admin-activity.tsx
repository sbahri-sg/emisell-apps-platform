'use client';
import { useEffect, useState } from 'react';
import { Activity, RefreshCw } from 'lucide-react';
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
import type { PortalAPI, Submission, Audit } from '@/lib/portal';
export default function AdminActivity({
  api,
  submissions,
}: {
  api: PortalAPI;
  submissions: Submission[];
}) {
  const [selected, setSelected] = useState('');
  const id = selected || submissions[0]?.id || '';
  const [snapshot, setSnapshot] = useState<{
    id: string;
    history: Audit[];
  } | null>(null);
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');
  const [action, setAction] = useState('all');
  const [revision, setRevision] = useState(0);
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    if (!id) return;
    let alive = true;
    api
      .request<{ history: Audit[] }>(`/submissions/${encodeURIComponent(id)}`)
      .then((d) => {
        if (alive) setSnapshot({ id, history: d.history });
      })
      .catch((e) => {
        if (alive) setError(e.message);
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [api, id, revision]);
  const history = snapshot?.id === id ? snapshot.history : [];
  const rows = history
    .filter(
      (r) =>
        (action === 'all' || r.action === action) &&
        `${r.actorId} ${r.action}`.toLowerCase().includes(query.toLowerCase()),
    )
    .slice()
    .sort((a, b) => Date.parse(b.occurredAt) - Date.parse(a.occurredAt));
  return (
    <section className="admin-ui-release-list admin-activity-page">
      <div className="overview-heading">
        <div>
          <h1>Aktivitas</h1>
          <p>Jejak audit review berdasarkan pengajuan aplikasi.</p>
        </div>
        <Button
          disabled={!id || loading}
          variant="outline"
          onClick={() => {
            setLoading(true);
            setError('');
            setSnapshot(null);
            setRevision((v) => v + 1);
          }}
        >
          <RefreshCw /> Muat ulang
        </Button>
      </div>
      <div className="scope-explainer">
        <Activity />
        <div>
          <h2>Aktivitas Review</h2>
          <p>
            Pilih pengajuan untuk melihat kejadian audit yang tercatat. Halaman
            ini belum mencakup audit login, API key, publikasi, atau seluruh
            layanan platform.
          </p>
        </div>
      </div>
      <section className="ui-release-table">
        <div className="ui-release-filters">
          <NativeSelect
            aria-label="Pengajuan aktivitas"
            value={id}
            onChange={(e) => {
              setSelected(e.target.value);
              setLoading(true);
              setError('');
              setSnapshot(null);
              setAction('all');
            }}
          >
            {!submissions.length && (
              <NativeSelectOption value="">
                Belum ada pengajuan
              </NativeSelectOption>
            )}
            {submissions.map((s) => (
              <NativeSelectOption key={s.id} value={s.id}>
                {s.snapshot.name} · {s.version} · {s.id}
              </NativeSelectOption>
            ))}
          </NativeSelect>
          <Input
            aria-label="Cari aktor atau tindakan"
            placeholder="Cari ID aktor atau tindakan…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          <NativeSelect
            aria-label="Filter tindakan"
            value={action}
            onChange={(e) => setAction(e.target.value)}
          >
            <NativeSelectOption value="all">Semua tindakan</NativeSelectOption>
            {[...new Set(history.map((r) => r.action))].map((a) => (
              <NativeSelectOption value={a} key={a}>
                {a}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>
        {error && (
          <p role="alert" className="access-error">
            {error}
          </p>
        )}
        {id && loading && <output>Memuat riwayat…</output>}
        <Table aria-label="Audit pengajuan">
          <TableHeader>
            <TableRow>
              <TableHead>Waktu</TableHead>
              <TableHead>Tindakan</TableHead>
              <TableHead>ID aktor</TableHead>
              <TableHead>Detail</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((r) => (
              <TableRow key={r.id}>
                <TableCell>
                  {new Date(r.occurredAt).toLocaleString('id-ID')}
                </TableCell>
                <TableCell>{r.action}</TableCell>
                <TableCell>
                  <code>{r.actorId}</code>
                </TableCell>
                <TableCell>
                  <details>
                    <summary>Lihat kejadian</summary>
                    <p className="break-all">ID: {r.id}</p>
                    <p className="break-all">
                      {r.feedback || 'Tidak ada catatan tambahan.'}
                    </p>
                  </details>
                </TableCell>
              </TableRow>
            ))}
            {!rows.length && (!loading || !id) && (
              <TableRow>
                <TableCell colSpan={4}>
                  {error
                    ? 'Riwayat belum dapat dimuat.'
                    : !id
                      ? 'Belum ada pengajuan untuk diperiksa.'
                      : 'Tidak ada kejadian yang cocok.'}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
        <p className="access-meta">
          {rows.length} kejadian ditampilkan. Urutan terbaru terlebih dahulu;
          tidak ada aktivitas atau waktu yang dibuat otomatis oleh tampilan.
        </p>
      </section>
    </section>
  );
}
