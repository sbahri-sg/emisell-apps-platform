import { readFile, writeFile } from 'node:fs/promises';
import { createInterface } from 'node:readline/promises';
import { Client, SessionStore, documentFile } from './client.mjs';
import { initApp, startDev } from './development.mjs';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';

export const help = `Emisell Developer CLI

emisell login --url https://apps-platform.example.com --email developer@example.com
emisell login --url URL --email EMAIL --password-stdin
emisell logout
emisell logout --local
emisell whoami
emisell apps list
emisell apps show APP_ID
emisell apps init --file app.json
emisell apps create --file app.json --request-key UNIQUE_KEY
emisell apps update APP_ID --file app.json --revision NUMBER --request-key UNIQUE_KEY
emisell reviews list
emisell reviews show REVIEW_ID
emisell reviews submit APP_ID --revision NUMBER --request-key UNIQUE_KEY --yes
emisell scopes
emisell ui list
emisell ui show RELEASE_ID
emisell ui create --file ui.json --request-key UNIQUE_KEY --yes
emisell resource-ui list
emisell resource-ui show RELEASE_ID
emisell resource-ui create --file resource-ui.json --request-key UNIQUE_KEY --yes
emisell app init --dir my-app --parent-origin http://localhost:3000 [--template embedded|products]
emisell app dev --dir my-app --port 4330 [--backend ./my-app/backend.mjs]
emisell testing list [--after-id CURSOR]
emisell testing show ASSIGNMENT_ID
emisell testing request --release-id RELEASE_ID [--release-kind ui|ui_resource] --merchant-id MERCHANT_ID --reason TEXT --request-key UNIQUE_KEY --yes
emisell --version

Output API berupa JSON. Tidak ada auto-retry mutasi.
Request-key: 8–128 karakter huruf/angka/_/-. Gunakan key baru untuk operasi baru.
Review bukan publish. CLI tidak memberi akses seller atau menandatangani rilis.
`;

