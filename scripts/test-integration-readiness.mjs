// Only a run-owned, loopback, tmpfs PostgreSQL instance. Never reads .env.
import { randomBytes } from 'node:crypto';
import { spawn, spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';

const cwd = fileURLToPath(new URL('../services/app-gateway/', import.meta.url));
const adminLogin = process.argv.includes('--admin-login');
const runId = randomBytes(12).toString('hex');
const password = randomBytes(32).toString('hex');
const name = `${adminLogin ? 'emisell_admin_login_test_' : 'emisell_readiness_test_'}${runId}`;
const env = Object.fromEntries(['PATH', 'HOME', 'TMPDIR', 'GOPATH', 'GOCACHE', 'GOMODCACHE', 'GOROOT', 'DOCKER_HOST', 'DOCKER_CONTEXT'].filter((key) => process.env[key]).map((key) => [key, process.env[key]]));
let container, child, interrupted = false;
const redact = (text) => text.replaceAll(password, '[redacted]');
const run = (args, extra = {}) => {
  const result = spawnSync('docker', args, { env: { ...env, ...extra }, encoding: 'utf8', timeout: 30000 });
  if (result.error || result.status !== 0) throw new Error(redact(`Docker failed: ${result.stderr || result.error?.message}`));
  return result.stdout.trim();
};
const interrupt = () => { interrupted = true; child?.kill('SIGTERM'); };
process.once('SIGINT', interrupt); process.once('SIGTERM', interrupt);
try {
  container = run(['run', '--rm', '-d', '--label', `emisell.readiness-test=${runId}`, '--tmpfs', '/var/lib/postgresql/data:rw', '-p', '127.0.0.1::5432', '-e', 'POSTGRES_USER=readiness_test', '-e', 'POSTGRES_PASSWORD', '-e', `POSTGRES_DB=${name}`, 'postgres:17-alpine'], { POSTGRES_PASSWORD: password });
  if (!/^[a-f0-9]{64}$/.test(container)) throw new Error('Unexpected container identity');
  const bindings = JSON.parse(run(['inspect', '--format', '{{json .NetworkSettings.Ports}}', container]))['5432/tcp'];
  if (bindings.length !== 1 || bindings[0].HostIp !== '127.0.0.1') throw new Error('Database must bind loopback only');
  let ready = false;
  for (let attempt = 0; attempt < 80 && !interrupted; attempt++) {
    const check = spawnSync('docker', ['exec', container, 'pg_isready', '-U', 'readiness_test', '-d', name], { env, stdio: 'ignore', timeout: 5000 });
    if (check.status === 0) { ready = true; break; }
    await delay(250);
  }
  if (!ready || interrupted) throw new Error('Disposable database unavailable');
  console.log(adminLogin ? 'Testing admin login, session isolation, revocation and rate limits against disposable PostgreSQL 17.' : 'Testing inspection, persistence and publication preconditions against disposable PostgreSQL 17.');
  const code = await new Promise((resolve, reject) => {
    child = spawn('go', ['test', '-race', '-count=1', '-timeout=90s', './internal/adapters/postgres', '-run', adminLogin ? '^TestAdminLoginPostgres$' : '^TestIntegrationReadinessPostgres$', '-v'], { cwd, env: { ...env, [adminLogin ? 'ADMIN_LOGIN_TEST_DATABASE_URL' : 'INTEGRATION_READINESS_TEST_DATABASE_URL']: `postgresql://readiness_test:${password}@127.0.0.1:${bindings[0].HostPort}/${name}?sslmode=disable` }, stdio: ['ignore', 'pipe', 'pipe'] });
    let tail = '';
    for (const stream of [child.stdout, child.stderr]) stream.on('data', (data) => { tail = (tail + data.toString()).slice(-32000); });
    child.once('error', reject);
    child.once('exit', (code) => { console.log(redact(tail)); resolve(code); });
  });
  if (code !== 0 || interrupted) throw new Error('Integration inspection tests did not pass');
} finally {
  if (container && /^[a-f0-9]{64}$/.test(container)) {
    if (run(['inspect', '--format', '{{index .Config.Labels "emisell.readiness-test"}}', container]) !== runId) throw new Error('Refusing cleanup of an unowned container');
    run(['stop', '--time', '1', container]);
    console.log('Removed only this run’s disposable database; existing data was untouched.');
  }
}
