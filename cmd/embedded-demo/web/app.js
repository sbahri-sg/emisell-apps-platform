import {getIdentity} from '/bridge.mjs';
const status = document.querySelector('#status');
const feedback = document.querySelector('#feedback');
const refresh = document.querySelector('#refresh');
const seller = location.pathname === '/seller';
if (seller) document.querySelector('.muted').textContent = 'Pilot localhost: identitas sesi seller, tanpa akses data bisnis. Sesi diperbarui menjelang kedaluwarsa saat tab terlihat; dijeda saat tab tersembunyi.';
let token, busy = false, renewals = 0, stopped = false, expiresAt = 0, failures = 0, retryAt = 0;
const delays = [2000, 5000, 15000];
function clearIdentity() {
  token = undefined;
  for (const id of ['merchant','actor','app','installation','expiry']) document.querySelector('#'+id).textContent = '—';
}
function failed(error) {
  clearIdentity();
  const denied = error.code === 'denied';
  feedback.dataset.tone = denied ? 'critical' : 'warning';
  status.textContent = denied ? 'Akses ditolak' : 'Koneksi belum tersedia'; status.className = 'eui-badge blocked';
  // Bridge errors intentionally do not disclose why issuance failed.
  feedback.textContent = denied ? 'Backend menolak akses sesi. Periksa akses aplikasi atau coba lagi.' : 'Sesi belum dapat diverifikasi. Mencoba menyambungkan kembali…';
  retryAt = failures < delays.length ? Date.now() + delays[failures++] : Infinity;
  if (retryAt === Infinity && !denied) feedback.textContent = 'Koneksi belum pulih. Coba lagi setelah koneksi tersedia.';
}
async function verify() {
  const response = await fetch(seller ? '/seller/identity' : '/demo/identity',{headers:{Authorization:'Bearer '+token},cache:'no-store',credentials:'omit',signal:AbortSignal.timeout(5000)});
  if (!response.ok) throw Object.assign(Error('Identity unavailable'), {code:[401,403].includes(response.status) ? 'denied' : 'unavailable'});
  const identity = await response.json();
  if (stopped || document.hidden) return;
  if (!Number.isFinite(identity.expiresAt) || identity.expiresAt * 1000 <= Date.now() || !['merchantId','actorId','appId','installationId'].every(key => typeof identity[key] === 'string' && identity[key])) throw Error('Invalid identity');
  expiresAt = identity.expiresAt * 1000;
  for (const [id,field] of [['merchant','merchantId'],['actor','actorId'],['app','appId'],['installation','installationId']]) document.querySelector('#'+id).textContent = identity[field];
  document.querySelector('#expiry').textContent = new Date(identity.expiresAt*1000).toLocaleTimeString('id-ID');
  status.textContent = 'Terhubung'; status.className = 'eui-badge connected';
  feedback.textContent = 'Backend telah memverifikasi identitas dan akses terkini.';
  feedback.dataset.tone = 'success';
}
async function fresh() {
  const session = await getIdentity({parentOrigin:seller ? 'http://localhost:3000' : 'http://127.0.0.1:4320'});
  if (stopped || document.hidden) return;
  token = session.token;
  await verify();
  if (!stopped) document.querySelector('#renewals').textContent = String(++renewals);
}
async function renew(force = true) {
  if (busy || stopped || document.hidden) return;
  busy = true; refresh.disabled = true;
  refresh.setAttribute('aria-busy', 'true');
  try {
    if (force || !token || Date.now() >= expiresAt - 15000) await fresh();
    else {
      try { await verify(); }
      catch (error) {
        // Expiry or a Core restart can invalidate a token without revocation.
        if (error.code !== 'denied') throw error;
        await fresh();
      }
    }
    failures = 0; retryAt = 0;
  } catch (error) { if (!stopped && !document.hidden) failed(error); }
  finally { busy = false; if (!stopped) { refresh.disabled = false; refresh.removeAttribute('aria-busy'); } }
}
function retry() { failures = 0; retryAt = 0; void renew(); }
refresh.addEventListener('click', retry);
// The lightweight timer never makes a request while hidden or while a token is fresh.
const checkTimer = setInterval(() => {
  if (!document.hidden && Date.now() >= retryAt && (!token || Date.now() >= expiresAt - 15000)) void renew(false);
},5000);
const resume = () => {
  if (!document.hidden) retry();
  else {
    clearIdentity();
    status.textContent = 'Sesi dijeda';
    feedback.textContent = 'Sesi akan diperiksa kembali saat tab dibuka.';
  }
};
document.addEventListener('visibilitychange', resume);
window.addEventListener('online', retry);
window.addEventListener('pagehide', () => {
  stopped = true; clearIdentity(); clearInterval(checkTimer);
  document.removeEventListener('visibilitychange', resume); window.removeEventListener('online', retry);
},{once:true});
void renew();
