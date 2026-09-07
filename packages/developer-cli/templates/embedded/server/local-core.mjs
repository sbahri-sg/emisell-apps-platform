import { request } from 'node:http';
import { IdentityError } from './identity.mjs';

// Local reviewed-UI Core introspection, not a production authorization API.
// No DNS, redirect, proxy, seller cookie or browser-provided destination.
export function createLocalCoreVerifier({ coreOrigin, appId, clientId } = {}) {
  let base;
  try { base = new URL(coreOrigin); } catch { throw new IdentityError('verifier_not_configured'); }
  if (base.protocol !== 'http:' || base.hostname !== '127.0.0.1' || base.username || base.password || base.pathname !== '/' || base.search || base.hash ||
      !/^app_[A-Z2-7]{26}$/.test(appId || '') || !/^eac_[A-Z2-7]{26}$/.test(clientId || '')) throw new IdentityError('verifier_not_configured');
  const url = new URL('/v1/app-platform/core/reviewed-ui/identity', base);
  return async (token, { signal } = {}) => {
    if (typeof token !== 'string' || token.length > 4096 || !/^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/.test(token)) throw new IdentityError('invalid_identity');
    const result = await new Promise((resolve, reject) => {
      const fail = () => reject(new IdentityError('verifier_unavailable'));
      const req = request(url, { method: 'GET', agent: false, signal,
        headers: { Accept: 'application/json', Authorization: `Bearer ${token}`, 'X-Emisell-App-Client': clientId } }, res => {
        if (res.statusCode === 401 || res.statusCode === 403) { res.destroy(); reject(new IdentityError('access_denied')); return; }
        if (res.statusCode !== 200 || !res.headers['content-type']?.includes('application/json')) { res.destroy(); fail(); return; }
        let size = 0; const chunks = [];
        res.on('data', chunk => { size += chunk.length; if (size > 8192) { res.destroy(); fail(); } else chunks.push(chunk); });
        res.on('error', fail);
        res.on('end', () => { try { resolve(JSON.parse(Buffer.concat(chunks).toString('utf8'))); } catch { fail(); } });
      });
      const timer = setTimeout(() => req.destroy(Error('timeout')), 4000);
      req.on('close', () => clearTimeout(timer));
      req.on('error', fail);
      req.end();
    });
    signal?.throwIfAborted();
    const fields = ['merchantId', 'actorId', 'installationId'];
    const now = Date.now() / 1000;
    if (!result || result.appId !== appId || result.clientId !== clientId || !fields.every(k => typeof result[k] === 'string' && /^[\x21-\x7e]{1,256}$/.test(result[k])) || !Number.isSafeInteger(result.expiresAt) || result.expiresAt <= now || result.expiresAt > now + 60) throw new IdentityError('access_denied');
    return { ...Object.fromEntries(fields.map(k => [k, result[k]])), appId, expiresAt: result.expiresAt };
  };
}
