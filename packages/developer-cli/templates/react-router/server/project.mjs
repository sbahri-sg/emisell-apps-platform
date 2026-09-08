import { open, lstat } from 'node:fs/promises';
import { constants } from 'node:fs';
import { basename, dirname, join, resolve } from 'node:path';
import { origin } from './origin.mjs';

export const configFile = 'emisell.app.json';
export const templates = ['react-router'];
export const sourceFiles = ['package.json', 'vite.config.ts', 'react-router.config.ts', 'tsconfig.json',
  'app/root.tsx', 'app/routes.ts', 'app/entry.server.tsx', 'app/entry.client.tsx', 'app/routes/home.tsx', 'app/routes/products.tsx', 'app/routes/resources.tsx',
  'app/lib/bridge.mjs', 'app/lib/emisell.ts', 'app/styles/emisell-ui.css', 'app/styles/app.css',
  'server/dev.mjs', 'server/http.mjs', 'server/backend.mjs', 'server/standalone.mjs',
  'server/project.mjs', 'server/origin.mjs', 'server/identity.mjs', 'server/local-core.mjs',
  'server/local-products.mjs', 'server/product-query.mjs', 'server/resource-query.mjs', 'server/runtime.mjs', 'server/production-backend.mjs', 'server/healthcheck.mjs',
  'Dockerfile', '.dockerignore', 'compose.yaml', 'DEPLOYMENT.md'];

export function supportedNode(version = process.versions.node) {
  const [major, minor] = version.split('.').map(Number);
  return major > 22 || (major === 22 && minor >= 12);
}

export function appName(value) {
  if (typeof value !== 'string' || !value.trim() || value !== value.trim() || value.length > 100 || /\p{C}/u.test(value)) {
    throw Error('Nama aplikasi wajib diisi, maksimum 100 karakter, tanpa karakter kontrol.');
  }
  return value;
}

// Metadata only. Never import a backend, load .env, or expose arbitrary fields.
export async function readProject(directory) {
  let handle, raw;
  try {
    const path = join(directory, configFile);
    const entry = await lstat(path);
    if (!entry.isFile() || entry.isSymbolicLink()) throw Error('invalid config');
    handle = await open(path, constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
    const stat = await handle.stat();
    if (!stat.isFile() || stat.size > 16384) throw Error('invalid config');
    raw = await handle.readFile('utf8');
    if (Buffer.byteLength(raw) > 16384) throw Error('invalid config');
  } catch (error) {
    if (error.code === 'ENOENT') throw Error('Project tidak ditemukan. Masuk ke folder aplikasi, gunakan --path, atau jalankan emisell app init.');
    throw Error('emisell.app.json harus file biasa (bukan symlink), terbaca, maksimum 16 KiB.');
  } finally { await handle?.close(); }
  let config;
  try { config = JSON.parse(raw); }
  catch { throw Error('emisell.app.json bukan JSON yang valid. Perbaiki konfigurasi lalu jalankan emisell app doctor.'); }
  if (config?.schema === 'emisell.local-app/v1' || ['embedded', 'products'].includes(config?.template)) {
    throw Error('Template lama sudah dihapus dari CLI ini. Buat project React Router baru; project lama tidak diubah. Gunakan CLI 0.3.1 bila masih perlu menjalankan project lama.');
  }
  if (!config || Array.isArray(config) || config.schema !== 'emisell.react-router-app/v1') throw Error('Bukan project React Router Emisell (schema emisell.react-router-app/v1).');
  const template = config.template;
  if (!templates.includes(template)) throw Error('Template harus react-router.');
  const parentOrigin = origin(config.parentOrigin);
  const appOrigin = config.appOrigin === undefined ? null : origin(config.appOrigin);
  if (appOrigin && (!/^https:\/\/[a-z0-9.-]+\.test$/.test(appOrigin) || appOrigin === parentOrigin)) throw Error('App origin development harus domain HTTPS .test terpisah; hosting memakai EMISELL_APP_URL.');
  const proof = config.endpointProof;
  if (proof !== undefined && (!proof || !/^proof_[A-Z2-7]{26}$/.test(proof.id) || !proof.document || Object.keys(proof.document).sort().join(',') !== 'challenge,clientId,releaseSha256,schema' || !Object.values(proof.document).every(v => typeof v === 'string') || Buffer.byteLength(JSON.stringify(proof.document)) > 4096)) throw Error('Endpoint proof tidak valid.');
  return {
    name: appName(config.name ?? basename(resolve(directory))), template, parentOrigin, appOrigin,
    uiKitVersion: config.uiKitVersion === '0.1.0' ? '0.1.0' : null, endpointProof: proof,
  };
}

export async function findProject(start = process.cwd(), explicit = false) {
  let directory = resolve(start);
  for (;;) {
    try {
      await lstat(join(directory, configFile));
      return directory; // Invalid/symlink configs must fail here, never fall back to another app.
    } catch (error) {
      if (error.code !== 'ENOENT') throw Error('Folder project tidak dapat diperiksa. Periksa --path dan izin file.');
    }
    const parent = dirname(directory);
    if (explicit || parent === directory) throw Error('Project tidak ditemukan. Masuk ke folder aplikasi, gunakan --path, atau jalankan emisell app init.');
    directory = parent;
  }
}

export async function inspectProject(directory) {
  const checks = [];
  const record = (id, status, message) => checks.push({ id, status, message });
  let config;
  try {
    config = await readProject(directory);
    record('config', 'pass', 'Konfigurasi project dan origin valid.');
  } catch (error) {
    record('config', 'error', error.message);
    return { readyForLocalPreview: false, storeAccess: 'not-checked', checks };
  }
  try {
    const stat = await lstat(join(directory, 'public'));
    if (!stat.isDirectory() || stat.isSymbolicLink()) throw Error('invalid public');
    for (const name of sourceFiles) {
      const asset = await lstat(join(directory, name));
      if (!asset.isFile() || asset.isSymbolicLink()) throw Error('invalid asset');
    }
    record('assets', 'pass', 'Source React Router, Vite dan backend tersedia.');
  } catch {
    record('assets', 'error', 'Periksa source template dan folder public: file harus tersedia dan bukan symlink.');
  }
  record('node', supportedNode() ? 'pass' : 'error', supportedNode() ? 'Node.js kompatibel.' : 'Template memerlukan Node.js 22.12 atau lebih baru.');
  try {
    // Resolution only, never execute dependency or project code in doctor.
    const { createRequire } = await import('node:module');
    const require = createRequire(join(directory, 'package.json'));
    for (const name of ['vite', '@react-router/dev/vite', '@react-router/express', '@react-router/node', 'express', 'react', 'react-dom', 'react-router', 'typescript']) require.resolve(name);
    record('dependencies', 'pass', 'Dependency framework terpasang.');
  } catch { record('dependencies', 'error', 'Jalankan npm install di folder project terlebih dahulu.'); }
  if (config.uiKitVersion !== '0.1.0') record('ui-kit', 'warning', 'Versi UI Kit tidak dikenali; pastikan aset sesuai starter.');
  record('https', 'warning', config.appOrigin
    ? 'Origin HTTPS lokal dikonfigurasi; DNS, sertifikat dan proxy belum diuji.'
    : 'Preview loopback tersedia. Testing seller memerlukan HTTPS dan konfigurasi operator.');
  record('access', 'warning', 'Backend, koneksi server, assignment dan izin seller belum diuji. Doctor tidak mengimpor backend atau membaca secret.');
  return { readyForLocalPreview: !checks.some(check => check.status === 'error'), storeAccess: 'not-checked', checks };
}
