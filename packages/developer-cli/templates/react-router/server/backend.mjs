import { open } from 'node:fs/promises';
import { constants } from 'node:fs';
import { parseEnv } from 'node:util';
import { join, resolve, relative, isAbsolute, sep } from 'node:path';
import { createLocalCoreVerifier } from './local-core.mjs';
import { createLocalProductReader } from './local-products.mjs';

const names = ['EMISELL_LOCAL_CORE_ORIGIN', 'EMISELL_LOCAL_APP_ID', 'EMISELL_LOCAL_CLIENT_ID', 'EMISELL_LOCAL_CLIENT_SECRET_FILE'];

// Read only known backend settings. Do not mutate process.env or send them to Vite.
export async function createBackend(root, env = process.env) {
  if (env.NODE_ENV === 'production') throw Error('Adapter bawaan hanya untuk lokal, bukan OAuth/production.');
  let file, values = {};
  try {
    file = await open(join(root, '.env'), constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
    const stat = await file.stat();
    if (!stat.isFile() || stat.size > 16384 || (process.platform !== 'win32' && (stat.mode & 0o077))) throw Error('invalid');
    const raw = await file.readFile('utf8');
    if (Buffer.byteLength(raw) > 16384) throw Error('invalid');
    values = parseEnv(raw);
  } catch (error) {
    if (error.code !== 'ENOENT') throw Error('.env harus file privat (600), bukan symlink, maksimum 16 KiB.');
  } finally { await file?.close(); }
  const settings = Object.fromEntries(names.map(name => [name, env[name] ?? values[name] ?? '']));
  if (!names.some(name => settings[name])) return {};
  const config = { coreOrigin: settings.EMISELL_LOCAL_CORE_ORIGIN, appId: settings.EMISELL_LOCAL_APP_ID, clientId: settings.EMISELL_LOCAL_CLIENT_ID };
  try {
    const secretFile = settings.EMISELL_LOCAL_CLIENT_SECRET_FILE;
    if (secretFile) for (const folder of ['app', 'public', 'node_modules', 'build/client']) {
      const offset = relative(resolve(root, folder), resolve(secretFile));
      if (!offset || (!offset.startsWith('..' + sep) && !isAbsolute(offset))) throw Error('Secret must be outside browser source/assets');
    }
    const verifySession = createLocalCoreVerifier(config);
    const readProducts = settings.EMISELL_LOCAL_CLIENT_SECRET_FILE ? createLocalProductReader({ ...config,
      secretFile: settings.EMISELL_LOCAL_CLIENT_SECRET_FILE, publicDirectory: join(root, 'public'),
    }) : undefined;
    const readResource = settings.EMISELL_LOCAL_CLIENT_SECRET_FILE ? (token, { path, ...options }) => createLocalProductReader({ ...config,
      secretFile: settings.EMISELL_LOCAL_CLIENT_SECRET_FILE, publicDirectory: join(root,'public'), resourcePath:path })(token,options) : undefined;
    return { verifySession, readProducts, readResource };
  } catch { throw Error('Konfigurasi backend lokal belum lengkap/valid. Periksa .env tanpa membagikan secret.'); }
}
