import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm, stat, chmod, readFile, symlink } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { Client, SessionStore, origin } from '../src/client.mjs';
import { run } from '../src/cli.mjs';

const token = 'A'.repeat(52);
const json = (data, headers = {}) => new Response(JSON.stringify(data), { headers: { 'content-type': 'application/json', ...headers } });
async function fixture(t) {
  const directory = await mkdtemp(join(tmpdir(), 'emisell-cli-test-'));
  t.after(() => rm(directory, { recursive: true, force: true }));
  return { directory, store: new SessionStore(join(directory, 'session')) };
}
test('origin disallows insecure remote, credentials and URL paths', () => {
  for (const url of ['http://example.com', 'https://user:password@example.com', 'https://example.com/api', 'https://example.com/?x=1', 'ftp://localhost', 'https://example.com/#x']) assert.throws(() => origin(url));
  assert.equal(origin('http://127.0.0.1:4317'), 'http://127.0.0.1:4317');
  assert.equal(origin('https://apps.example.com/'), 'https://apps.example.com');
});
test('session store private, outside project, expiration and symlink protection', async t => {
  const { directory, store } = await fixture(t);
  await assert.rejects(store.load(), /Belum login/);
  const session = { origin: 'https://apps.example.com', cookie: `emisell_portal_session=${token}`, expiresAt: Date.now() + 10000 };
  await store.save(session);
  assert.deepEqual(await store.load(), session);
  if (process.platform !== 'win32') {
    const path = join(store.directory, 'session.json');
    assert.equal((await stat(path)).mode & 0o777, 0o600);
    await chmod(path, 0o644);
    await assert.rejects(store.load(), /0600/);
    await chmod(path, 0o600);
    await store.clear();
    await symlink(join(directory, 'target'), path);
    await assert.rejects(store.load());
    await store.clear();
  }
  await store.save({ ...session, expiresAt: 1 });
  await assert.rejects(store.load(), /kedaluwarsa/);
  await store.clear();
});
test('login, identity, list, logout through real HTTP with matching Origin and cookie', async t => {
  const { store } = await fixture(t);
  let revoked = false, receivedPassword;
  const server = createServer(async (req, res) => {
    assert.equal(req.headers.origin, `http://${req.headers.host}`);
    res.setHeader('Content-Type', 'application/json');
    if (req.url === '/api/v1/portal/login') {
      let raw = ''; for await (const chunk of req) raw += chunk;
      receivedPassword = JSON.parse(raw).password;
      res.setHeader('Set-Cookie', `emisell_portal_session=${token}; Path=/api/v1; HttpOnly; Max-Age=28800`);
      res.end(JSON.stringify({ user: { surface: 'developer' } })); return;
    }
    assert.equal(req.headers.cookie, `emisell_portal_session=${token}`);
    if (req.url === '/api/v1/developer/session') res.end(JSON.stringify({ user: { surface: 'developer' }, organization: { id: 'org1' } }));
    else if (req.url === '/api/v1/developer/logout') { revoked = true; res.end('{}'); }
    else res.end(JSON.stringify({ apps: [{ id: 'app1' }], limit: 200 }));
  });
  server.listen(0, '127.0.0.1'); await once(server, 'listening');
  t.after(() => { server.closeAllConnections(); server.close(); });
  const output = [], options = { store, output: text => output.push(text), password: async () => 'synthetic-password' };
  await run(['login', '--url', `http://127.0.0.1:${server.address().port}`, '--email', 'dev@example.invalid'], options);
  assert.equal(receivedPassword, 'synthetic-password');
  assert.ok(!(await readFile(join(store.directory, 'session.json'), 'utf8')).includes('synthetic-password'));
  await run(['whoami'], options);
  await run(['apps', 'list'], options);
  assert.equal(JSON.parse(output.at(-1)).apps[0].id, 'app1');
  await run(['logout'], options);
  assert.ok(revoked);
  await assert.rejects(store.load(), /Belum login/);
  assert.ok(!output.join('').includes(token));
});
test('non-developer login never persisted', async t => {
  const { store } = await fixture(t);
  await assert.rejects(run(['login', '--url', 'https://apps.example.com', '--email', 'a@example.invalid'], {
    store, password: async () => 'secret', fetcher: async () => json({ user: { surface: 'admin' } }, { 'set-cookie': `emisell_portal_session=${token}; Max-Age=100` }),
  }), /akun developer/);
  await assert.rejects(store.load(), /Belum login/);
});
test('mutations preserve revision and request key; submit requires explicit confirmation', async t => {
  const { directory, store } = await fixture(t);
  const file = join(directory, 'app.json'), calls = [];
  await run(['apps', 'init', '--file', file], { output() {} });
  await assert.rejects(run(['apps', 'init', '--file', file]), /EEXIST/);
  await store.save({ origin: 'https://apps.example.com', cookie: `emisell_portal_session=${token}`, expiresAt: Date.now() + 10000 });
  const opts = { store, output() {}, fetcher: async (url, init) => {
    calls.push({ url, ...init });
    return json(url.endsWith('/session') ? { user: { surface: 'developer' } } : { app: { id: 'a1' } });
  } };
  await run(['apps', 'create', '--file', file, '--request-key', 'create-001'], opts);
  assert.equal(JSON.parse(calls.at(-1).body).revision, 0);
  await run(['apps', 'update', 'a1', '--file', file, '--revision', '2', '--request-key', 'update-001'], opts);
  assert.equal(calls.at(-1).method, 'PUT');
  assert.equal(JSON.parse(calls.at(-1).body).revision, 2);
  assert.equal(calls.at(-1).headers['Idempotency-Key'], 'update-001');
  const length = calls.length;
  await assert.rejects(run(['reviews', 'submit', 'a1', '--revision', '2', '--request-key', 'submit-001'], opts), /--yes/);
  assert.equal(calls.length, length);
  await run(['reviews', 'submit', 'a1', '--revision', '2', '--request-key', 'submit-001', '--yes'], opts);
  assert.equal(calls.at(-1).url, 'https://apps.example.com/api/v1/developer/apps/a1/submissions');
});
test('errors are safe and requests never auto-retry or follow redirects', async () => {
  let count = 0;
  const client = new Client('https://apps.example.com', '', async (_url, options) => {
    count++; assert.equal(options.redirect, 'error');
    return new Response('private-secret', { status: 409 });
  });
  await assert.rejects(client.request('/api/v1/developer/apps', { method: 'POST', body: {} }), error => /409/.test(error.message) && !error.message.includes('private-secret'));
  assert.equal(count, 1);
  await assert.rejects(client.request('/api/v1/admin/apps'), /Path/);
  await assert.rejects(client.request('/api/v1/developer/../admin/apps'), /Path/);
});
test('bad options and identifiers fail before contacting server', async () => {
  const opts = { fetcher() { throw new Error('must not call'); } };
  for (const args of [ ['apps', 'show', '..'], ['apps', 'list', '--url', 'https://evil.example'], ['login', '--password', 'secret'], ['apps', 'update', 'a1', '--revision', '0', '--request-key', 'abc12345', '--file', 'x'], ['publish'] ]) {
    await assert.rejects(run(args, opts));
  }
});
test('local logout removes expired session without claiming server revocation', async t => {
  const { store } = await fixture(t);
  await store.save({ origin: 'https://apps.example.com', cookie: `emisell_portal_session=${token}`, expiresAt: 1 });
  let message;
  await run(['logout', '--local'], { store, fetcher() { assert.fail('must not contact server'); }, output(text) { message = text; } });
  assert.match(message, /tidak dicabut/);
  await assert.rejects(store.load(), /Belum login/);
});
test('malformed server response never prints response body', async () => {
  const client = new Client('https://apps.example.com', '', async () => new Response('secret-token', { headers: { 'content-type': 'application/json' } }));
  await assert.rejects(client.request('/api/v1/developer/session'), error => error.message === 'Respons JSON tidak valid.');
});
