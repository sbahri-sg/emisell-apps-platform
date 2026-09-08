import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm, readFile, writeFile, readdir, unlink, symlink } from 'node:fs/promises';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { run } from '../src/cli.mjs';
import { initApp, startDev } from '../src/development.mjs';
import { findProject, readProject } from '../src/project.mjs';

const bin = fileURLToPath(new URL('../bin/emisell.mjs', import.meta.url));
async function fixture(t) {
  const cwd = await mkdtemp(join(tmpdir(), 'emisell-workflow-'));
  t.after(() => rm(cwd, { recursive: true, force: true }));
  const messages = [];
  return { cwd, messages, options: { cwd, interactive: false, output: value => messages.push(value),
    fetcher() { assert.fail('local command must not contact a server'); },
    store: { load() { assert.fail('local command must not load credentials'); } } } };
}

test('interactive init collects name, destination, template and trusted parent before creating files', async t => {
  const { cwd, messages, options } = await fixture(t);
  const answers = ['My Product App', 'my-product-app', '2', 'https://seller.emisell.test'];
  const questions = [];
  await run(['app', 'init'], { ...options, interactive: true, prompt: async question => {
    assert.deepEqual(await readdir(cwd), []);
    questions.push(question); return answers.shift();
  } });
  assert.equal(questions.length, 4);
  const config = await readProject(join(cwd, 'my-product-app'));
  assert.equal(config.name, 'My Product App');
  assert.equal(config.template, 'products');
  assert.equal(config.parentOrigin, 'https://seller.emisell.test');
  assert.match(messages[0], /emisell app doctor/);
  assert.match(messages[0], /Belum terhubung ke toko/);
});

test('missing noninteractive options, cancellation and invalid input never create a project', async t => {
  const { cwd, options } = await fixture(t);
  await assert.rejects(run(['app', 'init'], options), /non-interaktif/);
  await assert.rejects(run(['app', 'init'], { ...options, interactive: true, prompt: async () => { throw Error('cancelled'); } }), /cancelled/);
  for (const extra of [['--name', 'bad\nname'], ['--template', 'remote'], ['--path', ''], ['--parent-origin', 'http://remote.example']]) {
    const args = ['app', 'init', '--path', 'new-app', '--parent-origin', 'http://localhost:3000'];
    const index = args.indexOf(extra[0]); if (index >= 0) args.splice(index, 2);
    await assert.rejects(run([...args, ...extra], options));
  }
  assert.deepEqual(await readdir(cwd), []);
});

test('name/path flags, short aliases and legacy --dir remain noninteractive and never overwrite', async t => {
  const { cwd, options } = await fixture(t);
  await run(['app', 'init', '-n', 'New App', '-p', 'new-app', '--parent-origin', 'http://localhost:3000'], options);
  assert.equal((await readProject(join(cwd, 'new-app'))).name, 'New App');
  await run(['app', 'init', '--dir', 'old-app', '--parent-origin', 'http://localhost:3000'], options);
  assert.equal((await readProject(join(cwd, 'old-app'))).template, 'embedded');
  await run(['app', 'init', '--name', 'Name Only', '--parent-origin', 'http://localhost:3000'], options);
  assert.equal((await readProject(join(cwd, 'name-only'))).name, 'Name Only');
  const before = await readFile(join(cwd, 'old-app', 'emisell.app.json'), 'utf8');
  await assert.rejects(run(['app', 'init', '--dir', 'old-app', '--parent-origin', 'http://localhost:3000'], options), /EEXIST/);
  assert.equal(await readFile(join(cwd, 'old-app', 'emisell.app.json'), 'utf8'), before);
  await assert.rejects(run(['app', 'init', '--dir', 'a', '--path', 'b', '--parent-origin', 'http://localhost:3000'], options), /bukan keduanya/);
});

