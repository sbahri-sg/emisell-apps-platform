import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import { readFileSync } from 'node:fs';

// Real HTTP forms/cookies + merchant consent APIs. Browser UI verification is separate.
export async function smoke(lab, restart) {
  function client() {
    const cookies = new Map();
    return { cookies, async request(url, options = {}) {
      const response = await fetch(url, { ...options, redirect: 'manual', headers: { Cookie: [...cookies].map(([key,value]) => `${key}=${value}`).join('; '), ...options.headers } });
      for (const header of response.headers.getSetCookie()) { const [pair] = header.split(';'); const at = pair.indexOf('='); cookies.set(pair.slice(0,at), pair.slice(at+1)); }
      const text = await response.text(); return { status: response.status, text, location: response.headers.get('location'), json: () => JSON.parse(text) };
    } };
  }
  const providerA = client(), providerB = client();
  async function authorize(provider, merchant, launch) {
    assert.equal((await provider.request(launch)).status, 303);
    const home = await provider.request(lab.readerURL+'/');
    const csrf = home.text.match(/name="csrf" value="([^"]+)"/)[1];
    const connect = await provider.request(lab.readerURL+'/connect', { method: 'POST', headers: { Origin: lab.readerURL, 'Content-Type': 'application/x-www-form-urlencoded' }, body: new URLSearchParams({ csrf }) });
    assert.equal(connect.status, 303);
    const consent = new URL(connect.location);
    const input = { clientId: consent.searchParams.get('client_id'), redirectUri: consent.searchParams.get('redirect_uri'), state: consent.searchParams.get('state'),
      codeChallenge: consent.searchParams.get('code_challenge'), requestedScopes: ['read_products'], testInstallRequestId: consent.searchParams.get('test_install_request') };
    const merchantBrowser = client();
    const login = await merchantBrowser.request(lab.gatewayURL+'/auth/sandbox-merchant-login', { method: 'POST', headers: { Origin: lab.frontendURL, 'Content-Type': 'application/json' },
      body: JSON.stringify({ merchantId: merchant, merchantName: `Lab ${merchant}` }) });
    assert.equal(login.status, 201);
    const headers = { 'Content-Type': 'application/json', Origin: lab.frontendURL, 'X-CSRF-Token': merchantBrowser.cookies.get(lab.merchantCSRFCookie) };
    const missingSession = await client().request(lab.gatewayURL+'/v1/merchant/oauth/preview', { method: 'POST', headers, body: JSON.stringify(input) });
    assert.equal(missingSession.status, 401);
    const preview = await merchantBrowser.request(lab.gatewayURL+'/v1/merchant/oauth/preview', { method: 'POST', headers, body: JSON.stringify(input) });
    assert.equal(preview.status, 200); assert.equal(preview.json().data.developmentInstall, true);
    const granted = await merchantBrowser.request(lab.gatewayURL+'/v1/merchant/oauth/authorize', { method: 'POST', headers, body: JSON.stringify({ ...input, grantedScopes: ['read_products'] }) });
    assert.equal(granted.status, 201);
    const callback = granted.json().data.redirectTo;
    const foreignBrowser = await client().request(callback); assert.equal(foreignBrowser.status, 400);
    const badState = new URL(callback); badState.searchParams.set('state', 'z'.repeat(43));
    assert.equal((await provider.request(badState)).status, 400);
    const connected = await provider.request(callback); assert.equal(connected.status, 303); assert.equal(connected.location, '/products');
    assert.equal((await provider.request(callback)).status, 400);
    const list = await merchantBrowser.request(lab.gatewayURL+'/v1/merchant/installations');
    const installation = list.json().data.find((entry) => entry.appId === lab.appId || entry.app?.id === lab.appId);
    assert.ok(installation, 'installation persisted in gateway PostgreSQL');
    return { merchantBrowser, headers, installation };
  }
  const a = await authorize(providerA, 'merchant-a', lab.launches['merchant-a']);
  await authorize(providerB, 'merchant-b', lab.launches['merchant-b']);
  const products = async (provider, expected) => {
    const response = await provider.request(lab.readerURL+'/api/products'); assert.equal(response.status, 200);
    assert.deepEqual(response.json().data.map((p) => p.id), expected);
    assert.doesNotMatch(response.text, /es_at_|clientSecret|code_verifier/);
  };
  await products(providerA, ['product-a2','product-a1']); await products(providerB, ['product-b1']);
  assert.equal((await providerA.request(lab.readerURL+'/api/products/product-b1')).status, 404);
  assert.equal((await providerB.request(lab.readerURL+'/api/products/product-a1')).status, 404);
  assert.equal((await providerA.request(lab.readerURL+'/api/products?merchantId=merchant-b')).status, 400);
  const ciphertext = readFileSync(lab.workspace+'/reader-store.enc'); assert.ok(!ciphertext.includes(Buffer.from('es_at_')));
  console.log('PASS: merchant consent, PKCE/state binding, callback replay rejection, two-merchant isolation and encrypted provider storage.');
  await restart();
  await products(providerA, ['product-a2','product-a1']); await products(providerB, ['product-b1']);
  console.log('PASS: gateway PostgreSQL and encrypted provider sessions survive service restarts.');
  const revoked = await a.merchantBrowser.request(lab.gatewayURL+`/v1/merchant/installations/${a.installation.installationId}`, {
    method: 'DELETE', headers: { ...a.headers, 'Idempotency-Key': randomUUID() } });
  assert.equal(revoked.status, 204);
  assert.equal((await providerA.request(lab.readerURL+'/api/products')).status, 401);
  assert.equal((await providerA.request(lab.readerURL+'/api/products/product-a1')).status, 401);
  await products(providerB, ['product-b1']);
  console.log('PASS: merchant uninstall revokes list/detail access while the other merchant remains connected.');
}
