import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, realpath, readFile, writeFile, chmod, symlink, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { initApp } from '../src/development.mjs';
import { startTestServer as startDev } from './http-fixture.mjs';
import { productQuery, productPage } from '../templates/react-router/server/product-query.mjs';
import { run } from '../src/cli.mjs';

test('product queries and projected pages are bounded and reject authority overrides', () => {
  assert.deepEqual(productQuery(new URLSearchParams('q=Blue&cursor=page.sig&limit=2')), { q: 'Blue', cursor: 'page.sig', limit: 2 });
  for (const query of ['merchantId=other', 'limit=21', 'limit=01', 'q=', 'q=a&q=b', 'q=%00', 'q=%20a', `q=${'é'.repeat(51)}`, 'cursor=', 'cursor=bad', 'cursor='+'a'.repeat(1025)]) assert.throws(() => productQuery(new URLSearchParams(query)));
  assert.deepEqual(productPage({ data: [{ id: 'p', name: 'Blue', price: '1.25', secret: 'hidden' }], meta: { nextCursor: 'page.sig', hidden: 'hidden' } }, 5), { data: [{ id: 'p', name: 'Blue', price: '1.25' }], meta: { nextCursor: 'page.sig' } });
  for (const value of [{data:[],meta:{nextCursor:'bad'}}, {data:[],meta:{nextCursor:42}}, {data:[{id:'p',name:'Blue',price:'NaN'}]}]) assert.throws(() => productPage(value, 5));
});

test('generated products template serves UI only and forwards search/pagination after verification', async t => {
  const base = await realpath(await mkdtemp(join(tmpdir(), 'emisell-products-')));
  t.after(() => rm(base, { recursive: true, force: true }));
  const root = join(base, 'app');
  await run(['app', 'init', '--dir', root, '--parent-origin', 'https://seller.emisell.test', '--template', 'react-router'], { output(){} });
  assert.equal(JSON.parse(await readFile(join(root, 'emisell.app.json'), 'utf8')).template, 'react-router');
  assert.match(await readFile(join(root, 'README.md'), 'utf8'), /--release-kind ui_resource/);
  await assert.rejects(initApp(join(base, 'invalid'), 'http://localhost:3000', 'other'), /Template/);
  let active = true; const reads = [];
  const server = await startDev(root, 0, { verifySession: async () => {
    if (!active) throw Object.assign(Error('revoked'), { code: 'access_denied' });
    return { merchantId:'m', actorId:'a', appId:'app', installationId:'i', expiresAt:Math.floor(Date.now()/1000)+60 };
  }, readProducts: async (token, {query}) => { reads.push(query); return { data:[{id:'p',name:'Blue',price:'1000'}], meta:{nextCursor:'page.sig'} }; } });
  t.after(() => { server.closeAllConnections(); server.close(); });
  const url = `http://127.0.0.1:${server.address().port}`;
  for (const asset of ['/', '/products']) assert.equal((await fetch(url+asset)).status, 200);
  for (const file of ['/local-products-backend.mjs', '/server/local-products.mjs', '/client.secret', '/emisell.app.json']) assert.equal((await fetch(url+file)).status, 404);
  const send = query => fetch(url+'/api/products'+query, {method:'POST',headers:{Origin:url,Authorization:'Bearer abc.def.sig'}});
  assert.equal((await send('?q=Blue&limit=2&cursor=page.sig')).status, 200);
  assert.deepEqual(reads, [{q:'Blue',limit:2,cursor:'page.sig'}]);
  for (const query of ['?merchantId=other','?q=a&q=b','?limit=21']) assert.equal((await send(query)).status,400);
  active = false; assert.equal((await send('')).status,403); assert.equal(reads.length,1);
});

test('generated local product backend pins Core, rotates private secrets and never follows redirects', async t => {
  const base = await realpath(await mkdtemp(join(tmpdir(), 'emisell-reader-')));
  t.after(() => rm(base, { recursive:true, force:true }));
  const root = await initApp(join(base, 'app'), 'http://localhost:3000', 'react-router');
  const { createLocalProductReader } = await import(pathToFileURL(join(root,'server/local-products.mjs')));
  const appId = 'app_'+'A'.repeat(26), clientId = 'eac_'+'A'.repeat(26), secretFile = join(base,'client.secret');
  let secret = 'eacs_'+'s'.repeat(43), status=200, reads=0;
  await writeFile(secretFile,secret,{mode:0o600});
  const core = createServer((req,res) => {
    reads++; assert.equal(req.url,'/v1/products?limit=2&q=Blue&cursor=page.sig');
    assert.equal(req.headers['x-emisell-app-secret'],secret);
    assert.equal(req.headers['x-emisell-app-client'],clientId);
    assert.equal(req.headers.authorization,'Bearer abc.def.sig');
    assert.equal(req.headers.cookie,undefined); assert.equal(req.headers.origin,undefined);
    res.writeHead(status,{'Content-Type':'application/json','X-Emisell-App-Access':'resource-v1',Location:'https://example.invalid'});
    res.end(JSON.stringify({data:[{id:'p',name:'Blue',price:'1',secret:'hidden'}],meta:{nextCursor:'next.sig'}}));
  });
  core.listen(0,'127.0.0.1'); await once(core,'listening');
  t.after(() => {core.closeAllConnections();core.close();});
  const config={coreOrigin:`http://127.0.0.1:${core.address().port}`,appId,clientId,secretFile,publicDirectory:join(root,'public')};
  const reader=createLocalProductReader(config), options={query:{limit:2,q:'Blue',cursor:'page.sig'}};
  assert.equal((await reader('abc.def.sig',options)).meta.nextCursor,'next.sig');
  secret='eacs_'+'t'.repeat(43); await writeFile(secretFile,secret);
  assert.equal((await reader('abc.def.sig',options)).data[0].secret,undefined);
  status=302; await assert.rejects(reader('abc.def.sig',options),/verifier_unavailable/);
  status=403; await assert.rejects(reader('abc.def.sig',options),/access_denied/);
  await chmod(secretFile,0o644); await assert.rejects(reader('abc.def.sig',options),/verifier_not_configured/); await chmod(secretFile,0o600);
  const link=join(base,'link.secret'); await symlink(secretFile,link);
  await assert.rejects(createLocalProductReader({...config,secretFile:link})('abc.def.sig',options),/verifier_not_configured/);
  const publicSecret=join(root,'public','client.secret'); await writeFile(publicSecret,secret,{mode:0o600});
  await assert.rejects(createLocalProductReader({...config,secretFile:publicSecret})('abc.def.sig',options),/verifier_not_configured/);
  assert.throws(() => createLocalProductReader({...config,coreOrigin:'https://external.example'}));
  await assert.rejects(reader('abc.def.sig',{query:{merchantId:'other'}}),/invalid_query/);
  assert.equal(reads,4,'invalid config/query never sends a secret');
});
