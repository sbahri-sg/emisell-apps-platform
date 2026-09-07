import { getIdentity } from './bridge.mjs';

const status = document.querySelector('#status');
const connect = document.querySelector('#connect');
let expiryTimer;
const say = (text, tone) => { status.textContent = text; status.dataset.tone = tone || ''; };
try {
  const response = await fetch('/dev-config.json', { cache: 'no-store' });
  if (!response.ok) throw Error('config');
  const config = await response.json();
  if (window.parent === window) {
    say('Preview lokal. Buka melalui Dashboard seller setelah rilis dan assignment Testing disetujui.');
  } else {
    say('Belum terautentikasi. Periksa koneksi untuk meminta identitas dari parent yang diizinkan.');
    connect.disabled = false;
    connect.addEventListener('click', async () => {
      clearTimeout(expiryTimer);
      say('Memeriksa identitas dan akses terkini…');
      connect.disabled = true; connect.setAttribute('aria-busy', 'true');
      try {
        const identity = await getIdentity({ parentOrigin: config.parentOrigin });
        // Never decode claims as authority or persist/log this bearer token.
        const verified = await fetch('/api/session', {
          method: 'POST', headers: { Authorization: `Bearer ${identity.token}` },
          cache: 'no-store', redirect: 'error', signal: AbortSignal.timeout(10000),
        });
        if (verified.status === 503) say('Bridge merespons, tetapi verifikasi backend tidak tersedia. Akses tetap ditolak.', 'critical');
        else if (verified.ok) {
          const session = await verified.json();
          if (session.status !== 'connected' || !Number.isSafeInteger(session.expiresAt)) throw Error('invalid session');
          say('Identitas diverifikasi backend. Ini bukan izin untuk mengakses resource toko.', 'success');
          // No polling. Clear the status when this short-lived identity expires.
          expiryTimer = setTimeout(() => say('Identitas kedaluwarsa. Periksa kembali sebelum melanjutkan.'), Math.max(0, session.expiresAt * 1000 - Date.now()));
        } else say('Identitas atau akses ditolak backend.', 'critical');
      } catch { say('Koneksi tidak tersedia atau identitas ditolak. Tidak ada akses yang diberikan.', 'critical'); }
      finally { connect.disabled = false; connect.removeAttribute('aria-busy'); }
    });
  }
} catch { say('Konfigurasi preview tidak tersedia. Jalankan melalui emisell app dev.', 'critical'); }
