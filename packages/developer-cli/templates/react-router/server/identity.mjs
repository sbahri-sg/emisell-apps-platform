import { createPublicKey, verify } from 'node:crypto';

export class IdentityError extends Error {
  constructor(code) { super(code); this.code = code; }
}
const invalid = () => { throw new IdentityError('invalid_identity'); };
const valid = value => typeof value === 'string' && /^[\x21-\x7e]{1,256}$/.test(value);
const identityKeys = ['merchantId', 'sub', 'appId', 'installationId'];
function exactKeys(value, keys) {
  return value && typeof value === 'object' && !Array.isArray(value) &&
    Object.keys(value).length === keys.length && keys.every(k => Object.hasOwn(value, k));
}
function decode(value) {
  if (!/^[A-Za-z0-9_-]+$/.test(value)) invalid();
  const bytes = Buffer.from(value, 'base64url');
  if (bytes.toString('base64url') !== value) invalid();
  return bytes;
}
function json(value) {
  try { return JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(decode(value))); }
  catch { invalid(); }
}

// All configuration and callbacks are trusted backend inputs, never browser input.
// resolveExpected must independently resolve installation/staff context from your
// trusted integration. Claims are only lookup hints, never authorization evidence.
// authorizeCurrent must check current installation, staff, client, release and grant.
export function createIdentityVerifier({ keys, issuer, audience, resolveExpected, authorizeCurrent, now = () => Date.now() } = {}) {
  const trusted = new Map();
  if (keys && typeof keys === 'object') for (const [kid, pem] of Object.entries(keys)) {
    if (!valid(kid)) throw new IdentityError('verifier_not_configured');
    const key = createPublicKey(pem);
    if (key.asymmetricKeyType !== 'ed25519') throw new IdentityError('verifier_not_configured');
    trusted.set(kid, key);
  }
  return async function verifySession(token, { signal } = {}) {
    if (!trusted.size || !valid(issuer) || !valid(audience) || typeof resolveExpected !== 'function' || typeof authorizeCurrent !== 'function') throw new IdentityError('verifier_not_configured');
    if (typeof token !== 'string' || token.length > 4096) invalid();
    const parts = token.split('.');
    if (parts.length !== 3) invalid();
    const header = json(parts[0]);
    if (!exactKeys(header, ['alg', 'typ', 'kid']) || header.alg !== 'EdDSA' || header.typ !== 'emisell-embedded-id+jwt' || !valid(header.kid)) invalid();
    const key = trusted.get(header.kid), signature = decode(parts[2]);
    if (!key || signature.length !== 64 || !verify(null, Buffer.from(parts[0] + '.' + parts[1]), key, signature)) invalid();
    const claims = json(parts[1]);
    const timeValid = () => {
      const clock = Math.floor(now() / 1000);
      return Number.isSafeInteger(clock) && Number.isSafeInteger(claims.iat) && Number.isSafeInteger(claims.exp) && claims.iat > 0 && claims.iat <= clock && claims.exp > clock && claims.exp - claims.iat === 60;
    };
    if (!exactKeys(claims, [...identityKeys, 'iss', 'aud', 'iat', 'exp', 'jti']) ||
        !identityKeys.every(k => valid(claims[k])) || claims.iss !== issuer || claims.aud !== audience ||
        typeof claims.jti !== 'string' || !/^[A-Za-z0-9_-]{32}$/.test(claims.jti) || !timeValid()) invalid();
    signal?.throwIfAborted();
    const hints = Object.freeze(Object.fromEntries(identityKeys.map(k => [k, claims[k]])));
    const expected = await resolveExpected(hints, { audience, signal });
    if (!expected || !identityKeys.every(k => valid(expected[k]) && expected[k] === claims[k])) throw new IdentityError('access_denied');
    // Require an explicit boolean; an empty/no-op async callback must not grant access.
    if (await authorizeCurrent(hints, { audience, signal }) !== true) throw new IdentityError('access_denied');
    signal?.throwIfAborted();
    if (!timeValid()) invalid();
    return { merchantId: claims.merchantId, actorId: claims.sub, appId: claims.appId, installationId: claims.installationId, expiresAt: claims.exp };
  };
}
