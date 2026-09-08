import { request } from 'node:http';
import { open, realpath } from 'node:fs/promises';
import { constants } from 'node:fs';
import { isAbsolute, relative, resolve, sep } from 'node:path';
import { IdentityError } from './identity.mjs';
import { createLocalCoreVerifier } from './local-core.mjs';
import { resourceKind, resourceQuery, resourcePage } from './resource-query.mjs';

// Local-only adapter. All merchant/app/install authority is checked by Core
// and Apps Platform on every request; a verified identity alone grants nothing.
export function createLocalProductReader({ coreOrigin, appId, clientId, secretFile, publicDirectory, resourcePath = '/v1/products' } = {}) {
  createLocalCoreVerifier({ coreOrigin, appId, clientId }); // Validate the pinned loopback destination and bindings.
  if (typeof secretFile !== 'string' || !isAbsolute(secretFile) || !publicDirectory || !isAbsolute(publicDirectory)) throw new IdentityError('verifier_not_configured');
  resourceKind(resourcePath,new URLSearchParams(resourcePath.startsWith('/v1/products/')?'view=inventory':''));
  const endpoint = new URL(resourcePath, coreOrigin);
  const loadSecret = async () => {
    const path = await realpath(secretFile), publicPath = await realpath(publicDirectory);
    const fromPublic = relative(publicPath, path);
    if (!fromPublic || (!fromPublic.startsWith('..' + sep) && !isAbsolute(fromPublic))) throw Error('unsafe_secret');
    if (path !== resolve(secretFile)) throw Error('unsafe_secret');
    const handle = await open(path, constants.O_RDONLY | constants.O_NOFOLLOW);
    try {
      const stat = await handle.stat();
      if (!stat.isFile() || stat.size > 64 || (process.platform !== 'win32' && (stat.mode & 0o077))) throw Error('unsafe_secret');
      const secret = (await handle.readFile('utf8')).trim();
      if (!/^eacs_[A-Za-z0-9_-]{43}$/.test(secret)) throw Error('invalid_secret');
      return secret;
    } finally { await handle.close(); }
  };
  return async (token, { signal, query = {} } = {}) => {
    if (typeof token !== 'string' || token.length > 4096 || !/^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/.test(token)) throw new IdentityError('invalid_identity');
    const filters = resourceQuery(resourcePath, new URLSearchParams(query));
    let secret;
    try { secret = await loadSecret(); } catch { throw new IdentityError('verifier_not_configured'); }
    const url = new URL(endpoint); url.search = new URLSearchParams(filters).toString();
    const value = await new Promise((resolve, reject) => {
      const fail = () => reject(new IdentityError('verifier_unavailable'));
      const req = request(url, { method: 'GET', agent: false, signal,
        headers: { Accept: 'application/json', Authorization: `Bearer ${token}`, 'X-Emisell-App-Client': clientId, 'X-Emisell-App-Secret': secret } }, res => {
        if ([401, 403].includes(res.statusCode)) { res.destroy(); reject(new IdentityError('access_denied')); return; }
        if (res.statusCode !== 200 || res.headers['x-emisell-app-access'] !== 'resource-v1' || !res.headers['content-type']?.includes('application/json')) { res.destroy(); fail(); return; }
        let size = 0; const chunks = [];
        res.on('data', chunk => { size += chunk.length; if (size > 131072) { res.destroy(); fail(); } else chunks.push(chunk); });
        res.on('error', fail);
        res.on('end', () => { try { resolve(JSON.parse(Buffer.concat(chunks).toString('utf8'))); } catch { fail(); } });
      });
      const timer = setTimeout(() => req.destroy(Error('timeout')), 7000);
      req.on('close', () => clearTimeout(timer)); req.on('error', fail); req.end();
    });
    signal?.throwIfAborted();
    try { return resourcePage(resourcePath, value, filters.limit, filters); } catch { throw new IdentityError('verifier_unavailable'); }
  };
}
