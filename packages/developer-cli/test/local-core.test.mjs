import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { pathToFileURL } from 'node:url';
import { join } from 'node:path';
import { createLocalCoreVerifier } from '../templates/embedded/server/local-core.mjs';

const appId = 'app_' + 'A'.repeat(26), clientId = 'eac_' + 'A'.repeat(26);
async function serve(t, handler) {
  const server = createServer(handler); server.listen(0, '127.0.0.1'); await once(server, 'listening');
  t.after(() => { server.closeAllConnections(); server.close(); });
  return `http://127.0.0.1:${server.address().port}`;
}
test('local Core configuration rejects remote origins and unbound clients', () => {
  for (const coreOrigin of ['https://example.com', 'http://localhost:8000', 'http://127.0.0.1:8000/path', 'http://user@127.0.0.1:8000', 'http://127.0.0.1:8000?x=1']) assert.throws(() => createLocalCoreVerifier({ coreOrigin, appId, clientId }));
  assert.throws(() => createLocalCoreVerifier({ coreOrigin: 'http://127.0.0.1:8000', appId, clientId: 'wrong' }));
});
test('local Core forwards only bearer and bound client, denies stale/mismatched replies', async t => {
  let status = 200, patch = {}, calls = 0;
  const coreOrigin = await serve(t, (req, res) => {
    calls++;
    assert.equal(req.url, '/v1/app-platform/core/reviewed-ui/identity');
    assert.equal(req.headers.authorization, 'Bearer abc.def.ghi');
    assert.equal(req.headers['x-emisell-app-client'], clientId);
    assert.equal(req.headers.cookie, undefined); assert.equal(req.headers.origin, undefined);
    res.writeHead(status, { 'Content-Type': 'application/json', Location: 'https://example.invalid' });
    res.end(JSON.stringify({ merchantId: 'merchant1', actorId: 'actor1', installationId: 'installation1', appId, clientId, expiresAt: Math.floor(Date.now() / 1000) + 60, ...patch }));
  });
  const verify = createLocalCoreVerifier({ coreOrigin, appId, clientId });
  assert.equal((await verify('abc.def.ghi')).merchantId, 'merchant1');
  for (const change of [{ appId: 'other' }, { clientId: 'other' }, { expiresAt: 1 }, { expiresAt: Math.floor(Date.now() / 1000) + 3600 }]) {
    patch = change; await assert.rejects(verify('abc.def.ghi'), /access_denied/);
  }
  for (const code of [401, 403, 429, 503, 302]) { status = code; await assert.rejects(verify('abc.def.ghi')); }
  assert.equal(calls, 10);
});
test('actual api-service reviewed UI issuer/router rejects revoked seller and changed launch', { skip: !process.env.EMISELL_CORE_TEST_REPO }, async t => {
  const root = process.env.EMISELL_CORE_TEST_REPO;
  const { createReviewedUISessions } = await import(pathToFileURL(join(root, 'src/modules/app-platform/core-preview.ui-sessions.js')));
  const { createCorePreviewRouter } = await import(pathToFileURL(join(root, 'src/modules/app-platform/core-preview.routes.js')));
  const { default: express } = await import(pathToFileURL(join(root, 'node_modules/express/index.js')));
  let active = true, revision = 'a', reads = 0;
  const context = { merchantId: 'synthetic-merchant', coreActorId: 'synthetic-actor' };
  const parentOrigin = 'https://seller.example.com';
  const client = { reviewedLaunch: async () => {
    if (!active) throw Error('uninstalled');
    return { launch: { appId, clientId, releaseDigest: revision.repeat(64), url: 'https://app.example.com/', parentOrigin, mode: 'embedded' } };
  }, resourceProducts: async (session, secret, query) => {
    assert.equal(secret, 'eacs_' + 's'.repeat(43));
    assert.equal(session.merchantId, context.merchantId);
    assert.equal(session.actorId, context.coreActorId);
    assert.equal(session.appId, appId);
    assert.equal(session.clientId, clientId);
    assert.equal(session.installationId, 'ins_test');
    assert.deepEqual(query, reads === 0 ? { limit: 5 } : { limit: 2, q: 'Blue', cursor: 'page.signature' });
    reads++;
    return { data: [] };
  } };
  let staffAllowed = true;
  const authorize = async () => { if (!staffAllowed) throw Error('staff revoked'); return context; };
  // The router owns a private issuer. Issue through its real seller route below.
  const app = express(); app.use(express.json());
  app.use('/v1/app-platform/core', createCorePreviewRouter({ enabled: true, installEnabled: true, reviewedUI: true, client, authorize, origins: [parentOrigin], audit() {} }));
  const coreOrigin = await serve(t, app);
  const verify = createLocalCoreVerifier({ coreOrigin, appId, clientId });
  const response = await fetch(coreOrigin + '/v1/app-platform/core/installations/ins_test/ui-session', { method: 'POST', headers: { Origin: parentOrigin, 'Content-Type': 'application/json', 'X-Emisell-Preview': '1' }, body: '{}' });
  assert.equal(response.status, 200);
  const data = await response.json();
  const issued = data.identityToken ?? data.data?.identityToken;
  assert.equal(typeof issued, 'string');
  assert.equal((await verify(issued)).merchantId, context.merchantId);
  const productHeaders = { Authorization: `Bearer ${issued}`, 'X-Emisell-App-Client': clientId, 'X-Emisell-App-Secret': 'eacs_' + 's'.repeat(43) };
  const read = (headers = productHeaders, query = '') => fetch(coreOrigin + '/v1/app-platform/core/reviewed-ui/products' + query, { headers });
  assert.equal((await read()).status, 200);
  assert.equal(reads, 1);
  for (const patch of [{ Origin: parentOrigin }, { Cookie: 'session=wrong' }, { 'X-Emisell-App-Secret': '' }, { 'X-Emisell-App-Client': 'eac_' + 'B'.repeat(26) }, { Authorization: 'Bearer invalid' }])
    assert.equal((await read({ ...productHeaders, ...patch })).status, 403);
  assert.equal((await read(productHeaders, '?merchantId=other')).status, 400);
  assert.equal((await read(productHeaders, '?limit=2&q=Blue&cursor=page.signature')).status, 200);
  for (const query of ['?q=a&q=b', '?limit=21', '?q=%00', '?published=true']) assert.equal((await read(productHeaders, query)).status, 400);
  staffAllowed = false; await assert.rejects(verify(issued)); staffAllowed = true;
  staffAllowed = false; assert.equal((await read()).status, 403); staffAllowed = true;
  revision = 'b'; await assert.rejects(verify(issued)); revision = 'a';
  revision = 'b'; assert.equal((await read()).status, 403); revision = 'a';
  active = false; await assert.rejects(verify(issued)); active = true;
  active = false; assert.equal((await read()).status, 403); active = true;
  assert.equal(reads, 2, 'unauthorized product requests must not reach the backend');
  const foreignIssuer = createReviewedUISessions({ client, authorize });
  const foreign = await foreignIssuer.issue(context, {}, 'ins_test', parentOrigin);
  await assert.rejects(verify(foreign.identityToken));
});
