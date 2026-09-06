import {readFileSync} from 'node:fs';
import {runInNewContext} from 'node:vm';
import {test} from 'node:test';
import assert from 'node:assert/strict';

const source = readFileSync(new URL('./web/app.js', import.meta.url), 'utf8').replace("import {getIdentity} from '/bridge.mjs';", '');
const settle = () => new Promise(resolve => setImmediate(resolve));
function harness() {
  let now = 100000, issues = 0, code = 200, issuanceFails = false, checks = 0;
  const elements = new Map(), events = new Map();
  const element = selector => {
    if (!elements.has(selector)) elements.set(selector, {textContent:'', dataset:{}, setAttribute(){}, removeAttribute(){}, addEventListener(name, fn){events.set(selector+name, fn);}});
    return elements.get(selector);
  };
  let tick;
  const listener = {addEventListener(name, fn){events.set(name, fn);}, removeEventListener(name){events.delete(name);}};
  const document = {...listener, querySelector:element, hidden:false};
  runInNewContext(source, {
    document, window:listener, location:{pathname:'/seller'},
    Date:class extends Date {static now(){return now;}}, AbortSignal,
    setInterval(fn){tick = fn; return 1;}, clearInterval(){tick = () => {};},
    getIdentity:async () => {issues++; if (issuanceFails) throw Error('timeout'); return {token:'test'};},
    fetch:async () => {checks++; return {ok:code===200, status:code, json:async () => ({merchantId:'merchant',actorId:'actor',appId:'app',installationId:'installation',expiresAt:(now+60000)/1000})};},
  });
  return {element, events, document, get checks(){return checks;}, get issues(){return issues;}, set code(v){code=v;}, set issuanceFails(v){issuanceFails=v;}, async step(ms=5000){now+=ms; tick(); await settle();}};
}
test('visible expiry renews without manual click and denied access clears identity', async () => {
  const h = harness(); await settle();
  assert.equal(h.element('#status').textContent, 'Terhubung');
  await h.step(61000);
  assert.equal(h.issues, 2);
  h.code = 403; await h.step(45000);
  assert.equal(h.issues, 3);
  assert.equal(h.element('#status').textContent, 'Akses ditolak');
  assert.equal(h.element('#merchant').textContent, '—');
});
test('outage clears identity, retries finitely and recovers when online', async () => {
  const h = harness(); await settle(); h.code = 503;
  await h.step(45000);
  assert.equal(h.element('#status').textContent, 'Koneksi belum tersedia');
  assert.equal(h.element('#merchant').textContent, '—');
  for(let i=0;i<5;i++) await h.step(20000);
  const attempts = h.issues;
  await h.step(60000); assert.equal(h.issues, attempts);
  h.code = 200; h.events.get('online')(); await settle();
  assert.equal(h.element('#status').textContent, 'Terhubung');
});
test('bridge timeout is not called revoked; auto retry and pagehide cleanup', async () => {
  const h = harness(); await settle(); h.issuanceFails = true;
  await h.step(61000);
  assert.equal(h.element('#status').textContent, 'Koneksi belum tersedia');
  h.issuanceFails = false; await h.step();
  assert.equal(h.element('#status').textContent, 'Terhubung');
  h.events.get('pagehide')(); const attempts = h.issues;
  await h.step(61000); assert.equal(h.issues, attempts);
  assert.equal(h.element('#merchant').textContent, '—');
});
test('fresh token is not polled and hidden tab makes no requests until resumed', async () => {
  const h = harness(); await settle();
  for (let i=0;i<8;i++) await h.step();
  assert.equal(h.issues, 1); assert.equal(h.checks, 1);
  h.document.hidden = true; h.events.get('visibilitychange')();
  for (let i=0;i<20;i++) await h.step(60000);
  h.events.get('online')(); await settle();
  assert.equal(h.issues, 1); assert.equal(h.checks, 1);
  h.document.hidden = false; h.events.get('visibilitychange')(); await settle();
  assert.equal(h.issues, 2); assert.equal(h.checks, 2);
  assert.equal(h.element('#status').textContent, 'Terhubung');
});
test('renewal does not clear displayed identity while awaiting response', async () => {
  const h = harness(); await settle();
  h.events.get('#refreshclick')();
  assert.equal(h.element('#merchant').textContent, 'merchant');
  assert.equal(h.element('#status').textContent, 'Terhubung');
  await settle();
});
