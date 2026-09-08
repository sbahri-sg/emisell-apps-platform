// Explicit integration suite: installs framework dependencies in a fresh project.
// No real account, Core service, merchant, database or credential is used.
import assert from 'node:assert/strict';
import { mkdtemp, mkdir, realpath, writeFile, readFile, readdir, rm, symlink } from 'node:fs/promises';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { pathToFileURL } from 'node:url';
import { spawn } from 'node:child_process';
import { createServer, request } from 'node:http';
import { once } from 'node:events';
import { initApp } from '../src/development.mjs';
import { inspectProject } from '../src/project.mjs';

const preview = process.argv.includes('--preview');
if (process.argv.slice(2).some(arg => arg !== '--preview')) throw Error('Use no arguments, or --preview for synthetic browser QA.');
const base = await realpath(await mkdtemp(join(tmpdir(), 'emisell-framework-')));
await mkdir(join(base, '.local'));
const root = await initApp(join(base, '.local/app'), 'http://127.0.0.1:4361', 'react-router', 'Emisell Framework Test');
let core, app, parent;
const servers = [];
const environment = { ...process.env };
for (const name of Object.keys(environment)) if (/^(EMISELL_LOCAL_|EMISELL_PUBLIC_|VITE_)/.test(name) || name === 'NODE_ENV') delete environment[name];
async function command(args) {
  await new Promise((resolve, reject) => {
    const child = spawn(process.platform === 'win32' ? 'npm.cmd' : 'npm', args, { cwd: root, env: environment, stdio: 'inherit' });
    const timer = setTimeout(() => child.kill('SIGTERM'), 120000);
    child.once('error', reject);
    child.once('exit', code => { clearTimeout(timer); code === 0 ? resolve() : reject(Error('npm ' + args.join(' ') + ' failed')); });
  });
}
async function stop(server) {
  server?.closeAllConnections();
  if (server?.listening) await new Promise(resolve => server.close(resolve));
}
const canary = 'private-framework-canary';
async function response(origin, path, status = 200, options) {
  const res = await fetch(origin + path, { signal: AbortSignal.timeout(15000), ...options });
  const body = await res.text();
  assert.equal(res.status, status, path + ' status');
  assert.ok(!body.includes(canary), path + ' leaked a private file');
  return { res, body };
}

