// Invoked explicitly by the Go/Postgres integration test, not node --test.
import assert from 'node:assert/strict';
import { pathToFileURL } from 'node:url';
import { join } from 'node:path';
import { writeFile } from 'node:fs/promises';

if (process.argv[2]) {
  const root = process.env.EMISELL_CLI_TEST_PACKAGE_DIR;
  const base = root ? pathToFileURL(join(root, 'package.json')) : new URL('../package.json', import.meta.url);
  const { run } = await import(new URL('./src/cli.mjs', base));
  const { SessionStore } = await import(new URL('./src/client.mjs', base));
  let raw = ''; for await (const chunk of process.stdin) raw += chunk;
  const credentials = JSON.parse(raw);
  const store = new SessionStore(join(process.argv[3], 'session'));
  const output = [];
  const options = {
    store, output: value => output.push(value), wait:async()=>{},
    open:async value=>{
      const request=new URL(value).searchParams.get('request');
      const approval=await fetch(process.argv[2]+'/api/v1/developer-login/approve',{method:'POST',headers:{'Content-Type':'application/json',Authorization:`Bearer ${credentials.coreKey}`},body:JSON.stringify({request,subject:'cli-contract-owner',email:credentials.email,name:'CLI Test Owner',stores:[{id:credentials.merchant,commonId:'cli-store',name:'CLI Test Store'}]})});
      assert.equal(approval.status,200);
      const exchange=new URL((await approval.json()).exchangeUrl);
      const finished=await fetch(process.argv[2]+exchange.pathname+exchange.search,{headers:{Host:'localhost:4317'},redirect:'manual'});
      assert.equal(finished.status,303);
      const confirmation=new URL(finished.headers.get('location'));
      const page=await fetch(process.argv[2]+confirmation.pathname+confirmation.search,{headers:{Host:'localhost:4317'}});
      assert.equal(page.status,200);assert.match(await page.text(),/Izinkan login/);
      const confirm=await fetch(process.argv[2]+confirmation.pathname,{method:'POST',headers:{Host:'localhost:4317',Origin:'http://localhost:4317','Content-Type':'application/x-www-form-urlencoded'},body:confirmation.searchParams});
      assert.equal(confirm.status,200);
    },
    // Test-only routing: preserve the real configured portal Origin while using an ephemeral listener.
    fetcher: (url, init) => fetch(process.argv[2] + new URL(url).pathname, {...init,headers:{...init.headers,Host:'localhost:4317'}}),
  };
  await run(['login', '--url', 'http://localhost:4317'], options);
  await run(['whoami'], options);
  assert.equal(JSON.parse(output.at(-1)).user.email, credentials.email.toLowerCase());
  await run(['scopes'], options);
  await run(['testing', 'list'], options);
  assert.ok(Array.isArray(JSON.parse(output.at(-1)).assignments));
  await assert.rejects(run(['testing', 'show', 'missing_cli_assignment'], options), /404/);
  const file = join(process.argv[3], 'app.json');
  const doc = { name: 'CLI test', summary: 'Synthetic test', description: 'Isolated CLI contract test.', version: '1.0.0', capability: 'shipping/v1', scopes: ['orders.read', 'shipping.read', 'shipping.write'], endpoint: 'https://example.invalid/shipping' };
  await writeFile(file, JSON.stringify(doc));
  await run(['apps', 'create', '--file', file, '--request-key', 'cli-create-001'], options);
  const app = JSON.parse(output.at(-1)).app;
  assert.equal(app.revision, 1);
  await run(['apps', 'create', '--file', file, '--request-key', 'cli-create-001'], options);
  assert.equal(JSON.parse(output.at(-1)).app.id, app.id);
  await run(['apps', 'list'], options);
  assert.ok(JSON.parse(output.at(-1)).apps.some(a => a.id === app.id));
  await run(['apps', 'show', app.id], options);
  doc.name = 'CLI updated'; await writeFile(file, JSON.stringify(doc));
  await run(['apps', 'update', app.id, '--file', file, '--revision', '1', '--request-key', 'cli-update-001'], options);
  assert.equal(JSON.parse(output.at(-1)).app.revision, 2);
  await assert.rejects(run(['apps', 'update', app.id, '--file', file, '--revision', '1', '--request-key', 'cli-stale-001'], options), /409/);
  await run(['reviews', 'submit', app.id, '--revision', '2', '--request-key', 'cli-review-001', '--yes'], options);
  const submission = JSON.parse(output.at(-1)).submission;
  await run(['reviews', 'show', submission.id], options);
  await run(['reviews', 'list'], options);
  await run(['logout'], options);
  await assert.rejects(run(['apps', 'list'], options), /Belum login/);
  assert.ok(!output.join('').includes(credentials.coreKey));
  console.log('CLI real backend contract passed');
}
