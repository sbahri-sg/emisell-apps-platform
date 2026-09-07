import { getIdentity } from './bridge.mjs';

const el = Object.fromEntries(['status', 'search', 'load', 'rows', 'results', 'page', 'previous', 'next'].map(id => [id, document.getElementById(id)]));
let config, busy = false, ready = false, activeSearch = '', cursors = [null], pageIndex = 0, nextCursor = null, generation = 0, request;
const say = (text, tone = '') => { el.status.textContent = text; el.status.dataset.tone = tone; };
const controls = () => {
  el.search.disabled = el.load.disabled = busy || !ready;
  el.previous.disabled = busy || pageIndex === 0;
  el.next.disabled = busy || !nextCursor;
  el.load.setAttribute('aria-busy', String(busy));
};
const clear = () => { el.rows.replaceChildren(); el.results.hidden = true; cursors = [null]; pageIndex = 0; nextCursor = null; el.page.textContent = 'Belum ada data dimuat'; };

async function load(index, search) {
  if (busy || !ready) return;
  if (new TextEncoder().encode(search).length > 100 || /[\x00-\x1f\x7f]/.test(search)) { say('Kata pencarian terlalu panjang atau tidak valid.', 'critical'); return; }
  const cursor = index === 0 ? null : cursors[index];
  if (index > 0 && !cursor) return;
  busy = true; controls(); say('Membaca produk dari toko…');
  // Clear old rows while authorization is being rechecked; never retain a bearer.
  el.rows.replaceChildren(); el.results.hidden = true;
  const current = ++generation;
  request = new AbortController();
  try {
    const identity = await getIdentity({ parentOrigin: config.parentOrigin });
    if (current !== generation) return;
    const query = new URLSearchParams({ limit: '5' });
    if (search) query.set('q', search);
    if (cursor) query.set('cursor', cursor);
    const response = await fetch(`/api/products?${query}`, { method: 'POST',
      headers: { Authorization: `Bearer ${identity.token}` }, credentials: 'omit', cache: 'no-store', redirect: 'error',
      signal: AbortSignal.any([request.signal, AbortSignal.timeout(15000)]),
    });
    if ([401, 403].includes(response.status)) throw Error('denied');
    if (!response.ok) throw Error('unavailable');
    const value = await response.json();
    if (!Array.isArray(value.data) || value.data.length > 5 || !value.meta || !(value.meta.nextCursor === null || typeof value.meta.nextCursor === 'string')) throw Error('invalid');
    if (current !== generation) return;
    for (const product of value.data) {
      const row = document.createElement('tr'), name = document.createElement('td'), price = document.createElement('td'), id = document.createElement('small');
      name.textContent = product.name; id.textContent = `ID: ${product.id}`; name.append(id);
      // The current API has no currency field: do not assume a currency symbol.
      price.textContent = product.price; row.append(name, price); el.rows.append(row);
    }
    if (index === 0) cursors = [null];
    activeSearch = search; pageIndex = index; nextCursor = value.meta.nextCursor;
    cursors = cursors.slice(0, index + 1);
    if (nextCursor) cursors[index + 1] = nextCursor;
    el.results.hidden = value.data.length === 0;
    el.page.textContent = `Halaman ${index + 1} · ${value.data.length} produk${nextCursor ? '' : ' · Halaman terakhir'}`;
    say(value.data.length ? `${value.data.length} produk berhasil dibaca${search ? ' sesuai pencarian' : ' dari toko ini'}.` : search ? 'Tidak ada produk yang cocok. Coba nama atau SKU lain.' : 'Belum ada produk untuk ditampilkan.', value.data.length ? 'success' : '');
    el.load.textContent = 'Cari / muat ulang';
  } catch (error) {
    if (current !== generation) return;
    clear();
    say(error.message === 'denied' ? 'Akses ditolak atau sudah dicabut. Periksa izin dan status instalasi di Dashboard seller.' : 'Produk belum dapat dimuat. Periksa koneksi backend, lalu coba lagi.', 'critical');
  } finally { if (current === generation) { busy = false; controls(); } }
}

el.load.addEventListener('click', () => load(0, el.search.value.trim()));
el.search.addEventListener('keydown', event => { if (event.key === 'Enter') { event.preventDefault(); load(0, el.search.value.trim()); } });
el.previous.addEventListener('click', () => load(pageIndex - 1, activeSearch));
el.next.addEventListener('click', () => load(pageIndex + 1, activeSearch));
window.addEventListener('pagehide', () => { generation++; request?.abort(); clear(); busy = false; controls(); });
try {
  const response = await fetch('/dev-config.json', { cache: 'no-store', credentials: 'omit' });
  if (!response.ok) throw Error('config');
  config = await response.json();
  ready = window.parent !== window;
  say(ready ? 'Siap membaca produk. Klik Baca produk untuk memeriksa izin dan memuat data.' : 'Preview lokal. Buka melalui Dashboard seller setelah review, assignment dan instalasi disetujui.');
  controls();
} catch { say('Konfigurasi preview tidak tersedia. Jalankan melalui emisell app dev.', 'critical'); }
