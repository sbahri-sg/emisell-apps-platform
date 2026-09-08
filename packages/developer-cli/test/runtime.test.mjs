import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm, readFile, writeFile, mkdir } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { request } from 'node:http';
import { initApp } from '../src/development.mjs';
import { runtimeOptions, deploymentBackend } from '../templates/react-router/server/runtime.mjs';
import { startBoundary } from '../templates/react-router/server/http.mjs';
import { run } from '../src/cli.mjs';

const config = { name: 'Fixture', parentOrigin: 'http://localhost:3000' };
const productionEnv = { NODE_ENV: 'production', EMISELL_APP_URL: 'https://app.example.com', EMISELL_DASHBOARD_ORIGIN: 'https://seller.example.com', EMISELL_TLS_TERMINATION: 'external' };
async function fixture(t) {
  const base = await mkdtemp(join(tmpdir(), 'emisell-runtime-'));
  t.after(() => rm(base, { recursive: true, force: true }));
  return initApp(join(base, 'app'), config.parentOrigin);
}

test('runtime modes cannot implicitly promote local credentials or expose Vite', () => {
  assert.equal(runtimeOptions(config, { env: {} }).host, '127.0.0.1');
  assert.throws(() => runtimeOptions(config, { env: { HOST: '0.0.0.0' } }), /loopback/);
  assert.throws(() => runtimeOptions(config, { env: productionEnv }), /build/);
  assert.throws(() => runtimeOptions(config, { env: {}, containerPreview: true }), /build/);
  const hosted = runtimeOptions(config, { env: productionEnv, built: true });
  assert.equal(hosted.config.appOrigin, productionEnv.EMISELL_APP_URL);
  assert.equal(hosted.config.parentOrigin, productionEnv.EMISELL_DASHBOARD_ORIGIN);
  assert.equal(hosted.mode, 'production');
  assert.equal(hosted.port, 3000);
  const preview = runtimeOptions(config, { env: { NODE_ENV: 'production' }, built: true, containerPreview: true });
  assert.equal(preview.mode, 'container-preview'); assert.equal(preview.host, '0.0.0.0');
  for (const env of [
    { EMISELL_APP_URL: 'http://app.example.com' }, { EMISELL_APP_URL: 'https://app.example.com/path' },
    { EMISELL_APP_URL: 'https://seller.example.com' }, { EMISELL_DASHBOARD_ORIGIN: '' },
    { EMISELL_TLS_TERMINATION: '' }, { EMISELL_BACKEND_MODE: 'local' }, { EMISELL_LOCAL_CLIENT_SECRET_FILE: '/private/secret' },
    { PORT: '0' }, { PORT: '-1' }, { PORT: '8000.5' }, { PORT: '65536' }, { HOST: 'app.example.com' },
  ]) assert.throws(() => runtimeOptions(config, { env: { ...productionEnv, ...env }, built: true }));
  assert.throws(() => runtimeOptions(config, { env: { EMISELL_BACKEND_MODE: 'custom' }, built: true, containerPreview: true }), /disabled/);
  assert.throws(() => runtimeOptions(config, { env: { EMISELL_LOCAL_CORE_ORIGIN: 'http://127.0.0.1:8000' }, built: true, containerPreview: true }), /EMISELL_LOCAL/);
});

test('deployment scaffolding is generated; unconfigured custom backend fails closed', async t => {
  const root = await fixture(t);
  for (const file of ['Dockerfile', '.dockerignore', '.env.production.example', 'compose.yaml', 'compose.production.yaml', 'DEPLOYMENT.md']) assert.ok((await readFile(join(root, file))).length);
  assert.deepEqual(await deploymentBackend(root, productionEnv, config), {});
  await assert.rejects(deploymentBackend(root, { ...productionEnv, EMISELL_BACKEND_MODE: 'custom' }, config), /belum siap/);
  const docker = await readFile(join(root, 'Dockerfile'), 'utf8');
  assert.match(docker, /USER node/); assert.match(docker, /npm ci --omit=dev/); assert.match(docker, /@sha256:/);
  const compose = await readFile(join(root, 'compose.yaml'), 'utf8');
  assert.match(compose, /127\.0\.0\.1:4330:4330/); assert.match(compose, /read_only: true/);
});