try {
  await command(['install', '--no-fund']);
  await command(['run', 'typecheck']);
  const doctor = await inspectProject(root);
  assert.equal(doctor.readyForLocalPreview, true, JSON.stringify(doctor));

  let revoked = false;
  const appId = 'app_' + 'A'.repeat(26), clientId = 'eac_' + 'B'.repeat(26);
  const secret = 'eacs_' + 'C'.repeat(43), token = 'fixture.identity.token';
  const products = Array.from({ length: 7 }, (_, i) => ({ id: 'product-' + i, name: 'Produk Demo ' + (i + 1), price: String(10000 + i) }));
  core = createServer((req, res) => {
    if (revoked || req.headers.authorization !== 'Bearer ' + token || req.headers['x-emisell-app-client'] !== clientId) {
      res.writeHead(403); res.end(); return;
    }
    const url = new URL(req.url, 'http://fixture.invalid');
    res.setHeader('Content-Type', 'application/json');
    res.setHeader('X-Emisell-App-Access', 'resource-v1');
    if (url.pathname.endsWith('/identity')) {
      res.end(JSON.stringify({ merchantId: 'fixture-merchant', actorId: 'fixture-actor', installationId: 'fixture-installation', appId, clientId, expiresAt: Math.floor(Date.now()/1000) + 60 }));
    } else if (req.headers['x-emisell-app-secret'] === secret && (url.searchParams.get('view') === 'inventory' || url.pathname.startsWith('/v1/settings/location'))) {
      const inventory=url.searchParams.get('view')==='inventory';
      assert.ok(inventory ? /^\/v1\/products(?:\/[^/]+)?$/.test(url.pathname) : /^\/v1\/settings\/location(?:\/[^/]+)?$/.test(url.pathname));
      const basePath=inventory?'/v1/products':'/v1/settings/location',detailId=url.pathname.slice(basePath.length+1);
      const rows=Array.from({length:7},(_,i)=>inventory?
        {id:'product-'+i,trackInventory:true,continueSellingWhenOutOfStock:false,stock:-2,items:[{id:'product-'+i,type:'PRODUCT',stock:-2,levels:[{locationId:'location-0',available:-2}]}]}:
        {id:'location-'+i,name:'Warehouse '+i,isActive:i!==6,isPrimary:i===0,isPhysicalStorefront:false,isFulfillment:true,shippingIsActive:true,deliveryIsActive:false,pickUpIsActive:false});
      if(detailId)res.end(JSON.stringify({data:rows.find(row=>row.id===detailId)}));
      else {const filtered=rows.filter(row=>inventory||row.name.includes(url.searchParams.get('q')||'')),offset=url.searchParams.has('cursor')?5:0;
        res.end(JSON.stringify({data:filtered.slice(offset,offset+5),meta:{nextCursor:filtered.length>offset+5?'stock-next.signature':null}}));}
    } else if (url.pathname.endsWith('/products') && req.headers['x-emisell-app-secret'] === secret) {
      const q = url.searchParams.get('q') || '';
      const rows = products.filter(p => p.name.toLowerCase().includes(q.toLowerCase()));
      const index = url.searchParams.get('cursor') ? 5 : 0;
      res.end(JSON.stringify({ data: rows.slice(index, index + 5), meta: { nextCursor: rows.length > index + 5 ? 'fixture-page2.signature' : null } }));
    } else if (req.headers['x-emisell-app-secret'] === secret && url.pathname === '/v1/orders') {
      res.end(JSON.stringify({data:[{id:'order-fixture',numberFormat:'#1',status:'PAID',subtotal:'10000',currency:'IDR'}],meta:{nextCursor:null}}));
    } else if (req.headers['x-emisell-app-secret'] === secret && url.pathname === '/v1/settings/shipping') {
      res.end(JSON.stringify({data:{id:'shipping-fixture',splitShipping:false,estimatedShow:'OFF',profiles:[{id:'profile-fixture',name:'General',isPrimary:true}]},meta:{nextCursor:null}}));
    } else if (req.headers['x-emisell-app-secret'] === secret && /^\/v1\/(catalogs|collections)(?:\/[^/]+)?$/.test(url.pathname)) {
      const [, ,kind,detailId]=url.pathname.split('/');
      const rows=Array.from({length:7},(_,i)=>({id:kind+'-'+i,...(kind==='catalogs'?{title:'Demo '+i}:{name:'Demo '+i}),productCount:1}));
      if(detailId){const row=rows.find(v=>v.id===detailId);res.end(JSON.stringify({data:{...row,products:[{productId:'product-0',...(kind==='catalogs'?{variantId:null}:{})}]}}));}
      else { const filtered=rows.filter(row=>(row.title||row.name).includes(url.searchParams.get('q')||'')),offset=url.searchParams.has('cursor')?5:0;
        res.end(JSON.stringify({data:filtered.slice(offset,offset+5),meta:{nextCursor:filtered.length>offset+5?'groups-next.signature':null}})); }
    } else { res.writeHead(403); res.end(); }
  }).listen(0, '127.0.0.1');
  await once(core, 'listening'); servers.push(core);
  const secretPath = join(base, 'fixture.secret');
  await writeFile(secretPath, secret, { mode: 0o600 });
  await writeFile(join(root, '.env'), [
    'EMISELL_LOCAL_CORE_ORIGIN=http://127.0.0.1:' + core.address().port,
    'EMISELL_LOCAL_APP_ID=' + appId, 'EMISELL_LOCAL_CLIENT_ID=' + clientId,
    'EMISELL_LOCAL_CLIENT_SECRET_FILE=' + secretPath, 'EMISELL_PUBLIC_CANARY=' + canary,
  ].join('\n'), { mode: 0o600 });
  await writeFile(join(root, 'server/private.txt'), canary);
  await writeFile(join(root, 'app/private.server.ts'), 'export const secret = "' + canary + '";');
  await command(['run', 'build']);
  for (const file of await readdir(join(root, 'build/client/assets'))) {
    const content = await readFile(join(root, 'build/client/assets', file), 'utf8');
    assert.ok(!content.includes(canary) && !content.includes(secret), 'No private env or backend source in build');
  }
  await symlink(join(root, 'server/private.txt'), join(root, 'public/leak.txt'));
  await symlink(join(root, 'server/private.txt'), join(root, 'app/leak.txt'));
  const { start } = await import(pathToFileURL(join(root, 'server/dev.mjs')));
  const { createBackend } = await import(pathToFileURL(join(root, 'server/backend.mjs')));

  for (const built of [true, false]) {
    app = await start({ root, port: preview && !built ? 4360 : 0, built, adapters: await createBackend(root, {}) });
    servers.push(app);
    const origin = 'http://127.0.0.1:' + app.address().port;
    for (const route of ['/', '/products', '/resources/orders', '/resources/shipping', '/resources/catalogs', '/resources/collections', '/resources/inventory', '/resources/locations']) {
      const { res, body } = await response(origin, route);
      assert.ok(body.includes('Emisell Framework Test'));
      assert.ok(!body.includes('rendering aborted'));
      const nonce = /nonce-([^']+)/.exec(res.headers.get('content-security-policy'))?.[1];
      assert.ok(nonce);
      for (const script of body.matchAll(/<script([^>]*)>/g)) assert.ok(script[1].includes('nonce="' + nonce + '"'));
      assert.ok(body.includes('streamController.enqueue'));
      if (built) {
        for (const match of body.matchAll(/(?:href|src)="(\/assets\/[^"]+)"/g)) await response(origin, match[1]);
      }
    }
    await response(origin, '/favicon.svg');
    for (const path of ['/.env', '/.env?raw', '/server/private.txt', '/app/private.server.ts?raw', '/app/leak.txt',
      '/leak.txt', '/@fs/' + root + '/.env', '/@id/__x00__virtual:react-router/server-build',
      '/emisell.app.json', '/package.json', '/%252eenv', '/node_modules/.vite/.env']) {
      await response(origin, path, 404);
    }
    const headers = { Origin: origin, Authorization: 'Bearer ' + token };
    await response(origin, '/api/session', 200, { method: 'POST', headers });
    const first = JSON.parse((await response(origin, '/api/products', 200, { method: 'POST', headers })).body);
    assert.equal(first.data.length, 5); assert.equal(first.meta.nextCursor, 'fixture-page2.signature');
    const next = JSON.parse((await response(origin, '/api/products?cursor=fixture-page2.signature', 200, { method: 'POST', headers })).body);
    assert.equal(next.data.length, 2);
    const search = JSON.parse((await response(origin, '/api/products?q=Demo+3', 200, { method: 'POST', headers })).body);
    assert.equal(search.data[0].name, 'Produk Demo 3');
    for (const path of ['/v1/orders','/v1/settings/shipping','/v1/catalogs','/v1/collections']) {
      await response(origin,path,200,{method:'POST',headers});
      await response(origin,path+'?merchantId=other',400,{method:'POST',headers});
      await response(origin,path,403,{method:'POST',headers:{...headers,Origin:'http://evil.invalid'}});
    }
    for(const kind of ['catalogs','collections']) {
      const result=JSON.parse((await response(origin,`/v1/${kind}?q=2`,200,{method:'POST',headers})).body);
      assert.equal(result.data[0].id,kind+'-2');assert.equal(result.data.length,1);
      const next=JSON.parse((await response(origin,`/v1/${kind}?cursor=groups-next.signature`,200,{method:'POST',headers})).body);
      assert.equal(next.data.length,2);assert.equal(next.meta.nextCursor,null);
      const detail=JSON.parse((await response(origin,`/v1/${kind}/${kind}-0`,200,{method:'POST',headers})).body);
      assert.equal(detail.data.products[0].productId,'product-0');
    }
    for(const kind of ['inventory','locations']) {
      const path=kind==='inventory'?'/v1/products':'/v1/settings/location',selector=kind==='inventory'?'view=inventory&':'';
      const first=JSON.parse((await response(origin,path+'?'+selector+'limit=5',200,{method:'POST',headers})).body);
      assert.equal(first.data.length,5);assert.equal(first.meta.nextCursor,'stock-next.signature');
      const next=JSON.parse((await response(origin,path+'?'+selector+'cursor=stock-next.signature',200,{method:'POST',headers})).body);
      assert.equal(next.data.length,2);assert.equal(next.meta.nextCursor,null);
      const detail=JSON.parse((await response(origin,path+'/'+first.data[0].id+'?'+selector,200,{method:'POST',headers})).body);
      assert.equal(detail.data.id,first.data[0].id);
      if(kind==='inventory')assert.equal(detail.data.stock,-2);
      else {const search=JSON.parse((await response(origin,path+'?q=Warehouse+2',200,{method:'POST',headers})).body);assert.equal(search.data.length,1);assert.equal(search.data[0].id,'location-2');}
      await response(origin,path+'?'+selector+'merchantId=other',400,{method:'POST',headers});
      await response(origin,path+'?'+selector,403,{method:'POST',headers:{...headers,Origin:'http://evil.invalid'}});
    }
    await response(origin, '/api/products?merchantId=other', 400, { method: 'POST', headers });
    await response(origin, '/api/products', 401, { method: 'POST', headers: { Origin: origin } });
    await response(origin, '/api/products', 403, { method: 'POST', headers: { ...headers, Origin: 'http://evil.invalid' } });
    await response(origin, '/api/products', 403, { method: 'POST', headers, body: '{}' });
    revoked = true;
    await response(origin, '/api/products', 403, { method: 'POST', headers });
    await response(origin, '/v1/orders', 403, { method: 'POST', headers });
    await response(origin, '/v1/settings/shipping', 403, { method: 'POST', headers });
    await response(origin, '/v1/catalogs', 403, { method: 'POST', headers });
    await response(origin, '/v1/collections', 403, { method: 'POST', headers });
    await response(origin, '/v1/products?view=inventory', 403, { method: 'POST', headers });
    await response(origin, '/v1/settings/location', 403, { method: 'POST', headers });
    revoked = false;
    if (!built) {
      const paths = ['/app/entry.client.tsx', '/app/root.tsx', '/app/routes/home.tsx', '/app/routes/products.tsx',
        '/app/lib/emisell.ts', '/app/lib/bridge.mjs', '/app/styles/app.css', '/@vite/client',
        '/@id/__x00__virtual:react-router/inject-hmr-runtime', '/@id/__x00__virtual:react-router/hmr-runtime'];
      for (const path of paths) {
        const { body } = await response(origin, path);
        // Follow browser dependency-cache imports too: SSR alone is not a working app.
        for (const match of body.matchAll(/"(\/node_modules\/\.vite\/deps\/[^"]+)"/g)) await response(origin, match[1]);
      }
      const client = (await response(origin, '/@vite/client')).body;
      const wsToken = /const wsToken = "([^"]+)"/.exec(client)?.[1];
      assert.ok(wsToken, 'Vite websocket token found');
      const ws = await new Promise((resolve, reject) => {
        const req = request(origin + '/?token=' + wsToken, { headers: {
          Origin: origin, Connection: 'Upgrade', Upgrade: 'websocket', 'Sec-WebSocket-Version': '13',
          'Sec-WebSocket-Key': Buffer.alloc(16, 1).toString('base64'), 'Sec-WebSocket-Protocol': 'vite-hmr',
        } });
        const timer = setTimeout(() => req.destroy(Error('HMR handshake timeout')), 5000);
        req.once('upgrade', (_res, socket) => { clearTimeout(timer); resolve(socket); });
        req.once('error', error => { clearTimeout(timer); reject(error); });
        req.once('response', res => { clearTimeout(timer); res.resume(); reject(Error('HMR upgrade rejected')); });
        req.end();
      });
      const updated = new Promise((resolve, reject) => {
        const timer = setTimeout(() => { ws.destroy(); reject(Error('No HMR event')); }, 10000);
        let data = '';
        // No permessage-deflate requested: test frames carry readable JSON.
        ws.on('data', chunk => {
          data = (data + chunk.toString()).slice(-65536);
          if (data.includes('"type":"update"') || data.includes('"type":"full-reload"')) { clearTimeout(timer); resolve(); }
        });
      });
      const css = join(root, 'app/styles/app.css');
      await writeFile(css, (await readFile(css, 'utf8')) + '\n/* Framework HMR smoke */\n');
      await updated; ws.destroy();
    }
    console.log('PASS: ' + (built ? 'built preview' : 'Vite dev/HMR') + ', session/product reads, search/cursor, revoked grant, private files and CSP.');
    if (built || !preview) await stop(app);
  }
  if (preview) {
    const bridge = await readFile(join(root, 'app/lib/bridge.mjs'));
    parent = createServer((req, res) => {
      res.setHeader('Cache-Control', 'no-store');
      if (req.url === '/bridge.mjs') { res.setHeader('Content-Type', 'text/javascript'); res.end(bridge); return; }
      if (req.url === '/revoke' && req.method === 'POST') { revoked = true; res.writeHead(204); res.end(); return; }
      res.setHeader('Content-Type', 'text/html');
      res.end('<!doctype html><html lang="id"><title>Emisell synthetic framework QA</title><body style="margin:0"><p>Uji sintetis lokal — bukan toko asli. <button id="revoke">Cabut akses fixture</button></p><div id="app"></div><script type="module">import {mountEmbeddedApp} from "/bridge.mjs"; mountEmbeddedApp({container:document.querySelector("#app"),getSession:async()=>({launch:{mode:"embedded",url:"http://127.0.0.1:4360/",parentOrigin:"http://127.0.0.1:4361"},identityToken:"fixture.identity.token",expiresIn:60})});document.querySelector("#revoke").onclick=()=>fetch("/revoke",{method:"POST"});</script></body></html>');
    }).listen(4361, '127.0.0.1');
    await once(parent, 'listening'); servers.push(parent);
    console.log('Synthetic browser QA: http://127.0.0.1:4361 — Project: ' + root);
    await new Promise(resolve => { process.once('SIGINT', resolve); process.once('SIGTERM', resolve); });
  }
} finally {
  for (const server of servers.reverse()) await stop(server);
  await rm(base, { recursive: true, force: true });
}