test('info and doctor find project from subdirectories without exposing secrets or executing backend', async t => {
  const { cwd, options, messages } = await fixture(t);
  const project = await initApp(join(cwd, 'app'), 'http://localhost:3000', 'products');
  const file = join(project, 'emisell.app.json');
  const config = JSON.parse(await readFile(file));
  const secret = 'private-canary-must-not-be-shown';
  await writeFile(file, JSON.stringify({ ...config, secret, backend: './trap.mjs', endpointProof: {
    id: 'proof_' + 'A'.repeat(26), document: { schema: 'test', clientId: 'test', releaseSha256: 'a', challenge: secret },
  } }));
  await writeFile(join(project, '.env'), 'SECRET=' + secret);
  await writeFile(join(project, 'trap.mjs'), 'throw Error("Backend must not execute");');
  const code = await run(['app', 'doctor', '--json'], { ...options, cwd: join(project, 'server') });
  assert.equal(code, 0);
  const report = JSON.parse(messages.at(-1));
  assert.equal(report.readyForLocalPreview, true);
  assert.equal(report.storeAccess, 'not-checked');
  await run(['app', 'info', '-j'], { ...options, cwd: join(project, 'public') });
  const info = JSON.parse(messages.at(-1));
  assert.deepEqual(info.templateScopes, ['read_products']);
  assert.equal(info.endpointProofConfigured, true);
  assert.equal(info.storeAccess, 'not-checked');
  assert.equal(messages.join('').includes(secret), false);
  assert.equal(messages.join('').includes('trap.mjs'), false);
  await assert.rejects(run(['app', 'info', '--path', join(project, 'server')], options), /Project tidak ditemukan/);
});

test('doctor reports malformed, symlinked and oversized configs safely; discovery never bypasses a child config', async t => {
  const { cwd, options, messages } = await fixture(t);
  const project = await initApp(join(cwd, 'app'), 'http://localhost:3000');
  const child = join(project, 'server');
  const file = join(child, 'emisell.app.json');
  await writeFile(file, '{"secret":"secret-canary"');
  assert.equal(await findProject(child), child);
  assert.equal(await run(['app', 'doctor', '--path', child, '--json'], options), 1);
  assert.equal(JSON.parse(messages.at(-1)).readyForLocalPreview, false);
  assert.equal(messages.at(-1).includes('secret-canary'), false);
  await unlink(file); await symlink(join(project, 'emisell.app.json'), file);
  await assert.rejects(readProject(child), /symlink/);
  assert.equal(await run(['app', 'doctor', '--path', child], options), 1);
  await unlink(file); await writeFile(file, 'x'.repeat(16385));
  await assert.rejects(readProject(child), /16 KiB/);
  await assert.rejects(startDev(child, 0), /16 KiB/);
});

test('legacy configs and asset errors are checked consistently without granting access', async t => {
  const { cwd, options, messages } = await fixture(t);
  const project = await initApp(join(cwd, 'app'), 'http://localhost:3000');
  const file = join(project, 'emisell.app.json');
  const config = JSON.parse(await readFile(file));
  delete config.name; delete config.template;
  await writeFile(file, JSON.stringify(config));
  assert.equal((await readProject(project)).template, 'embedded');
  assert.equal(await run(['app', 'doctor', '--dir', project, '--json'], options), 0);
  await unlink(join(project, 'public', 'app.mjs'));
  assert.equal(await run(['app', 'doctor', '--dir', project, '--json'], options), 1);
  assert.equal(JSON.parse(messages.at(-1)).checks.find(check => check.id === 'assets').status, 'error');
  await symlink(join(project, 'emisell.app.json'), join(project, 'public', 'app.mjs'));
  assert.equal(await run(['app', 'doctor', '--dir', project], options), 1);
});

