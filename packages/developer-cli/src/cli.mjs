import { readFile, writeFile } from 'node:fs/promises';
import { Client, SessionStore, documentFile } from './client.mjs';
import { runLocal } from './local.mjs';
import { help, commandHelp } from './help.mjs';
import { browserLogin } from './browser-auth.mjs';
import { installSelection } from './install.mjs';

export { help } from './help.mjs';

function parse(args) {
  const words = [], flags = {};
  for (let i = 0; i < args.length; i++) {
    const arg = ({ '-h': '--help', '-n': '--name', '-p': '--path', '-j': '--json' })[args[i]] || args[i];
    if (!arg.startsWith('--')) { words.push(arg); continue; }
    const name = arg.slice(2);
    if (Object.hasOwn(flags, name)) throw new Error(`Opsi ganda: --${name}`);
    if (['password-stdin', 'yes', 'local', 'help', 'json', 'no-open', 'connect', 'reset'].includes(name)) flags[name] = true;
    else {
      if (!args[i + 1] || args[i + 1].startsWith('--')) throw new Error(`Nilai --${name} belum diisi.`);
      flags[name] = args[++i];
    }
  }
  return { words, flags };
}
function allowed(flags, names) {
  if (Object.keys(flags).some(k => !names.includes(k))) throw new Error('Opsi tidak dikenal untuk perintah ini. Lihat --help.');
}
function required(flags, name) {
  if (!flags[name]) throw new Error(`--${name} wajib diisi.`);
  return flags[name];
}
function id(value) {
  if (!/^[A-Za-z0-9_-]{1,200}$/.test(value || '')) throw new Error('ID tidak valid.');
  return value;
}
function mutation(flags) {
  const key = required(flags, 'request-key');
  if (!/^[A-Za-z0-9_-]{8,128}$/.test(key)) throw new Error('Request-key tidak valid (8–128 huruf/angka/_/-).');
  return key;
}
function revision(flags) {
  const value = required(flags, 'revision');
  if (!/^[1-9][0-9]*$/.test(value) || !Number.isSafeInteger(Number(value))) throw new Error('Revision harus bilangan bulat positif dari data terbaru.');
  return Number(value);
}

