import { createInterface } from 'node:readline/promises';
import { resolve } from 'node:path';
import { origin } from './client.mjs';
import { openBrowser } from './browser-auth.mjs';

const safeID = value => typeof value === 'string' && /^[A-Za-z0-9_-]{1,128}$/.test(value);
const appID = value => /^app_[A-Z2-7]{26}$/.test(value || '');
const clean = value => String(value ?? '').replace(/[\x00-\x1f\x7f-\x9f]/g, '').slice(0, 200);
async function choose(title, items, { interactive = Boolean(process.stdin.isTTY), output, select } = {}) {
  if (!items.length) throw Error(`${title}: belum tersedia. Login ulang untuk memperbarui daftar toko.`);
  if (!interactive && !select) throw Error(`${title}: gunakan --app dan --store pada terminal non-interaktif.`);
  output([title, ...items.map((x,i) => `${i+1}. ${clean(x.name)} (${x.id})`)].join('\n'));
  let answer;
  if (select) answer = await select(title, items);
  else {
    const rl = createInterface({ input: process.stdin, output: process.stdout });
    try { answer = await rl.question('Pilih nomor: '); } finally { rl.close(); }
  }
  if (!/^[1-9][0-9]*$/.test(String(answer)) || !items[Number(answer)-1]) throw Error('Pilihan tidak valid.');
  return items[Number(answer)-1];
}

export async function installSelection(client, session, store, flags, options) {
  const directory = resolve(options.cwd || process.cwd(), flags.path || flags.dir || '.');
  const previous = session.selections?.[directory];
  const account = (await client.request('/api/v1/developer/account')).data;
  if (!account.profile || !Array.isArray(account.profile.stores) || account.profile.stores.length > 200) throw Error('Akun belum terhubung dengan merchant. Login ulang.');
  const stores = account.profile.stores;
  if (stores.some(s => !safeID(s.id) || !safeID(s.commonId) || typeof s.name !== 'string') || new Set(stores.map(s=>s.id)).size !== stores.length) throw Error('Daftar toko tidak valid.');
  const { data } = await client.request('/api/v1/developer/apps');
  if (!Array.isArray(data.apps) || data.apps.some(a=>!appID(a.id))) throw Error('Daftar aplikasi tidak valid.');
  const apps = data.apps.filter(a => a.activeVersion).map(a => ({ ...a, name: a.document?.name || a.id }));
  const requestedApp = flags.app || (!flags.reset && previous?.appId);
  const app = requestedApp ? apps.find(a=>a.id===requestedApp) : await choose('Pilih aplikasi', apps, options);
  if (!app) throw Error('Aplikasi tidak dimiliki akun ini atau belum memiliki versi aktif. Pilih ulang dengan --reset.');
  const requestedStore = flags.store || (!flags.reset && previous?.storeId);
  const merchant = requestedStore ? stores.find(s=>s.id===requestedStore || s.commonId===requestedStore) : await choose('Pilih toko', stores, options);
  if (!merchant) throw Error('Toko tidak tersedia untuk akun ini. Login ulang atau gunakan --reset.');
  const { data: handoff } = await client.request(`/api/v1/developer/apps/${app.id}/install-url`);
  let target;
  try {
    target = new URL(handoff.url);
    if (target.origin !== origin(account.sellerOrigin) || target.username || target.password || target.hash ||
        target.pathname !== '/auth/stores' || target.searchParams.get('app') !== app.id ||
        target.searchParams.get('version') !== app.activeVersion.document.version || [...target.searchParams].length !== 2) throw Error();
  } catch { throw Error('Alamat instalasi dari server tidak valid.'); }
  // Display-only store profile is NOT authorization. Dashboard rechecks its own
  // cookie, current membership and app permissions at the existing grant route.
  // Keep merchant switching in Core. A preferred store is a navigation hint,
  // never an instruction to change cookies or grant access automatically.
  target.searchParams.set('store', merchant.id);
  const selections = { ...(session.selections || {}), [directory]: { appId: app.id, storeId: merchant.id } };
  await store.save({ ...session, selections });
  options.output(`Aplikasi: ${clean(app.name)}\nToko: ${clean(merchant.name)}\nDashboard akan memeriksa sesi dan izin toko kembali.`);
  if (!options.linkOnly) {
    options.output(`Konfirmasi toko pilihan di Dashboard, lalu lanjutkan consent:\n${target.href}\nBelum dianggap terpasang sampai Anda menekan Install di Dashboard.`);
    if (!flags['no-open']) await (options.open || openBrowser)(target.href);
  }
  return { appId: app.id, storeId: merchant.id, url: target.href };
}
