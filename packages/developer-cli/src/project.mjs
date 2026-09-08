import { open, lstat } from 'node:fs/promises';
import { constants } from 'node:fs';
import { basename, dirname, join, resolve } from 'node:path';
import { origin } from './client.mjs';

export const configFile = 'emisell.app.json';
export const templates = ['embedded', 'products'];
export const starterAssets = ['index.html', 'app.mjs', 'bridge.mjs', 'emisell-ui.css'];

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
  if (!config || Array.isArray(config) || config.schema !== 'emisell.local-app/v1') throw Error('Bukan project starter Emisell (schema emisell.local-app/v1).');
  const template = config.template ?? 'embedded';
  if (!templates.includes(template)) throw Error('Template harus embedded atau products.');
  const parentOrigin = origin(config.parentOrigin);
  const appOrigin = config.appOrigin === undefined ? null : origin(config.appOrigin);
  if (appOrigin && (!/^https:\/\/[a-z0-9.-]+\.test$/.test(appOrigin) || appOrigin === parentOrigin)) throw Error('App origin harus domain HTTPS .test terpisah.');
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

export function publicAssets(config) {
  return config.template === 'products' ? [...starterAssets, 'products.css'] : starterAssets;
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
    for (const name of publicAssets(config)) {
      const asset = await lstat(join(directory, 'public', name));
      if (!asset.isFile() || asset.isSymbolicLink()) throw Error('invalid asset');
    }
    record('assets', 'pass', 'Aset preview tersedia sebagai file biasa.');
  } catch {
    record('assets', 'error', 'Periksa folder public dan aset starter: file harus tersedia dan bukan symlink.');
  }
  if (config.uiKitVersion !== '0.1.0') record('ui-kit', 'warning', 'Versi UI Kit tidak dikenali; pastikan aset sesuai starter.');
  record('https', 'warning', config.appOrigin
    ? 'Origin HTTPS lokal dikonfigurasi; DNS, sertifikat dan proxy belum diuji.'
    : 'Preview loopback tersedia. Testing seller memerlukan HTTPS dan konfigurasi operator.');
  record('access', 'warning', 'Backend, koneksi server, assignment dan izin seller belum diuji. Doctor tidak mengimpor backend atau membaca secret.');
  return { readyForLocalPreview: !checks.some(check => check.status === 'error'), storeAccess: 'not-checked', checks };
}
