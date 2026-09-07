import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm, readFile, writeFile, symlink, unlink } from 'node:fs/promises';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { get } from 'node:http';
import { initApp, startDev } from '../src/development.mjs';
import { run } from '../src/cli.mjs';

test('UI creation uses reviewed UI endpoint, explicit consent and no business scopes', async t => {
  const dir = await mkdtemp(join(tmpdir(), 'emisell-ui-cli-'));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const file = join(dir, 'ui.json');
  const document = { name:'Demo', summary:'Local test', version:'0.1.0', mode:'embedded', url:'https://app.example.com/', reason:'Test' };
  await writeFile(file, JSON.stringify(document));
  const calls = [];
  const options = { store:{load:async()=>({origin:'https://portal.example.com',cookie:'emisell_portal_session=test'})},output(){},fetcher:async(url,init)=>{
    calls.push({url,...init});
    return new Response(JSON.stringify(url.endsWith('/session')?{user:{surface:'developer'}}:{releases:[]}),{headers:{'content-type':'application/json'}});
  }};
  const args = ['ui','create','--file',file,'--request-key','ui-test-001'];
  await assert.rejects(run(args,options),/--yes/);
  assert.equal(calls.length,0);
  await run([...args,'--yes'],options);
  assert.equal(calls.at(-1).url,'https://portal.example.com/api/v1/developer/ui-releases');
  assert.deepEqual(JSON.parse(calls.at(-1).body),document);
  await writeFile(file, JSON.stringify({...document,scopes:['orders.read']}));
  await assert.rejects(run([...args,'--yes'],options),/Dokumen UI/);
  await writeFile(file, JSON.stringify({...document,requiredScopes:['read_products']}));
  await run(['resource-ui','create','--file',file,'--request-key','resource-ui-test-001','--yes'],options);
  assert.match(calls.at(-1).url,/\/ui-resource-releases$/);
  assert.deepEqual(JSON.parse(calls.at(-1).body).requiredScopes,['read_products']);
  await writeFile(file, JSON.stringify({...document,requiredScopes:['read_orders']}));
  await assert.rejects(run(['resource-ui','create','--file',file,'--request-key','resource-ui-test-002','--yes'],options),/read_products/);
  await run(['ui','list'],options);
  await run(['ui','show','release_test'],options);
  assert.match(calls.at(-1).url,/ui-releases\/release_test$/);
});

test('local HTTPS origin and proof are explicit, bounded and not forwarded-header driven', async t => {
  const base = await mkdtemp(join(tmpdir(), 'emisell-https-test-'));
  t.after(() => rm(base, { force: true, recursive: true }));
  const project = await initApp(join(base, 'app'), 'https://seller.emisell.test');
  const configPath = join(project, 'emisell.app.json');
  const config = JSON.parse(await readFile(configPath, 'utf8'));
  config.appOrigin = 'https://app.emisell.test';
  config.endpointProof = { id: 'proof_ABCDEFGHIJKLMNOPQRSTUVWXYZ', document: { schema: 'test', clientId: 'test', releaseSha256: 'a', challenge: 'nonce' } };
  await writeFile(configPath, JSON.stringify(config));
  const server = await startDev(project, 0, { verifySession: async () => ({ merchantId: 'test', actorId: 'test', appId: 'test', installationId: 'test', expiresAt: Math.floor(Date.now()/1000)+60 }) });
  t.after(() => { server.closeAllConnections(); server.close(); });
  const url = `http://127.0.0.1:${server.address().port}`;
  const path = '/.well-known/emisell-app-verification/' + config.endpointProof.id;
  assert.deepEqual(await (await fetch(url+path)).json(), config.endpointProof.document);
  assert.equal((await fetch(url+path+'?x=1')).status, 404);
  for (const origin of ['https://evil.test', url]) {
    assert.equal((await fetch(url+'/api/session', { method:'POST', headers:{ Origin:origin, Authorization:'Bearer test', 'X-Forwarded-Host':'app.emisell.test' } })).status, 403);
  }
  assert.equal((await fetch(url+'/api/session', { method:'POST', headers:{ Origin:config.appOrigin, Authorization:'Bearer test' } })).status, 200);
  config.appOrigin = 'https://remote.example';
  await writeFile(configPath, JSON.stringify(config));
  await assert.rejects(startDev(project, 0), /origin/);
});