export async function run(args, { store = new SessionStore(), fetcher = fetch, output = text => console.log(text), ...localOptions } = {}) {
  if (args.length === 0 || (args.length === 1 && ['--help', '-h', 'help', 'app', 'auth'].includes(args[0]))) { output(help); return; }
  if (args.length === 1 && args[0] === '--version') {
    output(JSON.parse(await readFile(new URL('../package.json', import.meta.url), 'utf8')).version); return;
  }
  const { words: w, flags: f } = parse(args);
  if (f.help && Object.hasOwn(commandHelp, w.join(' '))) {
    allowed(f, ['help']); output(commandHelp[w.join(' ')]); return;
  }
  if (w.length === 2 && w[0] === 'auth' && ['login', 'logout'].includes(w[1])) w.shift();
  if (w.join(' ') === 'auth status') w.splice(0, 2, 'whoami');
  const command = w.slice(0, 2).join(' ');
  const connectedDev = command === 'app dev' && (f.connect || f.app || f.store);
  const link = w.join(' ') === 'app config link';
  if (w[0] === 'app' && ['init', 'dev', 'build', 'info', 'doctor'].includes(w[1]) && w.length === 2 && !connectedDev) {
    return runLocal(w[1], f, { ...localOptions, output });
  }
  if (['app deploy', 'app release', 'app config'].includes(command) && !link) {
    throw Error('Perintah ini belum tersedia: sambungan server dan rilis produksi belum diaktifkan. Gunakan app info / app doctor untuk pemeriksaan lokal; ui create atau resource-ui create hanya mengajukan review.');
  }
  if (w.length === 1 && w[0] === 'logout' && f.local) {
    allowed(f, ['local']);
    await store.clear();
    output('Sesi lokal dihapus. Sesi server tidak dicabut; akan berakhir sesuai masa berlakunya.'); return;
  }
  if (w[0] === 'login' && w.length === 1) {
    if (f.email || f['password-stdin']) throw Error('Login email/password developer sudah dihapus. Gunakan emisell auth login --url URL melalui browser merchant.');
    allowed(f, ['url', 'no-open']);
    const client = new Client(required(f, 'url'), '', fetcher);
    const session = await browserLogin(client, { ...localOptions, output, noOpen: f['no-open'] });
    await store.save(session);
    output('Login merchant berhasil. Sesi CLI privat; tidak membaca cookie browser. Sesi berakhir setelah 1 jam tanpa aktivitas.'); return;
  }
  if (command === 'apps init' && w.length === 2) {
    allowed(f, ['file']);
    const template = { name: 'My product app', summary: '', description: '', version: '1.0.0', capability: 'private-products/v1', scopes: [], endpoint: '', accessScopes: { profile: 'shopify-authenticated-2026-09-05', required: ['read_products'], optional: [] } };
    const file = required(f, 'file');
    await writeFile(file, JSON.stringify(template, null, 2) + '\n', { flag: 'wx', mode: 0o600 });
    output('Dokumen aplikasi pribadi read_products dibuat. apps create membuat konfigurasi Active, bukan instalasi toko atau halaman aplikasi.'); return;
  }
  if (command === 'app install' || link || connectedDev) {
    if (!link && w.length !== 2) throw Error('Argumen tidak dikenal. Lihat --help.');
    allowed(f, ['app','store','path','dir','no-open','reset', ...(connectedDev ? ['connect','port','backend'] : [])]);
    const { client, session } = await authenticated(store, fetcher);
    await installSelection(client, session, store, f, { ...localOptions, output, linkOnly: link });
    if (connectedDev) {
      const flags = Object.fromEntries(Object.entries(f).filter(([k]) => ['path','dir','port','backend'].includes(k)));
      return runLocal('dev', flags, { ...localOptions, output });
    }
    return;
  }
  let path, method = 'GET', body, key;
  const uiPath = w[0] === 'resource-ui' ? '/ui-resource-releases' : '/ui-releases';
  if (['ui list', 'resource-ui list'].includes(command) && w.length === 2) {
    allowed(f, []); path = uiPath;
  } else if (['ui show', 'resource-ui show'].includes(command) && w.length === 3) {
    allowed(f, []); path = `${uiPath}/${id(w[2])}`;
  } else if (['ui create', 'resource-ui create'].includes(command) && w.length === 2) {
    allowed(f, ['file', 'request-key', 'yes']);
    if (!f.yes) throw Error('Rilis UI diajukan untuk review. Tambahkan --yes.');
    key = mutation(f); path = uiPath; method = 'POST';
    const raw = await readFile(required(f, 'file'), 'utf8');
    if (Buffer.byteLength(raw) > 16384) throw Error('Dokumen UI terlalu besar.');
    body = JSON.parse(raw);
    const fields = ['name', 'summary', 'version', 'mode', 'url', 'reason'];
    const accepted = w[0] === 'resource-ui' ? [...fields, 'requiredScopes'] : fields;
    if (!body || Array.isArray(body) || typeof body !== 'object' || Object.keys(body).some(k => !accepted.includes(k)) || !fields.every(k => typeof body[k] === 'string' && body[k].trim())) throw Error('Dokumen UI harus berisi name, summary, version, mode, url, reason tanpa credential.');
    if (w[0] === 'resource-ui' && (!Array.isArray(body.requiredScopes) || !body.requiredScopes.length || body.requiredScopes.length > 7 || !body.requiredScopes.every((s,i)=>['read_catalogs','read_collections','read_inventory','read_locations','read_orders','read_products','read_shipping'].includes(s) && (i === 0 || body.requiredScopes[i-1] < s)))) throw Error('requiredScopes harus unik dan terurut: read_catalogs, read_collections, read_inventory, read_locations, read_orders, read_products, read_shipping. Pilih hanya izin yang dibutuhkan; pengajuan tidak memberi akses toko.');
  } else if (command === 'testing list' && w.length === 2) {
    allowed(f, ['after-id']);
    path = '/test-assignments' + (f['after-id'] ? `?afterId=${id(f['after-id'])}` : '');
  } else if (command === 'testing show' && w.length === 3) {
    allowed(f, []); path = `/test-assignments/${id(w[2])}`;
  } else if (command === 'testing request' && w.length === 2) {
    allowed(f, ['release-id', 'release-kind', 'merchant-id', 'reason', 'request-key', 'yes']);
    if (!f.yes) throw Error('Assignment bukan instalasi. Tambahkan --yes untuk mengirim permintaan ke admin.');
    key = mutation(f);
    const reason = required(f, 'reason').trim();
    if (!reason || reason.length > 2000) throw Error('Reason wajib diisi, maksimum 2000 karakter.');
    path = '/test-assignments'; method = 'POST';
    const releaseKind = f['release-kind'] || 'ui';
    if (!['ui', 'ui_resource'].includes(releaseKind)) throw Error('Release kind harus ui atau ui_resource.');
    body = { releaseKind, releaseId: id(required(f, 'release-id')), merchantId: id(required(f, 'merchant-id')), reason };
  } else if (command === 'stores list' && w.length === 2) { allowed(f, []); path = '/account'; }
  else if (w.length === 1 && w[0] === 'whoami') { allowed(f, []); path = '/session'; }
  else if (w.length === 1 && w[0] === 'scopes') { allowed(f, []); path = '/access-scopes'; }
  else if (w.length === 1 && w[0] === 'logout') { allowed(f, []); path = '/logout'; method = 'POST'; body = {}; }
  else if (w.length === 2 && ['apps list', 'reviews list'].includes(command)) {
    allowed(f, []); path = command === 'apps list' ? '/apps' : '/submissions';
  } else if (w.length === 3 && ['apps show', 'reviews show'].includes(command)) {
    allowed(f, []); path = `${command === 'apps show' ? '/apps' : '/submissions'}/${id(w[2])}`;
  } else if (command === 'apps create' && w.length === 2) {
    allowed(f, ['file', 'request-key']); key = mutation(f);
    path = '/apps'; method = 'POST'; body = { revision: 0, document: await documentFile(required(f, 'file')) };
  } else if (command === 'apps update' && w.length === 3) {
    allowed(f, ['file', 'revision', 'request-key']); key = mutation(f);
    path = `/apps/${id(w[2])}`; method = 'PUT'; body = { revision: revision(f), document: await documentFile(required(f, 'file')) };
  } else if (command === 'reviews submit' && w.length === 3) {
    allowed(f, ['revision', 'request-key', 'yes']);
    if (!f.yes) throw new Error('Pengajuan dikirim ke reviewer. Tambahkan --yes untuk mengonfirmasi.');
    key = mutation(f); path = `/apps/${id(w[2])}/submissions`; method = 'POST'; body = { revision: revision(f) };
  } else throw new Error('Perintah tidak dikenal. Jalankan emisell --help.');
  const { client, identity } = await authenticated(store, fetcher, w[0] !== 'logout');
  if (w[0] === 'whoami') { output(JSON.stringify(identity.data, null, 2)); return; }
  if (w[0] === 'logout') {
    await client.request('/api/v1/developer/logout', { method, body });
    await store.clear(); output('Logout berhasil; sesi server dicabut dan sesi lokal dihapus.'); return;
  }
  const result = await client.request('/api/v1/developer' + path, { method, body, key });
  output(JSON.stringify(result.data, null, 2));
}

async function authenticated(store, fetcher, activity = true) {
  const session = await store.load();
  const client = new Client(session.origin, session.cookie, fetcher);
  let identity;
  try { identity = await client.request('/api/v1/developer/session'); }
  catch (error) { if (error.status === 401) await store.clear(); throw error; }
  if (identity.data.user?.surface !== 'developer') throw Error('Sesi bukan akun developer.');
  if (session.accountId && session.accountId !== identity.data.user.id) throw Error('Identitas akun berubah. Login ulang.');
  if (activity) {
    const result = await client.request('/api/v1/developer/activity', { method: 'POST', body: {} });
    const expiry = Date.parse(result.data.expiresAt);
    if (!Number.isFinite(expiry) || expiry <= Date.now() || expiry > Date.now()+3605000) throw Error('Masa berlaku sesi tidak valid.');
    session.expiresAt = expiry;
    await store.save(session);
  }
  return { client, session, identity };
}
