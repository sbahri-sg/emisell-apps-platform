import { useEffect, useRef, useState } from 'react';
import { useParams, useRouteLoaderData } from 'react-router';
import type { AppConfig } from '../root';
import { emisell } from '../lib/emisell';
import type { ResourceResult, ResourceRow } from '../lib/emisell';

export default function Resources() {
  const { kind } = useParams();
  const shipping = kind === 'shipping',
    grouping = kind === 'catalogs' || kind === 'collections';
  const inventory = kind === 'inventory',
    locations = kind === 'locations';
  const title = inventory
    ? 'Stok'
    : locations
      ? 'Lokasi'
      : kind === 'catalogs'
        ? 'Katalog'
        : kind === 'collections'
          ? 'Koleksi'
          : shipping
            ? 'Konfigurasi pengiriman'
            : 'Pesanan toko';
  const { parentOrigin } = useRouteLoaderData<AppConfig>('root')!;
  const [data, setData] = useState<ResourceResult | null>(null),
    [detail, setDetail] = useState<unknown>(null),
    [message, setMessage] = useState('Klik baca untuk memeriksa izin terbaru.'),
    [busy, setBusy] = useState(false),
    [status, setStatus] = useState('');
  const active = useRef<AbortController | null>(null);
  const [q, setQ] = useState('');
  useEffect(() => {
    active.current?.abort();
    setData(null);
    setDetail(null);
    setBusy(false);
    setStatus('');
    setQ('');
    setMessage('Klik baca untuk memeriksa izin terbaru.');
    return () => active.current?.abort();
  }, [kind]);
  const path = inventory
    ? '/v1/products'
    : locations
      ? '/v1/settings/location'
      : shipping
        ? '/v1/settings/shipping'
        : grouping
          ? `/v1/${kind}`
          : '/v1/orders';
  async function load(cursor?: string | null, id?: string) {
    active.current?.abort();
    const pending = new AbortController();
    active.current = pending;
    setBusy(true);
    setData(null);
    setDetail(null);
    setMessage('Memeriksa izin dan membaca data…');
    try {
      if (window.parent === window)
        throw Error('Buka aplikasi dari Dashboard seller.');
      const query = new URLSearchParams();
      if (inventory) query.set('view', 'inventory');
      if (!id) {
        query.set('limit', '5');
        if (cursor) query.set('cursor', cursor);
        if (kind === 'orders' && status) query.set('status', status);
        if ((grouping || locations) && q.trim()) query.set('q', q.trim());
      }
      const result = await emisell.resource(
        parentOrigin,
        id ? path + (shipping ? '/profile/' : '/') + id : path,
        query,
        pending.signal,
      );
      if (pending.signal.aborted) return;
      if (id) setDetail(result.data);
      else setData(result);
      setMessage(
        result.data === null
          ? 'Belum ada konfigurasi tersimpan.'
          : 'Pembacaan berhasil.',
      );
    } catch (error) {
      if (!pending.signal.aborted)
        setMessage(error instanceof Error ? error.message : 'Pembacaan gagal.');
    } finally {
      if (!pending.signal.aborted) setBusy(false);
    }
  }
  const rows: ResourceRow[] = Array.isArray(data?.data)
    ? data.data
    : (data?.data?.profiles ?? []);
  if (
    ![
      'orders',
      'shipping',
      'catalogs',
      'collections',
      'inventory',
      'locations',
    ].includes(kind || '')
  )
    return <p>Resource tidak didukung.</p>;
  return (
    <section className="starter-card">
      <div className="section-heading">
        <h2>{title}</h2>
        <span className="scope">{`read_${kind}`}</span>
      </div>
      <p>
        {inventory
          ? 'Baca saldo stok produk/varian per lokasi aktif. Nilai negatif tetap ditampilkan; saldo bukan jaminan dapat dijual jika pelacakan stok dimatikan.'
          : locations
            ? 'Baca nama dan status lokasi tersimpan, termasuk lokasi nonaktif. Tanpa alamat atau nomor telepon.'
            : grouping
              ? 'Baca metadata dan referensi produk toko. Tidak membuka harga khusus, stok, atau detail produk.'
              : shipping
                ? 'Baca konfigurasi, profil, dan zona tersimpan. Bukan tarif kurir atau pembuatan shipment.'
                : 'Baca pesanan dan item. Tidak mencakup draft, kontak pelanggan, pembayaran, atau perubahan pesanan.'}
      </p>
      {(grouping || locations) && (
        <label>
          Cari nama{' '}
          <input
            value={q}
            disabled={busy}
            maxLength={100}
            onChange={(e) => {
              setQ(e.target.value);
              setData(null);
              setDetail(null);
            }}
          />
        </label>
      )}
      {kind === 'orders' && (
        <label>
          Status{' '}
          <select
            value={status}
            disabled={busy}
            onChange={(e) => {
              setStatus(e.target.value);
              setData(null);
              setDetail(null);
            }}
          >
            <option value="">Semua</option>
            {[
              'UNPAID',
              'PAID',
              'PENDING',
              'PROCESSING',
              'SHIPPING',
              'COMPLETED',
              'CANCELED',
              'REFUNDED',
            ].map((s) => (
              <option key={s}>{s}</option>
            ))}
          </select>
        </label>
      )}
      <button className="primary" disabled={busy} onClick={() => load()}>
        {busy ? 'Memuat…' : `Baca ${title.toLowerCase()}`}
      </button>
      <p role="status" aria-live="polite">
        {message}
      </p>
      {data && rows.length === 0 && <p>Tidak ada data pada halaman ini.</p>}
      {rows.length > 0 && (
        <ul>
          {rows.map((row) => (
            <li key={row.id}>
              <strong>
                {row.name || row.title || row.numberFormat || row.id}
              </strong>{' '}
              {row.status} {row.subtotal} {row.currency}
              {inventory && ` · Saldo ${row.stock}`}
              {locations && (row.isActive ? ' · Aktif' : ' · Nonaktif')}
              {grouping && ` · ${row.productCount} referensi produk`}
              <button disabled={busy} onClick={() => load(null, row.id)}>
                Detail
              </button>
            </li>
          ))}
        </ul>
      )}
      {data?.meta?.nextCursor && (
        <button disabled={busy} onClick={() => load(data.meta?.nextCursor)}>
          Berikutnya
        </button>
      )}
      {detail !== null && (
        <>
          <button disabled={busy} onClick={() => load()}>
            Kembali ke daftar
          </button>
          <pre>{JSON.stringify(detail, null, 2)}</pre>
        </>
      )}
      <p className="footnote">
        Setiap pembacaan memeriksa instalasi dan izin terbaru. Menambah menu
        tidak memberikan izin; rilis dan persetujuan seller tetap diperlukan.
      </p>
    </section>
  );
}