function parse(args) {
  const words = [], flags = {};
  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    if (!arg.startsWith('--')) { words.push(arg); continue; }
    const name = arg.slice(2);
    if (Object.hasOwn(flags, name)) throw new Error(`Opsi ganda: --${name}`);
    if (['password-stdin', 'yes', 'local'].includes(name)) flags[name] = true;
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

async function passwordFromInput(fromStdin) {
  if (fromStdin) {
    if (process.stdin.isTTY) throw new Error('--password-stdin memerlukan pipe, bukan terminal interaktif.');
    let result = '';
    for await (const chunk of process.stdin) {
      result += chunk;
      if (Buffer.byteLength(result) > 1024) throw new Error('Input password terlalu panjang.');
    }
    return result.replace(/\r?\n$/, '');
  }
  if (!process.stdin.isTTY) throw new Error('Gunakan --password-stdin untuk non-interaktif.');
  // Suppress terminal echo; never put a password into argv, environment or logs.
  const { Writable } = await import('node:stream');
  const silent = new Writable({ write(_chunk, _encoding, callback) { callback(); } });
  const rl = createInterface({ input: process.stdin, output: silent, terminal: true });
  process.stderr.write('Password: ');
  try {
    return await new Promise((resolve, reject) => {
      rl.once('SIGINT', () => reject(new Error('Login dibatalkan.')));
      rl.question('').then(resolve, reject);
    });
  } finally { rl.close(); process.stderr.write('\n'); }
}

export async function run(args, { store = new SessionStore(), fetcher = fetch, output = text => console.log(text), password = passwordFromInput } = {}) {
  if (args.length === 0 || (args.length === 1 && ['--help', 'help'].includes(args[0]))) { output(help); return; }
  if (args.length === 1 && args[0] === '--version') {
    output(JSON.parse(await readFile(new URL('../package.json', import.meta.url), 'utf8')).version); return;
  }
  const { words: w, flags: f } = parse(args);
  const command = w.slice(0, 2).join(' ');
  if (command === 'app init' && w.length === 2) {
    allowed(f, ['dir', 'parent-origin', 'template']);
    const target = await initApp(required(f, 'dir'), required(f, 'parent-origin'), f.template || 'embedded');
    output(`Starter dibuat: ${target}. Jalankan emisell app dev --dir "${target}". Belum terhubung ke toko.`); return;
  }
  if (command === 'app dev' && w.length === 2) {
    allowed(f, ['dir', 'port', 'backend']);
    const port = f.port || '4330';
    if (!/^[0-9]+$/.test(port) || Number(port) < 1024 || Number(port) > 65535) throw Error('Port harus 1024–65535.');
    const directory = required(f, 'dir');
    let verifySession, readProducts;
    if (f.backend) {
      // Explicit opt-in: this executes developer-owned backend code, never remote URLs.
      const module = await import(pathToFileURL(resolve(f.backend)));
      if (typeof module.verifySession !== 'function') throw Error('Backend harus mengekspor verifySession.');
      verifySession = module.verifySession;
      if (module.readProducts !== undefined && typeof module.readProducts !== 'function') throw Error('readProducts harus berupa fungsi.');
      readProducts = module.readProducts;
    }
    const server = await startDev(directory, Number(port), { verifySession, readProducts });
    output(`Preview: http://127.0.0.1:${server.address().port}. Refresh setelah edit. Ctrl+C untuk berhenti. Ini bukan server produksi atau instalasi seller.`);
    await new Promise(resolve => {
      const stop = () => { server.closeAllConnections(); server.close(resolve); };
      process.once('SIGINT', stop); process.once('SIGTERM', stop);
      server.once('close', () => { process.removeListener('SIGINT', stop); process.removeListener('SIGTERM', stop); resolve(); });
    }); return;
  }
  if (w.length === 1 && w[0] === 'logout' && f.local) {
    allowed(f, ['local']);
    await store.clear();
    output('Sesi lokal dihapus. Sesi server tidak dicabut; akan berakhir sesuai masa berlakunya.'); return;
  }
  if (w[0] === 'login' && w.length === 1) {
    allowed(f, ['url', 'email', 'password-stdin']);
    const client = new Client(required(f, 'url'), '', fetcher);
    const email = required(f, 'email');
    const secret = await password(Boolean(f['password-stdin']));
    if (!secret) throw new Error('Password kosong.');
    const session = await client.login(email, secret);
    await store.save(session);
    output('Login developer berhasil. Sesi disimpan lokal; password tidak disimpan.'); return;
  }
  if (command === 'apps init' && w.length === 2) {
    allowed(f, ['file']);
    const template = { name: 'My shipping app', summary: '', description: '', version: '0.1.0', capability: 'shipping/v1', scopes: ['orders.read', 'shipping.read', 'shipping.write'], endpoint: '' };
    const file = required(f, 'file');
    await writeFile(file, JSON.stringify(template, null, 2) + '\n', { flag: 'wx', mode: 0o600 });
    output('Template draft shipping dibuat. Isi metadata dan endpoint HTTPS sebelum review.'); return;
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
    if (w[0] === 'resource-ui' && (!Array.isArray(body.requiredScopes) || body.requiredScopes.length !== 1 || body.requiredScopes[0] !== 'read_products')) throw Error('Resource UI hanya mendukung requiredScopes: ["read_products"]. Pengajuan tidak memberi akses toko.');
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
  } else if (w.length === 1 && w[0] === 'whoami') { allowed(f, []); path = '/session'; }
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
  const session = await store.load();
  const client = new Client(session.origin, session.cookie, fetcher);
  // Validate identity for every authenticated command, not a role supplied by local config.
  const identity = await client.request('/api/v1/developer/session');
  if (identity.data.user?.surface !== 'developer') throw new Error('Sesi bukan akun developer.');
  if (w[0] === 'whoami') { output(JSON.stringify(identity.data, null, 2)); return; }
  if (w[0] === 'logout') {
    await client.request('/api/v1/developer/logout', { method, body });
    await store.clear(); output('Logout berhasil; sesi server dicabut dan sesi lokal dihapus.'); return;
  }
  const result = await client.request('/api/v1/developer' + path, { method, body, key });
  output(JSON.stringify(result.data, null, 2));
}