function http(port, path, { host = 'app.example.com', method = 'GET', headers = {} } = {}) {
  return new Promise((resolve, reject) => {
    const req = request({ hostname: '127.0.0.1', port, path, method, headers: { Host: host, ...headers } }, res => {
      const chunks = []; res.on('data', chunk => chunks.push(chunk));
      res.on('end', () => resolve({ status: res.statusCode, headers: res.headers, body: Buffer.concat(chunks).toString() }));
    });
    req.on('error', reject); req.end();
  });
}
test('production HTTP preserves exact Host/Origin, hides dev endpoints, readiness checks dependencies', async t => {
  let ready = false, active = true, web = false, calls = 0;
  const adapters = {
    checkReady: async () => { calls++; return ready; },
    verifySession: async token => {
      if (!active || token !== 'fixture') throw Object.assign(Error(), { code: 'access_denied' });
      return { merchantId: 'fixture-merchant', actorId: 'fixture-actor', appId: 'fixture-app', installationId: 'fixture-install', expiresAt: Math.floor(Date.now()/1000)+60 };
    },
    readProducts: async () => ({ data: [{ id: 'product1', name: 'Fixture', price: '1000' }] }),
  };
  const server = await startBoundary({ config: runtimeOptions(config, { env: productionEnv, built: true }).config, production: true, port: 0,
    isWebReady: () => web, adapters, handler: (req, res) => { assert.equal(req.headers['x-forwarded-host'], undefined); res.end('UI'); } });
  t.after(() => { server.closeAllConnections(); server.close(); });
  const port = server.address().port;
  assert.equal((await http(port, '/health/live')).status, 200); assert.equal(calls, 0);
  assert.equal((await http(port, '/health/ready')).status, 503);
  web = true; assert.equal((await http(port, '/health/ready')).status, 503);
  ready = true; assert.equal((await http(port, '/health/ready')).status, 200);
  assert.equal((await http(port, '/health/ready', { host: `127.0.0.1:${port}` })).status, 200);
  assert.equal((await http(port, '/', { host: `127.0.0.1:${port}` })).status, 403);
  assert.equal((await http(port, '/', { host: 'evil.example', headers: { 'X-Forwarded-Host': 'app.example.com' } })).status, 403);
  const ui = await http(port, '/', { headers: { 'X-Forwarded-Host': 'evil.example' } });
  assert.equal(ui.status, 200); assert.ok(!ui.headers['content-security-policy'].includes('wss:'));
  for (const path of ['/dev-config.json', '/.env', '/server/production-backend.mjs', '/node_modules/a.js', '/app/routes/home.tsx', '/@vite/client']) assert.equal((await http(port, path)).status, 404);
  const headers = { Origin: 'https://app.example.com', Authorization: 'Bearer fixture' };
  assert.equal((await http(port, '/api/products', { method: 'POST', headers })).status, 200);
  assert.equal((await http(port, '/api/products', { method: 'POST', headers: { ...headers, Origin: 'https://seller.example.com' } })).status, 403);
  assert.equal((await http(port, '/api/products', { method: 'POST', headers: { ...headers, Cookie: 'seller=never' } })).status, 403);
  assert.equal((await http(port, '/api/products?merchantId=other', { method: 'POST', headers })).status, 400);
  active = false;
  assert.equal((await http(port, '/api/products', { method: 'POST', headers })).status, 403);
  adapters.checkReady = async () => { throw Error('private dependency failure'); };
  const down = await http(port, '/health/ready'); assert.equal(down.status, 503); assert.ok(!down.body.includes('private'));
});

test('disabled production backend never reports readiness or grants access', async t => {
  const server = await startBoundary({ config: runtimeOptions(config, { env: productionEnv, built: true }).config, port: 0, production: true });
  t.after(() => { server.closeAllConnections(); server.close(); });
  assert.equal((await http(server.address().port, '/health/live')).status, 200);
  assert.equal((await http(server.address().port, '/health/ready')).status, 503);
  assert.equal((await http(server.address().port, '/api/session', { method: 'POST' })).status, 503);
});

test('production bounds concurrent requests and releases capacity when responses finish', async t => {
  const held = [];
  const server = await startBoundary({ config: runtimeOptions(config, { env: productionEnv, built: true }).config,
    production: true, port: 0, handler: (_req, res) => held.push(res) });
  t.after(() => { server.closeAllConnections(); server.close(); });
  const pending = Array.from({ length: 64 }, () => http(server.address().port, '/'));
  // Keep rejections observed if an assertion fails and cleanup closes sockets.
  for (const request of pending) void request.catch(() => {});
  for (let n = 0; n < 100 && held.length < 64; n++) await new Promise(resolve => setTimeout(resolve, 10));
  assert.equal(held.length, 64);
  assert.equal((await http(server.address().port, '/health/live')).status, 503);
  for (const res of held) res.end('UI');
  await Promise.all(pending);
  assert.equal((await http(server.address().port, '/health/live')).status, 200);
});

test('a hanging readiness adapter is aborted and cannot report success', async t => {
  let aborted = false;
  const adapters = { verifySession() {}, readProducts() {}, checkReady: ({ signal }) => new Promise(() => { signal.addEventListener('abort', () => { aborted = true; }, { once: true }); }) };
  const server = await startBoundary({ config: runtimeOptions(config, { env: productionEnv, built: true }).config, production: true, port: 0, adapters });
  t.after(() => { server.closeAllConnections(); server.close(); });
  const start = Date.now();
  assert.equal((await http(server.address().port, '/health/ready')).status, 503);
  assert.ok(aborted); assert.ok(Date.now() - start < 3500);
});

test('app build locates project, runs build once, propagates errors and does not call portal', async t => {
  const root = await fixture(t), child = join(root, 'app/subdir'); await mkdir(child);
  let built = 0;
  const options = { cwd: child, output: () => {}, runBuild: async path => { assert.equal(path, root); built++; return 0; }, fetcher: () => { throw Error('No network'); } };
  await run(['app', 'build'], options); assert.equal(built, 1);
  await run(['app', 'build', '--help'], options); assert.equal(built, 1);
  await assert.rejects(run(['app', 'build', '--yes'], options), /Opsi/);
  await assert.rejects(run(['app', 'build'], { ...options, runBuild: async () => 7 }), /Build gagal/);
});
