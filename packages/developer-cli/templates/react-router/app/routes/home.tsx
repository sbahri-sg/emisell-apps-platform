import { useEffect, useRef, useState } from 'react';
import { Link, useRouteLoaderData } from 'react-router';
import type { AppConfig } from '../root';
import { emisell } from '../lib/emisell';

export function meta() { return [{ title: 'Koneksi · Emisell App' }]; }

export default function Home() {
  const { parentOrigin } = useRouteLoaderData<AppConfig>('root')!;
  const [embedded, setEmbedded] = useState(false), [busy, setBusy] = useState(false);
  const [status, setStatus] = useState({ tone: '', text: 'Preview lokal. Buka aplikasi melalui Dashboard seller untuk menguji koneksi.' });
  const controller = useRef<AbortController | null>(null);
  const expiry = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  useEffect(() => {
    setEmbedded(window.parent !== window);
    const clear = () => { controller.current?.abort(); clearTimeout(expiry.current); setBusy(false); setStatus({ tone: '', text: 'Periksa koneksi kembali untuk memverifikasi sesi terkini.' }); };
    window.addEventListener('pagehide', clear);
    return () => { controller.current?.abort(); clearTimeout(expiry.current); window.removeEventListener('pagehide', clear); };
  }, []);
  async function connect() {
    controller.current?.abort();
    clearTimeout(expiry.current);
    const pending = new AbortController(); controller.current = pending;
    setBusy(true); setStatus({ tone: '', text: 'Memverifikasi identitas melalui backend…' });
    try {
      const session = await emisell.session(parentOrigin, pending.signal);
      if (!pending.signal.aborted) {
        setStatus({ tone: 'success', text: 'Identitas berhasil diverifikasi. Izin baca produk tetap diperiksa setiap permintaan.' });
        expiry.current = setTimeout(() => setStatus({ tone: '', text: 'Identitas sudah kedaluwarsa. Periksa koneksi kembali untuk sesi baru.' }), Math.max(0, Math.min(60000, session.expiresAt * 1000 - Date.now())));
      }
    } catch {
      if (!pending.signal.aborted) setStatus({ tone: 'error', text: 'Koneksi belum tersedia. Periksa backend, sesi seller, dan status instalasi.' });
    } finally { if (!pending.signal.aborted) setBusy(false); }
  }
  return <section className="content-grid">
    <div className="starter-card"><p className="eyebrow">MULAI DI SINI</p><h2>Satu tempat untuk membangun integrasi</h2>
      <p>UI aplikasi sudah siap. Sambungkan backend untuk menguji identitas dan membaca produk dari toko yang memasang aplikasi ini.</p>
      <div className={'notice ' + status.tone} role="status" aria-live="polite">{status.text}</div>
      <button className="primary" onClick={connect} disabled={!embedded || busy}>{busy ? 'Memeriksa…' : 'Periksa koneksi'}</button>
      <Link className="text-link" to="/products">Lihat contoh pembaca produk →</Link>
    </div>
    <aside className="starter-card"><h2>Langkah berikutnya</h2><ol className="steps">
      <li><strong>Edit aplikasi</strong><span>Mulai dari app/routes/home.tsx. Perubahan tampil melalui hot reload.</span></li>
      <li><strong>Hubungkan backend</strong><span>Isi .env privat mengikuti README. Jangan taruh secret di frontend.</span></li>
      <li><strong>Minta persetujuan seller</strong><span>Review, assignment, dan install diperlukan sebelum akses data.</span></li>
    </ol></aside>
  </section>;
}
