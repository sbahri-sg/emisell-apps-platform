import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm, writeFile, chmod, symlink, unlink, readFile, readdir } from 'node:fs/promises';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { initApp, startDev } from '../src/development.mjs';
import { readProject, supportedNode, sourceFiles } from '../src/project.mjs';
import { createBackend } from '../templates/react-router/server/backend.mjs';
import { startBoundary } from '../templates/react-router/server/http.mjs';
import { request } from 'node:http';

async function fixture(t) {
  const base = await mkdtemp(join(tmpdir(), 'emisell-template-'));
  t.after(() => rm(base, { recursive: true, force: true }));
  return { base, root: await initApp(join(base, 'app'), 'http://localhost:3000') };
}

test('only the React Router scaffold is generated and old projects are never migrated in place', async t => {
  const { base, root } = await fixture(t);
  assert.equal((await readProject(root)).template, 'react-router');
  const pkg = JSON.parse(await readFile(join(root, 'package.json')));
  assert.equal(pkg.private, true);
  assert.equal(pkg.dependencies['react-router'], '7.18.3');
  assert.equal(pkg.scripts.build, 'react-router build');
  assert.deepEqual(await readdir(join(root, 'public')), ['favicon.svg']);
  for (const file of sourceFiles) assert.ok((await readFile(join(root, file))).length, file);
  for (const template of ['embedded', 'products']) {
    await assert.rejects(initApp(join(base, template), 'http://localhost:3000', template), /sudah dihapus/);
    const old = JSON.stringify({ schema: 'emisell.local-app/v1', template, parentOrigin: 'http://localhost:3000' });
    await writeFile(join(root, 'emisell.app.json'), old);
    await assert.rejects(readProject(root), /CLI 0.3.1/);
    await assert.rejects(startDev(root), /CLI 0.3.1/);
    assert.equal(await readFile(join(root, 'emisell.app.json'), 'utf8'), old);
  }
});

test('framework Node floor matches Vite, without restricting other CLI commands', () => {
  for (const value of ['20.20.0', '22.0.0', '22.11.9']) assert.equal(supportedNode(value), false);
  for (const value of ['22.12.0', '22.22.0', '24.16.0']) assert.equal(supportedNode(value), true);
});

test('backend env is optional, private, bounded, non-symlink and never mutates process env', async t => {
  const { base, root } = await fixture(t);
  assert.deepEqual(await createBackend(root, {}), {});
  const file = join(root, '.env');
  const settings = {
    EMISELL_LOCAL_CORE_ORIGIN: 'http://127.0.0.1:8000',
    EMISELL_LOCAL_APP_ID: 'app_' + 'A'.repeat(26),
    EMISELL_LOCAL_CLIENT_ID: 'eac_' + 'B'.repeat(26),
  };
  const original = process.env.EMISELL_LOCAL_APP_ID;
  await writeFile(file, Object.entries(settings).map(([key, value]) => key + '=' + value).join('\n'), { mode: 0o600 });
  const backend = await createBackend(root, {});
  assert.equal(typeof backend.verifySession, 'function');
  assert.equal(backend.readProducts, undefined);
  assert.equal(process.env.EMISELL_LOCAL_APP_ID, original);
  await assert.rejects(createBackend(root, { EMISELL_LOCAL_CORE_ORIGIN: 'https://core.example.com' }), /Konfigurasi/);
  for (const directory of ['app', 'public', 'node_modules', 'build/client']) {
    await assert.rejects(createBackend(root, { EMISELL_LOCAL_CLIENT_SECRET_FILE: join(root, directory, 'client.secret') }), /Konfigurasi/);
  }
  await assert.rejects(createBackend(root, { NODE_ENV: 'production' }), /production/);
  if (process.platform !== 'win32') {
    await chmod(file, 0o644);
    await assert.rejects(createBackend(root, {}), /600/);
    await chmod(file, 0o600);
  }
  await writeFile(file, 'x'.repeat(16385));
  await assert.rejects(createBackend(root, {}), /16 KiB/);
  await unlink(file);
  await writeFile(join(base, 'private.env'), 'CANARY=private', { mode: 0o600 });
  await symlink(join(base, 'private.env'), file);
  await assert.rejects(createBackend(root, {}), /symlink/);
  await unlink(file);
  await writeFile(file, 'EMISELL_LOCAL_CLIENT_SECRET_FILE=/private/secret');
  await chmod(file, 0o600);
  await assert.rejects(createBackend(root, {}), /Konfigurasi/);
});

test('HTTP shutdown terminates an open HMR socket; foreign websocket Origin is rejected', async t => {
  const server = await startBoundary({ config: { parentOrigin: 'http://localhost:3000' }, port: 0 });
  t.after(() => { server.closeAllConnections(); server.close(); });
  server.on('upgrade', (_req, socket) => { if (!socket.destroyed) socket.write('HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n'); });
  const origin = 'http://127.0.0.1:' + server.address().port;
  const connect = source => new Promise((resolve, reject) => {
    const req = request(origin, { headers: { Origin: source, Connection: 'Upgrade', Upgrade: 'websocket' } });
    const timer = setTimeout(() => req.destroy(Error('timeout')), 1000);
    req.once('error', error => { clearTimeout(timer); reject(error); });
    req.once('upgrade', (_res, socket) => { clearTimeout(timer); resolve(socket); });
    req.end();
  });
  await assert.rejects(connect('http://foreign.invalid'));
  const socket = await connect(origin);
  t.after(() => socket.destroy());
  await Promise.race([
    new Promise(resolve => server.close(resolve)),
    new Promise((_, reject) => { const timer = setTimeout(() => reject(Error('shutdown hung')), 1000); timer.unref(); }),
  ]);
});
