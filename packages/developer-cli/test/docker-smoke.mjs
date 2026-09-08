// Opt-in Docker integration test. Creates only uniquely labeled disposable
// images/containers and synthetic data; no live service, account or database.
import assert from 'node:assert/strict';
import { mkdtemp, realpath, writeFile, mkdir, rm, readFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { randomUUID } from 'node:crypto';
import { spawn } from 'node:child_process';
import { request } from 'node:http';
import { initApp } from '../src/development.mjs';

const base = await realpath(await mkdtemp(join(tmpdir(), 'emisell-docker-')));
const root = await initApp(join(base, 'app'), 'http://localhost:3000', 'react-router', 'Docker Synthetic Test');
const id = 'emisell-cli-test-' + randomUUID();
const image = id + ':fixture';
const containers = [];
const canary = 'MUST_NOT_ENTER_IMAGE_' + randomUUID();
async function cmd(bin, args, { cwd = root, allowedFailure = false, timeout = 240000 } = {}) {
  return new Promise((resolve, reject) => {
    const child = spawn(bin, args, { cwd, stdio: ['ignore', 'pipe', 'pipe'] });
    let text = '';
    const keep = chunk => { text = (text + chunk).slice(-20000); };
    child.stdout.on('data', keep); child.stderr.on('data', keep);
    const timer = setTimeout(() => child.kill('SIGTERM'), timeout);
    child.once('error', error => { clearTimeout(timer); reject(error); });
    child.once('exit', (code, signal) => {
      clearTimeout(timer);
      if (code === 0 || allowedFailure) resolve({ code, text });
      else reject(Error(`${bin} ${args[0]} failed (${signal || code}): ${text.replaceAll(canary, '[canary]')}`));
    });
  });
}
const docker = (...args) => cmd('docker', args);
async function runContainer(suffix, environment, command = [], mounts = []) {
  const name = id + '-' + suffix; containers.push(name);
  await docker('run', '-d', '--name', name, '--label', 'emisell.cli.test=' + id,
    '--read-only', '--cap-drop', 'ALL', '--security-opt', 'no-new-privileges:true',
    '--tmpfs', '/tmp:size=64m,mode=1777', '-p', '127.0.0.1::' + environment.PORT,
    ...Object.entries(environment).flatMap(([key, value]) => ['-e', key + '=' + value]), ...mounts, image, ...command);
  const mapped = (await docker('port', name, environment.PORT + '/tcp')).text.trim();
  const url = 'http://' + mapped;
  const host = environment.EMISELL_APP_URL ? new URL(environment.EMISELL_APP_URL).host : '127.0.0.1:' + environment.PORT;
  async function check(path, status = 200, options = {}) {
    const result = await new Promise((resolve, reject) => {
      const req = request(url + path, { signal: AbortSignal.timeout(5000), ...options, headers: { Host: host, ...options.headers } }, res => {
        const chunks = []; res.on('data', chunk => chunks.push(chunk));
        res.on('end', () => resolve({ res, body: Buffer.concat(chunks).toString() }));
        res.on('error', reject);
      });
      req.on('error', reject); req.end();
    });
    assert.equal(result.res.statusCode, status, suffix + path + ': ' + result.body.slice(0, 100));
    assert.ok(!result.body.includes(canary)); return result;
  }
  let started = false, lastError;
  for (let i = 0; i < 80; i++) {
    try { await check('/health/live'); started = true; break; } catch (error) { lastError = error.message; await new Promise(resolve => setTimeout(resolve, 250)); }
  }
  assert.ok(started, suffix + ' did not start: ' + lastError + ' ' + (await docker('logs', name)).text);
  return { name, check };
}
try {
  await docker('version');
  await cmd('npm', ['install', '--package-lock-only', '--ignore-scripts', '--no-fund']);
  for (const file of ['.env', 'server/canary.secret', 'public/canary.key']) await writeFile(join(root, file), canary);
  await mkdir(join(root, '.local')); await writeFile(join(root, '.local/private.json'), canary);
  const previewConfig = JSON.parse((await docker('compose', 'config', '--format', 'json')).text);
  assert.equal(previewConfig.services.app.ports[0].host_ip, '127.0.0.1');
  assert.ok(previewConfig.services.app.read_only);
  console.log('Building a fresh generated project in Docker (typecheck + build + production dependencies)…');
  await docker('build', '--label', 'emisell.cli.test=' + id, '--target', 'runtime', '-t', image, '.');
  await docker('run', '--rm', '--entrypoint', 'node', image, '-e', `
    const fs = require('node:fs'), assert = require('node:assert/strict');
    assert.equal(process.getuid(), 1000);
    for (const p of ['.env','.local','server/canary.secret','public','node_modules/vite','node_modules/@react-router/dev']) assert.equal(fs.existsSync(p), false, p);
    assert.ok(fs.existsSync('build/server/index.js'));
  `);
  const built = await runContainer('preview', { PORT: '4330', EMISELL_BACKEND_MODE: 'disabled' }, ['node', 'server/standalone.mjs', '--built', '--container-preview']);
  const page = await built.check('/'); assert.match(page.body, /Container preview/);
  for (const match of page.body.matchAll(/(?:href|src)="(\/assets\/[^\"]+)"/g)) await built.check(match[1]);
  await built.check('/products'); await built.check('/health/ready');
  await built.check('/api/session', 503, { method: 'POST' });
  await docker('exec', built.name, 'node', 'server/healthcheck.mjs');
  const env = { PORT: '3000', EMISELL_APP_URL: 'https://app.example.com', EMISELL_DASHBOARD_ORIGIN: 'https://seller.example.com', EMISELL_TLS_TERMINATION: 'external' };
  const hosted = await runContainer('hosted-disabled', env);
  const prodPage = await hosted.check('/'); assert.match(prodPage.body, /Hosted app/);
  await hosted.check('/health/ready', 503);
  assert.equal((await cmd('docker', ['exec', hosted.name, 'node', 'server/healthcheck.mjs'], { allowedFailure: true })).code, 1);
  await hosted.check('/', 403, { headers: { Host: 'evil.example', 'X-Forwarded-Host': 'app.example.com' } });
  for (const path of ['/server/production-backend.mjs', '/dev-config.json', '/.env', '/@vite/client']) await hosted.check(path, 404);
  // Synthetic custom backend validates the deployment seam, not real Core grants.
  const backend = join(base, 'synthetic-backend.mjs');
  await writeFile(backend, `import { existsSync } from 'node:fs';
    export async function createBackend() { return {
      checkReady: async () => !existsSync('/tmp/down'),
      verifySession: async token => {
        if (token !== 'synthetic-token' || existsSync('/tmp/revoked')) throw Object.assign(Error(), {code:'access_denied'});
        return { merchantId:'synthetic-merchant', actorId:'synthetic-actor', appId:'synthetic-app', installationId:'synthetic-install', expiresAt:Math.floor(Date.now()/1000)+60 };
      },
      readProducts: async () => ({data:[{id:'synthetic-product',name:'Synthetic Product',price:'1000'}]})
    }; }
  `, { mode: 0o644 });
  const custom = await runContainer('custom', { ...env, EMISELL_BACKEND_MODE: 'custom' }, [], ['--mount', 'type=bind,source=' + backend + ',target=/app/server/production-backend.mjs,readonly']);
  await custom.check('/health/ready');
  const headers = { Origin: 'https://app.example.com', Authorization: 'Bearer synthetic-token' };
  assert.equal(JSON.parse((await custom.check('/api/products', 200, { method: 'POST', headers })).body).data[0].id, 'synthetic-product');
  await custom.check('/api/products', 403, { method: 'POST', headers: { ...headers, Origin: 'https://other.example' } });
  await custom.check('/api/products?merchantId=other', 400, { method: 'POST', headers });
  await docker('exec', custom.name, 'node', '-e', "require('fs').writeFileSync('/tmp/revoked','true')");
  await custom.check('/api/products', 403, { method: 'POST', headers });
  await docker('exec', custom.name, 'node', '-e', "require('fs').writeFileSync('/tmp/down','true')");
  await custom.check('/health/ready', 503);
  const bad = await cmd('docker', ['run', '--rm', '-e', 'EMISELL_BACKEND_MODE=custom', ...Object.entries(env).flatMap(([k,v]) => ['-e', k+'='+v]), image], { allowedFailure: true });
  assert.equal(bad.code, 1); assert.match(bad.text, /Adapter production belum siap/);
  for (const name of containers) {
    await docker('stop', '--time', '15', name);
    assert.equal((await docker('inspect', '--format', '{{.State.ExitCode}}', name)).text.trim(), '0');
  }
  console.log('Docker passed: fresh build, non-root/read-only runtime, no dev dependencies/secrets, UI/assets, health, exact Host/Origin, fail-closed and synthetic custom adapter, revocation, graceful shutdown.');
} finally {
  for (const name of containers) {
    const found = await cmd('docker', ['inspect', '--format', '{{index .Config.Labels "emisell.cli.test"}}', name], { allowedFailure: true });
    if (found.code === 0 && found.text.trim() === id) await cmd('docker', ['rm', '-f', name], { allowedFailure: true });
  }
  await cmd('docker', ['image', 'rm', image], { allowedFailure: true });
  await rm(base, { recursive: true, force: true });
}
