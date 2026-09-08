import { isIP } from 'node:net';
import { origin } from './origin.mjs';

// Production settings come from the hosting environment, never the browser or
// the development .env. No implicit promotion of a local adapter to production.
export function runtimeOptions(config, { env = process.env, built = false, containerPreview = false, port } = {}) {
  if (containerPreview && !built) throw Error('Container preview harus memakai hasil build.');
  const production = env.NODE_ENV === 'production' && !containerPreview;
  if (production && !built) throw Error('Production harus memakai npm run build lalu npm start, bukan Vite.');
  const rawPort = port ?? env.PORT ?? (production ? 3000 : 4330);
  if (!/^\d+$/.test(String(rawPort)) || !Number.isInteger(Number(rawPort)) || Number(rawPort) < (port === 0 ? 0 : 1) || Number(rawPort) > 65535) throw Error('PORT harus 1–65535 (port 0 hanya lewat API test otomatis).');
  const host = env.HOST || (containerPreview ? '0.0.0.0' : '127.0.0.1');
  if (!isIP(host) || !['127.0.0.1', '0.0.0.0'].includes(host)) throw Error('HOST harus 127.0.0.1 atau 0.0.0.0.');
  const mode = production ? 'production' : containerPreview ? 'container-preview' : 'development';
  if (production || containerPreview) {
    if (Object.keys(env).some(k => k.startsWith('EMISELL_LOCAL_') && env[k])) throw Error('Konfigurasi EMISELL_LOCAL_* tidak boleh digunakan di runtime container/production.');
  }
  if (production) {
    let appOrigin, parentOrigin;
    try {
      appOrigin = origin(env.EMISELL_APP_URL);
      parentOrigin = origin(env.EMISELL_DASHBOARD_ORIGIN);
      if (!appOrigin.startsWith('https://') || !parentOrigin.startsWith('https://') || appOrigin === parentOrigin ||
          env.EMISELL_APP_URL !== appOrigin || env.EMISELL_DASHBOARD_ORIGIN !== parentOrigin) throw Error('invalid');
    } catch { throw Error('EMISELL_APP_URL dan EMISELL_DASHBOARD_ORIGIN wajib origin HTTPS berbeda, tanpa path.'); }
    if (env.EMISELL_TLS_TERMINATION !== 'external') throw Error('Set EMISELL_TLS_TERMINATION=external setelah menyiapkan reverse proxy HTTPS dan jaringan privat.');
    if (!['disabled', 'custom'].includes(env.EMISELL_BACKEND_MODE || 'disabled')) throw Error('EMISELL_BACKEND_MODE harus disabled atau custom.');
    return { mode, production, host, port: Number(rawPort), config: { ...config, appOrigin, parentOrigin } };
  }
  if (!containerPreview && (host !== '127.0.0.1' || (config.appOrigin && !/^https:\/\/[a-z0-9.-]+\.test$/.test(config.appOrigin)))) throw Error('Vite/preview lokal hanya loopback dengan origin HTTPS .test.');
  if (containerPreview) {
    if (env.EMISELL_BACKEND_MODE && env.EMISELL_BACKEND_MODE !== 'disabled') throw Error('Container preview hanya UI; backend harus disabled.');
    return { mode, production, host, port: Number(rawPort), config: { ...config, appOrigin: null, endpointProof: undefined } };
  }
  return { mode, production, host, port: Number(rawPort), config };
}

export async function deploymentBackend(root, env, config) {
  if ((env.EMISELL_BACKEND_MODE || 'disabled') === 'disabled') return {};
  // Fixed server-side module under the app's control, not a URL/module from input.
  const { pathToFileURL } = await import('node:url');
  const { join } = await import('node:path');
  try {
    const module = await import(pathToFileURL(join(root, 'server/production-backend.mjs')));
    const backend = await module.createBackend({ env, config });
    if (!['verifySession', 'readProducts', 'checkReady'].every(k => typeof backend?.[k] === 'function')) throw Error('incomplete');
    return backend;
  } catch { throw Error('Adapter production belum siap. Implementasikan server/production-backend.mjs sesuai DEPLOYMENT.md. Tidak ada fallback lokal.'); }
}
