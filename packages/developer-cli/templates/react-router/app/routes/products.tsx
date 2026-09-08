import { useEffect, useRef, useState } from 'react';
import { useRouteLoaderData } from 'react-router';
import type { FormEvent } from 'react';
import type { AppConfig } from '../root';
import { emisell } from '../lib/emisell';
import type { ProductPage } from '../lib/emisell';

export function meta() { return [{ title: 'Produk · Emisell App' }]; }

export default function Products() {
  const { parentOrigin } = useRouteLoaderData<AppConfig>('root')!;
  const [embedded, setEmbedded] = useState(false), [busy, setBusy] = useState(false);
  const [search, setSearch] = useState(''), [activeSearch, setActiveSearch] = useState('');
  const [page, setPage] = useState(0), [data, setData] = useState<ProductPage | null>(null);
  const [status, setStatus] = useState({ tone: '', text: 'Buka melalui Dashboard seller untuk membaca produk setelah izin disetujui.' });
  const cursors = useRef<(string | null)[]>([null]);
  const controller = useRef<AbortController | null>(null);
  useEffect(() => {
    setEmbedded(window.parent !== window);
    const clear = () => { controller.current?.abort(); setData(null); setBusy(false); cursors.current = [null]; setPage(0); setStatus({ tone: '', text: 'Muat kembali untuk memeriksa izin terkini.' }); };
    window.addEventListener('pagehide', clear);
    return () => { controller.current?.abort(); window.removeEventListener('pagehide', clear); };
  }, []);
  async function load(index: number, query: string) {
    if (index < 0 || (index > 0 && !cursors.current[index])) return;
    controller.current?.abort();
    const pending = new AbortController(); controller.current = pending;
    setBusy(true); setData(null); setStatus({ tone: '', text: 'Memeriksa izin dan memuat produk…' });
    try {
      const result = await emisell.products(parentOrigin, { q: query, cursor: index ? cursors.current[index] : null, signal: pending.signal });
      if (pending.signal.aborted) return;
      cursors.current = index === 0 ? [null] : cursors.current.slice(0, index + 1);
      if (result.meta.nextCursor) cursors.current[index + 1] = result.meta.nextCursor;
      setPage(index); setActiveSearch(query); setData(result);
      setStatus({ tone: 'success', text: result.data.length ? `${result.data.length} produk berhasil dibaca.` : query ? 'Tidak ada produk yang cocok. Coba nama atau SKU lain.' : 'Belum ada produk di toko ini.' });
    } catch (error) {
      if (!pending.signal.aborted) { cursors.current = [null]; setPage(0); setStatus({ tone: 'error', text: error instanceof Error ? error.message : 'Produk belum dapat dimuat.' }); }
    } finally { if (!pending.signal.aborted) setBusy(false); }
  }
  function searchProducts(event: FormEvent) { event.preventDefault(); void load(0, search.trim()); }
  return <section className="starter-card"><div className="section-heading"><div><h2>Produk toko</h2><p>Contoh baca produk asli, pencarian nama/SKU, dan pagination.</p></div><span className="scope">read_products</span></div>
    <form className="product-search" onSubmit={searchProducts}><label htmlFor="product-search">Nama atau SKU</label>
      <div><input id="product-search" value={search} onChange={event => setSearch(event.target.value)} placeholder="Cari produk…" maxLength={100} disabled={busy}/>
        <button className="primary" type="submit" disabled={!embedded || busy}>{busy ? 'Memuat…' : 'Baca produk'}</button></div></form>
    <div className={'notice ' + status.tone} role="status" aria-live="polite">{status.text}</div>
    {data && data.data.length > 0 && <><div className="table-scroll"><table><thead><tr><th>Produk</th><th>Harga</th></tr></thead><tbody>
      {data.data.map(product => <tr key={product.id}><td>{product.name}<small>{product.id}</small></td><td>{product.price}</td></tr>)}
    </tbody></table></div><div className="pagination"><span>Halaman {page + 1}{!data.meta.nextCursor && ' · Halaman terakhir'}</span><div>
      <button disabled={busy || page === 0} onClick={() => load(page - 1, activeSearch)}>Sebelumnya</button>
      <button disabled={busy || !data.meta.nextCursor} onClick={() => load(page + 1, activeSearch)}>Berikutnya</button>
    </div></div></>}
    <p className="footnote">Tidak ada izin mengubah produk atau membaca order. Harga ditampilkan sesuai API, tanpa asumsi mata uang. Data lama dibersihkan saat izin diperiksa kembali.</p>
  </section>;
}