test('dev works from project cwd/subdirectory and preserves explicit backend paths without autoload', async t => {
  const { cwd, options, messages } = await fixture(t);
  const project = await initApp(join(cwd, 'app'), 'http://localhost:3000');
  await writeFile(join(project, 'backend.mjs'), 'throw Error("Must not auto load");');
  await writeFile(join(cwd, 'explicit.mjs'), 'export async function verifySession() { return {}; }');
  for (const backend of [false, true]) {
    let server, read;
    const code = await run(['app', 'dev', ...(backend ? ['--path', 'app', '--backend', './explicit.mjs'] : [])], {
      ...options, cwd: backend ? cwd : join(project, 'public'),
      startPreview: async (root, port, adapters) => {
        assert.equal(root, project); assert.equal(port, 4330);
        assert.equal(typeof adapters.verifySession, backend ? 'function' : 'undefined');
        server = await startDev(root, 0, adapters);
        t.after(() => { server.closeAllConnections(); server.close(); });
        return server;
      },
      output: value => {
        messages.push(value);
        read = fetch(`http://127.0.0.1:${server.address().port}`).then(async res => {
          assert.equal(res.status, 200); await res.arrayBuffer();
        }).finally(() => { server.closeAllConnections(); server.close(); });
      },
    });
    await read;
    assert.equal(code, 0);
  }
  assert.match(messages[0], /akses data ditolak/);
});

test('occupied port gets actionable error and existing preview stays running', async t => {
  const { cwd } = await fixture(t);
  const project = await initApp(join(cwd, 'app'), 'http://localhost:3000');
  const server = await startDev(project, 0);
  t.after(() => { server.closeAllConnections(); server.close(); });
  const port = server.address().port;
  await assert.rejects(startDev(project, port), /sedang digunakan.*--port/);
  const response = await fetch(`http://127.0.0.1:${port}`);
  assert.equal(response.status, 200); await response.arrayBuffer();
});

test('help is local, unknown/duplicate options are rejected and deployment remains unavailable', async t => {
  const { options, messages } = await fixture(t);
  for (const command of ['init', 'dev', 'info', 'doctor']) {
    await run(['app', command, '-h'], options);
    assert.match(messages.at(-1), new RegExp('emisell app ' + command));
  }
  await run(['auth', 'login', '--help'], options);
  assert.match(messages.at(-1), /login browser\/OAuth belum tersedia/);
  for (const args of [ ['app', 'dev', '--store', 'a'], ['app', 'info', '-p', 'a', '--path', 'b'], ['app', 'init', '-n', 'a', '--name', 'b'] ]) {
    await assert.rejects(run(args, options), /Opsi/);
  }
  await assert.rejects(run(['app', 'deploy'], options), /belum tersedia/);
});

test('auth login/logout aliases retain the existing developer-only session flow', async () => {
  let saved, cleared = false;
  const opts = { output() {}, password: async () => 'synthetic-password',
    store: { save: async session => { saved = session; }, load: async () => saved, clear: async () => { cleared = true; } },
    fetcher: async () => new Response(JSON.stringify({ user: { surface: 'developer' } }), { headers: {
      'content-type': 'application/json', 'set-cookie': 'emisell_portal_session=' + 'a'.repeat(52) + '; Max-Age=100',
    } }),
  };
  await run(['auth', 'login', '--url', 'https://portal.example.com', '--email', 'dev@example.invalid'], opts);
  assert.equal(saved.origin, 'https://portal.example.com');
  await run(['auth', 'logout'], opts);
  assert.equal(cleared, true);
});

test('real CLI doctor emits JSON and a failing exit code without credentials or hangs', async t => {
  const { cwd } = await fixture(t);
  const result = spawnSync(process.execPath, [bin, 'app', 'doctor', '--json'], { cwd, encoding: 'utf8', timeout: 5000 });
  assert.equal(result.status, 1);
  assert.equal(JSON.parse(result.stdout).readyForLocalPreview, false);
  assert.equal(result.stderr, '');
  const init = spawnSync(process.execPath, [bin, 'app', 'init'], { cwd, encoding: 'utf8', timeout: 5000 });
  assert.equal(init.status, 1);
  assert.match(init.stderr, /non-interaktif/);
  assert.deepEqual(await readdir(cwd), []);
});
