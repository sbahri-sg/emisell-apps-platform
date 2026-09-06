import {mountEmbeddedApp} from '/bridge.mjs';
const open = document.querySelector('#open');
const revoke = document.querySelector('#revoke');
const feedback = document.querySelector('#feedback');
const state = document.querySelector('#state');
let config, disconnect;
async function getSession() {
  const response = await fetch('/demo/session', {method:'POST', headers:{'Content-Type':'application/json','X-Demo-CSRF':config.csrf}, body:JSON.stringify({installationId:config.installationId}), cache:'no-store'});
  if (!response.ok) throw Error('Akses demo tidak tersedia.');
  return response.json();
}
open.addEventListener('click', async () => {
  open.disabled = true;
  try {
    disconnect?.();
    disconnect = await mountEmbeddedApp({container:document.querySelector('#frame'),getSession});
    state.textContent = 'Demo dibuka';
    feedback.textContent = 'Lihat identitas terverifikasi di aplikasi. Coba perbarui sesi, lalu cabut akses uji.';
  } catch { feedback.textContent = 'Demo tidak dapat dibuka. Akses mungkin sudah dicabut.'; }
});
revoke.addEventListener('click', async () => {
  revoke.disabled = true;
  try {
    const response = await fetch('/demo/revoke', {method:'POST',headers:{'X-Demo-CSRF':config.csrf},cache:'no-store'});
    if (!response.ok) throw Error();
    state.textContent = 'Akses dicabut'; state.classList.add('blocked'); open.disabled = true;
    feedback.textContent = 'Token yang masih berlaku juga ditolak oleh backend. Aplikasi akan menampilkan penolakan dalam 5 detik.';
  } catch { revoke.disabled = false; feedback.textContent = 'Pencabutan belum terkonfirmasi. Silakan coba lagi.'; }
});
try {
  const response = await fetch('/demo/config', {cache:'no-store'});
  if (!response.ok) throw Error();
  config = await response.json();
  open.disabled = false; revoke.disabled = false;
  feedback.textContent = 'Klik Open demo untuk membuka aplikasi tanpa formulir login tambahan.';
} catch { feedback.textContent = 'Server demo tidak tersedia. Coba muat ulang halaman.'; }
window.addEventListener('pagehide', () => disconnect?.(), {once:true});
