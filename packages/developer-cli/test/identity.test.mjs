import { test } from 'node:test';
import assert from 'node:assert/strict';
import { generateKeyPairSync, sign } from 'node:crypto';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createIdentityVerifier } from '../templates/react-router/server/identity.mjs';
import { initApp } from '../src/development.mjs';
import { startTestServer as startDev } from './http-fixture.mjs';

const pair = generateKeyPairSync('ed25519');
const pem = pair.publicKey.export({ type: 'spki', format: 'pem' });
const expected = { merchantId: 'merchant1', sub: 'actor1', appId: 'app1', installationId: 'install1' };
const timestamp = () => Math.floor(Date.now() / 1000);
function token(changes = {}, header = {}, key = pair.privateKey) {
  const encode = obj => Buffer.from(JSON.stringify(obj)).toString('base64url');
  const message = encode({ alg: 'EdDSA', typ: 'emisell-embedded-id+jwt', kid: 'key1', ...header }) + '.' + encode({ ...expected, iss: 'platform', aud: 'client1', iat: timestamp(), exp: timestamp() + 60, jti: 'a'.repeat(32), ...changes });
  return message + '.' + sign(null, Buffer.from(message), key).toString('base64url');
}
function verifier(options = {}) {
  return createIdentityVerifier({ keys: { key1: pem }, issuer: 'platform', audience: 'client1', resolveExpected: async () => expected, authorizeCurrent: async () => true, ...options });
}
test('identity verifier accepts pinned Ed25519 and strips token claims from result', async () => {
  const result = await verifier()(token());
  assert.equal(result.merchantId, 'merchant1');
  assert.equal(result.actorId, 'actor1');
  assert.deepEqual(Object.keys(result).sort(), ['actorId', 'appId', 'expiresAt', 'installationId', 'merchantId']);
});
test('rejects wrong signature, issuer, client, type, key, time and identity', async () => {
  const verify = verifier();
  for (const value of [
    token({}, {}, generateKeyPairSync('ed25519').privateKey), token({ iss: 'evil' }), token({ aud: 'other' }),
    token({}, { alg: 'none' }), token({}, { kid: 'missing' }), token({}, { jku: 'https://evil.example' }),
    token({ exp: timestamp() - 1, iat: timestamp() - 61 }), token({ iat: timestamp() + 30, exp: timestamp() + 90 }),
    token({ exp: timestamp() + 3600 }), token({ merchantId: 'other' }), token({ sub: 'other' }),
    token({ appId: 'other' }), token({ installationId: 'other' }), 'bad', 'a'.repeat(4097),
  ]) await assert.rejects(verify(value));
});
test('every request checks current access, including revoke, uninstall and unavailable', async () => {
  let state = true, calls = 0;
  const verify = verifier({ authorizeCurrent: async () => { calls++; return state; } });
  const issued = token();
  await verify(issued); state = false;
  await assert.rejects(verify(issued), /access_denied/);
  assert.equal(calls, 2);
  await assert.rejects(verifier({ resolveExpected: async () => null })(issued), /access_denied/);
  await assert.rejects(verifier({ authorizeCurrent: async () => undefined })(issued), /access_denied/);
  await assert.rejects(verifier({ authorizeCurrent: undefined })(issued), /verifier_not_configured/);
  await assert.rejects(createIdentityVerifier()(issued), /verifier_not_configured/);
  await assert.rejects(verifier({ authorizeCurrent: async () => { throw Error('outage'); } })(issued));
});
test('expired during authorization and cancelled checks cannot return success', async () => {
  let now = Date.now();
  const verify = verifier({ now: () => now, authorizeCurrent: async () => { now += 61000; return true; } });
  await assert.rejects(verify(token()), /invalid_identity/);
  await assert.rejects(verifier()(token(), { signal: AbortSignal.abort() }));
});
test('session HTTP adapter verifies with Origin check and safe error mapping', async t => {
  const base = await mkdtemp(join(tmpdir(), 'emisell-session-test-'));
  t.after(() => rm(base, { recursive: true, force: true }));
  const root = await initApp(join(base, 'app'), 'http://localhost:3000');
  let active = true;
  const server = await startDev(root, 0, { verifySession: verifier({ authorizeCurrent: async () => active }) });
  t.after(() => { server.closeAllConnections(); server.close(); });
  const origin = `http://127.0.0.1:${server.address().port}`;
  const send = (bearer, source = origin) => fetch(origin + '/api/session', { method: 'POST', headers: { Origin: source, Authorization: `Bearer ${bearer}` } });
  const issued = token();
  assert.equal((await send(issued, 'https://evil.example')).status, 403);
  assert.equal((await send('bad')).status, 401);
  const result = await send(issued);
  assert.equal(result.status, 200);
  assert.equal(result.headers.get('cache-control'), 'no-store');
  assert.equal((await result.json()).status, 'connected');
  active = false;
  assert.equal((await send(issued)).status, 403);
});
test('product preview requires verified identity and an explicit server-side reader', async t => {
  const base = await mkdtemp(join(tmpdir(), 'emisell-product-test-'));
  t.after(() => rm(base, { recursive: true, force: true }));
  const root = await initApp(join(base, 'app'), 'http://localhost:3000');
  let active = true, reads = 0, rows = [{ id: 'product1', name: '<script>not HTML</script>', price: '1000', secret: 'never-return' }];
  const server = await startDev(root, 0, { verifySession: verifier({ authorizeCurrent: async () => active }), readProducts: async () => { reads++; return { data: rows, secret: 'not-public' }; } });
  t.after(() => { server.closeAllConnections(); server.close(); });
  const origin = `http://127.0.0.1:${server.address().port}`;
  const issued = token();
  const send = (bearer = issued, source = origin, body) => fetch(origin + '/api/products', { method: 'POST', headers: { Origin: source, Authorization: `Bearer ${bearer}` }, body });
  assert.equal((await send(issued, 'https://evil.example')).status, 403);
  assert.equal((await send('bad')).status, 401);
  assert.equal((await send(issued, origin, '{}')).status, 403);
  assert.equal(reads, 0);
  const response = await send();
  assert.equal(response.status, 200);
  assert.equal(response.headers.get('cache-control'), 'no-store');
  assert.deepEqual(await response.json(), { data: [{ id: 'product1', name: '<script>not HTML</script>', price: '1000' }], meta: { nextCursor: null } });
  active = false;
  assert.equal((await send()).status, 403);
  assert.equal(reads, 1);
  active = true; rows = Array(6).fill(rows[0]);
  assert.equal((await send()).status, 503);
  const identityOnly = await startDev(root, 0, { verifySession: verifier() });
  t.after(() => { identityOnly.closeAllConnections(); identityOnly.close(); });
  const other = `http://127.0.0.1:${identityOnly.address().port}`;
  assert.equal((await fetch(other + '/api/products', { method: 'POST', headers: { Origin: other, Authorization: `Bearer ${issued}` } })).status, 503);
});