test('starter uses pinned source UI/bridge, never overwrites a project', async t => {
  const base = await mkdtemp(join(tmpdir(), 'emisell-starter-test-'));
  t.after(() => rm(base, { force: true, recursive: true }));
  const project = join(base, 'app');
  await initApp(project, 'http://localhost:3000');
  await assert.rejects(initApp(project, 'http://localhost:3000'), /EEXIST/);
  await assert.rejects(initApp(join(base, 'bad'), 'http://remote.example'), /HTTPS/);
  for (const [asset, source] of [['emisell-ui.css', '../../../pkg/appui/emisell-ui.css'], ['bridge.mjs', '../../../pkg/embedded/bridge.mjs']]) {
    const canonical = new URL(source, import.meta.url);
    assert.equal(await readFile(join(project, 'public', asset), 'utf8'), await readFile(canonical, 'utf8'));
  }
});
test('preview serves only starter assets, no credentials or fake backend success', async t => {
  const base = await mkdtemp(join(tmpdir(), 'emisell-dev-test-'));
  t.after(() => rm(base, { force: true, recursive: true }));
  const project = await initApp(join(base, 'app'), 'http://localhost:3000');
  const server = await startDev(project, 0);
  t.after(() => { server.closeAllConnections(); server.close(); });
  const url = `http://127.0.0.1:${server.address().port}`;
  const res = await fetch(url);
  assert.equal(res.status, 200);
  assert.match(res.headers.get('content-security-policy'), /frame-ancestors http:\/\/localhost:3000;/);
  assert.match(await res.text(), /eui-page/);
  for (const path of ['/emisell.app.json', '/.env', '/README.md', '/%2e%2e/package.json', '/node_modules/x', '/index.html?secret=x']) assert.equal((await fetch(url + path)).status, 404);
  const badHost = await new Promise((resolve, reject) => get(url, { headers: { Host: 'evil.example' } }, res => { res.resume(); resolve(res.statusCode); }).on('error', reject));
  assert.equal(badHost, 403);
  assert.equal((await fetch(url, { method: 'POST' })).status, 405);
  const denied = await fetch(url + '/api/session', { method: 'POST', headers: { Authorization: 'Bearer synthetic' } });
  assert.equal(denied.status, 503);
  assert.equal((await denied.json()).error, 'backend_identity_verifier_not_configured');
  assert.deepEqual(await (await fetch(url + '/dev-config.json')).json(), { parentOrigin: 'http://localhost:3000' });
  await unlink(join(project, 'public', 'app.mjs'));
  await symlink(join(project, 'emisell.app.json'), join(project, 'public', 'app.mjs'));
  assert.equal((await fetch(url + '/app.mjs')).status, 404);
});
test('testing command uses existing UI assignment contract and requires consent to request', async () => {
  const calls = [];
  const options = { store: { load: async () => ({ origin: 'https://apps.example.com', cookie: 'emisell_portal_session=test' }) }, output() {}, fetcher: async (url, init) => {
    calls.push({ url, ...init });
    return new Response(JSON.stringify(url.endsWith('/session') ? { user: { surface: 'developer' } } : { assignments: [], nextAfterId: '' }), { headers: { 'content-type': 'application/json' } });
  } };
  const args = ['testing', 'request', '--release-id', 'ui1', '--merchant-id', 'merchant1', '--reason', 'test', '--request-key', 'testing-001'];
  await assert.rejects(run(args, options), /--yes/);
  assert.equal(calls.length, 0);
  await run([...args, '--yes'], options);
  assert.deepEqual(JSON.parse(calls.at(-1).body), { releaseKind: 'ui', releaseId: 'ui1', merchantId: 'merchant1', reason: 'test' });
  assert.equal(calls.at(-1).headers['Idempotency-Key'], 'testing-001');
  await run([...args, '--release-kind', 'ui_resource', '--yes'], options);
  assert.equal(JSON.parse(calls.at(-1).body).releaseKind, 'ui_resource');
  const before = calls.length;
  await assert.rejects(run([...args, '--release-kind', 'invalid', '--yes'], options), /Release kind/);
  assert.equal(calls.length, before);
  await run(['testing', 'list', '--after-id', 'assign1'], options);
  assert.match(calls.at(-1).url, /test-assignments\?afterId=assign1$/);
  await run(['testing', 'show', 'assign1'], options);
  assert.match(calls.at(-1).url, /test-assignments\/assign1$/);
  await assert.rejects(run(['testing', 'approve', 'assign1'], options), /tidak dikenal/);
});
