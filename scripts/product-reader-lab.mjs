// Local-only lab. All mutable data and credentials belong to this one disposable run.
import { generateKeyPairSync, randomBytes, randomUUID } from 'node:crypto';
import { spawn, spawnSync } from 'node:child_process';
import { cpSync, existsSync, mkdirSync, mkdtempSync, rmSync, symlinkSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';

const root = fileURLToPath(new URL('../', import.meta.url));
const backend = process.argv.find((arg, index) => index > 1 && !arg.startsWith('--'));
if (!backend || !existsSync(join(resolve(backend), 'tests/fixtures/app-platform-resource-server.mjs'))) {
  throw new Error('Pass the api-service checkout: npm run example:lab -- /absolute/path/to/api-service');
}
const apiService = resolve(backend);
const check = process.argv.includes('--check');
const runId = randomBytes(12).toString('hex');
const workspace = mkdtempSync(join(tmpdir(), 'emisell-product-reader-'));
const baseEnv = Object.fromEntries(['PATH', 'HOME', 'TMPDIR', 'GOPATH', 'GOCACHE', 'GOMODCACHE', 'GOROOT', 'DOCKER_HOST', 'DOCKER_CONTEXT'].filter((key) => process.env[key]).map((key) => [key, process.env[key]]));
const containers = [], children = [], secrets = [];
let stopping = false;
let stopRequested = false, resolveStop;
const requestStop = () => { stopRequested = true; resolveStop?.(); };
process.once('SIGINT', requestStop);
process.once('SIGTERM', requestStop);
const checkStop = () => { if (stopRequested) throw new Error('Local lab interrupted'); };
const secret = () => { const value = randomBytes(32).toString('hex'); secrets.push(value); return value; };
const redact = (text) => secrets.reduce((output, value) => output.replaceAll(value, '[redacted]'), text);
const privateFile = (name, data) => { const path = join(workspace, name); writeFileSync(path, data, { mode: 0o600 }); return path; };
function run(program, args, env = baseEnv, cwd = root) {
  const result = spawnSync(program, args, { cwd, env, encoding: 'utf8', timeout: 120000, maxBuffer: 16*1024*1024 });
  if (result.error || result.status !== 0) throw new Error(`${program} failed: ${redact(`${result.stdout || ''}${result.stderr || ''}`)}`);
  return (result.stdout || '').trim();
}
function start(name, program, args, env, cwd) {
  checkStop();
  const child = spawn(program, args, { env, cwd, stdio: ['ignore', 'pipe', 'pipe'] });
  child.labName = name; child.tail = ''; children.push(child);
  for (const stream of [child.stdout, child.stderr]) stream.on('data', (data) => {
    child.tail = (child.tail + redact(data.toString())).slice(-12000);
    if (!check && name === 'Product Reader') {
      for (const line of data.toString().split('\n')) {
        const route = line.match(/Product Reader local request: ((GET|POST) \/[^?\s]*)$/)?.[1];
        if (route) console.log(`Product Reader: ${route}`);
      }
    }
  });
  return child;
}
async function stop(child) {
  if (!child || child.exitCode !== null || child.signalCode !== null) return;
  child.kill('SIGTERM');
  for (let attempt = 0; attempt < 40 && child.exitCode === null && child.signalCode === null; attempt++) await delay(100);
  if (child.exitCode === null && child.signalCode === null) child.kill('SIGKILL');
  await delay(100);
}
async function ready(url, child) {
  for (let attempt = 0; attempt < 100; attempt++) {
    checkStop();
    if (child && (child.exitCode !== null || child.signalCode !== null)) throw new Error(`${child.labName} stopped: ${child.tail}`);
    try { const response = await fetch(url, { signal: AbortSignal.timeout(1000) }); await response.body?.cancel(); if (response.ok) return; } catch {}
    await delay(200);
  }
  throw new Error(`Local service did not become ready: ${url}${child ? '\n'+child.tail : ''}`);
}
async function database(user, name) {
  checkStop();
  const password = secret();
  const container = run('docker', ['run', '--rm', '-d', '--label', `emisell.product-reader-lab=${runId}`,
    '--tmpfs', '/var/lib/postgresql/data:rw', '-p', '127.0.0.1::5432', '-e', `POSTGRES_USER=${user}`,
    '-e', 'POSTGRES_PASSWORD', '-e', `POSTGRES_DB=${name}`, 'postgres:17-alpine'], { ...baseEnv, POSTGRES_PASSWORD: password });
  if (!/^[a-f0-9]{64}$/.test(container)) throw new Error('Unexpected test container identity');
  containers.push(container);
  const binding = JSON.parse(run('docker', ['inspect', '--format', '{{json .NetworkSettings.Ports}}', container]))['5432/tcp'];
  if (binding.length !== 1 || binding[0].HostIp !== '127.0.0.1') throw new Error('Database did not bind loopback');
  for (let attempt = 0; attempt < 80; attempt++) {
    checkStop();
    const result = spawnSync('docker', ['exec', container, 'pg_isready', '-U', user, '-d', name], { env: baseEnv, stdio: 'ignore', timeout: 5000 });
    if (result.status === 0) return `postgresql://${user}:${password}@127.0.0.1:${binding[0].HostPort}/${name}?sslmode=disable`;
    await delay(250);
  }
  throw new Error('Disposable database did not start');
}
async function cleanup() {
  if (stopping) return; stopping = true;
  for (const child of [...children].reverse()) await stop(child);
  for (const container of containers) {
    const label = run('docker', ['inspect', '--format', '{{index .Config.Labels "emisell.product-reader-lab"}}', container]);
    if (label !== runId) throw new Error('Refusing cleanup of an unowned container');
    run('docker', ['stop', '--time', '1', container]);
  }
  // workspace was created by mkdtemp in this process and contains only this lab's data.
  rmSync(workspace, { recursive: true, force: true });
  console.log('Lab stopped. Removed its two test databases and generated example credentials; existing services were not changed.');
}

let gatewayProcess, readerProcess;
try {
  console.log('Preparing two isolated PostgreSQL databases and local services.');
  const resourceDB = await database('resource_test', `emisell_resource_test_${runId}`);
  const gatewayDB = await database('gateway_test', `emisell_gateway_test_${runId}`);
  const fixtureEnv = { ...baseEnv, NODE_ENV: 'test', DATABASE_URL: resourceDB, DATABASE_URL_REPLICA_1: '',
    RESOURCE_TEST_DATABASE_URL: resourceDB, RESOURCE_TEST_DATABASE_NAME: `emisell_resource_test_${runId}`,
    CHECKPOINT_DISABLE: '1', PRISMA_HIDE_UPDATE_MESSAGE: 'true' };
  run(process.execPath, ['node_modules/prisma/build/index.js', 'db', 'push', '--skip-generate', '--schema', 'prisma/models'], fixtureEnv, apiService);
  run(process.execPath, ['tests/fixtures/app-platform-resource-server.mjs', '--seed'], fixtureEnv, apiService);
  const { publicKey, privateKey } = generateKeyPairSync('rsa', { modulusLength: 2048 });
  const signingKey = privateFile('resource-private.pem', privateKey.export({ type: 'pkcs8', format: 'pem' }));
  const backendProcess = start('Resource backend', process.execPath, ['tests/fixtures/app-platform-resource-server.mjs'],
    { ...fixtureEnv, RESOURCE_TEST_PUBLIC_KEY: publicKey.export({ type: 'spki', format: 'pem' }) }, apiService);
  let resourceURL;
  for (let attempt = 0; attempt < 100; attempt++) { resourceURL = backendProcess.tail.match(/^http:\/\/127\.0\.0\.1:\d+/m)?.[0]; if (resourceURL) break; if (backendProcess.exitCode !== null) throw new Error('Resource backend failed'); await delay(100); }
  if (!resourceURL) throw new Error('Resource backend did not report its loopback URL');
  const port = Number(process.env.PRODUCT_READER_LAB_PORT_BASE || '3013');
  if (!Number.isInteger(port) || port < 1024 || port > 65000) throw new Error('Invalid lab port base');
  const frontendURL = `http://localhost:${port}`, readerURL = `http://localhost:${port+1}`, gatewayURL = `http://localhost:${port+2}`;
  const organizationId = '01995f72-0000-7000-8000-000000000001';
  const developerToken = secret();
  const cookiePrefix = `reader_lab_${runId.slice(0,8)}`;
  const gatewayEnv = { ...baseEnv, APP_ENV: 'development', HTTP_ADDRESS: `127.0.0.1:${port+2}`,
    FRONTEND_URL: frontendURL, PUBLIC_GATEWAY_URL: gatewayURL, CORS_ALLOWED_ORIGINS: frontendURL,
    REPOSITORY_DRIVER: 'postgres', DATABASE_URL: gatewayDB, DEVELOPMENT_BEARER_TOKEN: developerToken,
    DEVELOPMENT_ORGANIZATION_ID: organizationId, DEVELOPMENT_PLATFORM_OPERATOR: 'true',
    SECRET_ENCRYPTION_KEY_BASE64: randomBytes(32).toString('base64'), EMISELL_BACKEND_DEVELOPMENT_TOKEN: secret(),
    EMISELL_RESOURCE_ENABLED: 'true', EMISELL_RESOURCE_BASE_URL: resourceURL, EMISELL_RESOURCE_KEY_ID: 'contract-test',
    EMISELL_RESOURCE_PRIVATE_KEY_FILE: signingKey, EMISELL_RESOURCE_TEST_MERCHANT_IDS: 'merchant-a,merchant-b', DEVELOPMENT_LOOPBACK_APP_HTTP: 'true',
    AUTH_SESSION_COOKIE_NAME: `${cookiePrefix}_developer`, AUTH_CSRF_COOKIE_NAME: `${cookiePrefix}_csrf`,
    MERCHANT_SESSION_COOKIE_NAME: `${cookiePrefix}_merchant`, MERCHANT_CSRF_COOKIE_NAME: `${cookiePrefix}_merchant_csrf` };
  const gatewayCwd = join(root, 'services/app-gateway');
  const gatewayBinary = join(workspace, 'app-gateway'), migrateBinary = join(workspace, 'migrate'), readerBinary = join(workspace, 'product-reader');
  run('go', ['build', '-o', gatewayBinary, './cmd/server'], baseEnv, gatewayCwd);
  run('go', ['build', '-o', migrateBinary, './cmd/migrate'], baseEnv, gatewayCwd);
  run('go', ['build', '-o', readerBinary, '.'], baseEnv, join(root, 'examples/product-reader'));
  run(migrateBinary, [], gatewayEnv, gatewayCwd);
  const startGateway = () => start('App Gateway', gatewayBinary, [], gatewayEnv, gatewayCwd);
  gatewayProcess = startGateway(); await ready(`${gatewayURL}/readyz`, gatewayProcess);
  const management = async (method, path, body) => {
    const response = await fetch(gatewayURL+path, { method, headers: { Authorization: `Bearer ${developerToken}`, 'X-Organization-ID': organizationId,
      'Idempotency-Key': randomUUID(), 'Content-Type': 'application/json' }, ...(body === undefined ? {} : { body: JSON.stringify(body) }) });
    if (!response.ok) throw new Error(`Lab management ${method} ${path}: ${response.status}`);
    return response.status === 204 ? null : (await response.json()).data;
  };
  const app = await management('POST', '/v1/apps', { name: 'Product Reader Local Lab', distribution: 'custom', appUrl: readerURL+'/' });
  await management('PUT', `/v1/apps/${app.id}/scopes`, { scopes: [{ scope: 'read_products', access: 'required' }] });
  const credential = await management('POST', `/v1/apps/${app.id}/credentials`, { environment: 'sandbox' });
  secrets.push(credential.clientSecret);
  const version = await management('POST', `/v1/apps/${app.id}/versions`, { version: '0.1.0' });
  await management('POST', `/v1/apps/${app.id}/versions/${version.id}/release`, {});
  const launches = {};
  for (const merchantId of ['merchant-a', 'merchant-b']) {
    const login = await fetch(gatewayURL+'/auth/sandbox-merchant-login', { method: 'POST', headers: { 'Content-Type': 'application/json', Origin: frontendURL },
      body: JSON.stringify({ merchantId, merchantName: merchantId === 'merchant-a' ? 'Merchant A · Local Lab' : 'Merchant B · Local Lab' }) });
    if (!login.ok) throw new Error(`Cannot register synthetic merchant: ${login.status}`);
    await login.body?.cancel();
    launches[merchantId] = (await management('POST', `/v1/apps/${app.id}/test-install-requests`, { merchantId })).launchUrl;
  }
  const readerConfig = { AppID: app.id, ClientID: credential.credential.clientId, ClientSecret: credential.clientSecret,
    PublicURL: readerURL, GatewayURL: gatewayURL, ConsentURL: frontendURL+'/install', ConnectedAppsURL: frontendURL+'/merchant/apps',
    StoreFile: join(workspace, 'reader-store.enc'), EncryptionKeyFile: privateFile('reader-encryption-key', randomBytes(32)),
    ListenAddress: `127.0.0.1:${port+1}`, LocalDevelopment: true };
  const readerConfigFile = privateFile('reader-config.json', JSON.stringify(readerConfig));
  const startReader = () => start('Product Reader', readerBinary, [], { ...baseEnv, PRODUCT_READER_CONFIG_FILE: readerConfigFile }, workspace);
  readerProcess = startReader(); await ready(readerURL+'/', readerProcess);
  const lab = { root, apiService, workspace, runId, frontendURL, readerURL, gatewayURL, organizationId, developerToken,
    appId: app.id, launches, merchantCookie: gatewayEnv.MERCHANT_SESSION_COOKIE_NAME, merchantCSRFCookie: gatewayEnv.MERCHANT_CSRF_COOKIE_NAME };
  const labFile = privateFile('lab.json', JSON.stringify(lab));
  if (check) {
    const { smoke } = await import('./product-reader-smoke.mjs');
    await smoke(lab, async () => { await stop(readerProcess); await stop(gatewayProcess); gatewayProcess = startGateway(); await ready(gatewayURL+'/readyz', gatewayProcess); readerProcess = startReader(); await ready(readerURL+'/', readerProcess); });
  } else {
    // Separate checkout copy prevents build/env/cookie collisions with the user's running dashboard.
    const frontendDir = join(workspace, 'frontend'); mkdirSync(frontendDir, { mode: 0o700 });
    for (const name of ['app', 'lib', 'public', 'docs', '.openai', 'package.json', 'package-lock.json', 'tsconfig.json', 'next-env.d.ts', 'vite.config.ts', 'next.config.ts', 'proxy.ts']) {
      if (existsSync(join(root, name))) cpSync(join(root, name), join(frontendDir, name), { recursive: true });
    }
    symlinkSync(join(root, 'node_modules'), join(frontendDir, 'node_modules'), 'dir');
    const frontendProcess = start('Lab frontend', process.execPath, [join(root, 'node_modules/vinext/dist/cli.js'), 'dev', '--host', '127.0.0.1', '--port', String(port)],
      { ...baseEnv, NEXT_PUBLIC_APP_GATEWAY_URL: gatewayURL, APP_GATEWAY_INTERNAL_URL: gatewayURL, NEXT_PUBLIC_SITE_URL: frontendURL,
        NEXT_PUBLIC_APP_GATEWAY_TOKEN: '', NEXT_PUBLIC_ORGANIZATION_ID: '', NEXT_PUBLIC_ENABLE_DEVELOPMENT_LOGIN: 'true',
        AUTH_SESSION_COOKIE_NAME: gatewayEnv.AUTH_SESSION_COOKIE_NAME, NEXT_PUBLIC_AUTH_CSRF_COOKIE_NAME: gatewayEnv.AUTH_CSRF_COOKIE_NAME,
        NEXT_PUBLIC_MERCHANT_CSRF_COOKIE_NAME: gatewayEnv.MERCHANT_CSRF_COOKIE_NAME, NEXT_PUBLIC_SANDBOX_MERCHANT_ID: 'merchant-a' }, frontendDir);
    await ready(frontendURL+'/install', frontendProcess);
    console.log(`Product Reader: ${readerURL}/\nDeveloper Console (isolated): ${frontendURL}/apps/${app.id}\nConnected Apps (isolated): ${frontendURL}/merchant/apps\nPrivate lab state: ${labFile}`);
    console.log('Choose a test-install launch link in the isolated Developer Console. Ctrl+C stops and removes this lab only.');
    await new Promise((done) => { resolveStop = done; if (stopRequested) done(); });
  }
} catch (error) {
  if (!stopRequested) throw error;
} finally {
  await cleanup();
  process.off('SIGINT', requestStop);
  process.off('SIGTERM', requestStop);
}
