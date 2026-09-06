import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import { PortalAPI, PortalError, blankDocument, scopesFor } from './portal.ts';

void test('admin staff and developer directories use validated pagination, not raw query paths', async (t) => {
  const urls: string[] = [];
  t.mock.method(globalThis, 'fetch', async (url: string) => {
    urls.push(url);
    return new Response(JSON.stringify({ accounts: [], organizations: [] }), {
      status: 200,
    });
  });
  const admin = new PortalAPI('admin');
  await admin.request('/staff', 'GET', undefined, false, { afterId: '' });
  await admin.request('/staff', 'GET', undefined, false, {
    afterId: 'portal-local-admin',
  });
  await admin.request('/developers', 'GET', undefined, false, {
    afterId: 'dev-local',
  });
  assert.deepEqual(urls, [
    '/api/v1/admin/staff',
    '/api/v1/admin/staff?afterId=portal-local-admin',
    '/api/v1/admin/developers?afterId=dev-local',
  ]);
  await assert.rejects(
    admin.request('/staff?afterId=test'),
    /Invalid portal path/,
  );
  await assert.rejects(
    admin.request('/staff', 'GET', undefined, false, {
      afterId: 'a&role=admin',
    }),
    /Invalid page cursor/,
  );
  await assert.rejects(
    admin.request('/staff', 'POST', {}, false, { afterId: 'a' }),
    /Invalid page cursor/,
  );
  await assert.rejects(
    new PortalAPI('developer').request('/staff', 'GET', undefined, false, {
      afterId: 'a',
    }),
    /Invalid page cursor/,
  );
  assert.equal(urls.length, 3);
});

void test('portal clients use explicit surface, same-origin cookies and no merchant context', async (t) => {
  const calls: { url: string; options?: RequestInit }[] = [];
  t.mock.method(
    globalThis,
    'fetch',
    async (url: string, options?: RequestInit) => {
      calls.push({ url, options });
      return new Response(JSON.stringify({ apps: [] }), { status: 200 });
    },
  );
  await new PortalAPI('admin').request('/session');
  await new PortalAPI('developer').request('/apps');
  assert.deepEqual(
    calls.map((c) => c.url),
    ['/api/v1/admin/session', '/api/v1/developer/apps'],
  );
  for (const call of calls) {
    assert.equal(call.options?.credentials, 'same-origin');
    assert.equal(call.options?.cache, 'no-store');
    assert.equal(call.options?.body, undefined);
  }
  const source = readFileSync(
    new URL('../components/portal.tsx', import.meta.url),
    'utf8',
  );
  assert.doesNotMatch(
    source,
    /localStorage|sessionStorage|workspace=|\/api\/v1\/login|\/installations|tenantId/,
  );
});
void test('uncertain mutation retries reuse the same key; successful new operations get a new one', async (t) => {
  const keys: string[] = [];
  let fail = true;
  t.mock.method(
    globalThis,
    'fetch',
    async (_: string, options: RequestInit) => {
      keys.push((options.headers as Record<string, string>)['Idempotency-Key']);
      if (fail) {
        fail = false;
        throw new Error('Lost connection');
      }
      return new Response(JSON.stringify({ ok: true }), { status: 200 });
    },
  );
  const api = new PortalAPI('developer');
  const body = { revision: 0, document: blankDocument() };
  await assert.rejects(api.request('/apps', 'POST', body, true));
  await api.request('/apps', 'POST', body, true);
  await api.request('/apps', 'POST', body, true);
  assert.equal(keys[0], keys[1]);
  assert.notEqual(keys[1], keys[2]);
});
void test('permissions follow capability and errors remain explicit', async (t) => {
  assert.deepEqual(scopesFor('shipping/v1'), [
    'orders.read',
    'shipping.read',
    'shipping.write',
  ]);
  t.mock.method(
    globalThis,
    'fetch',
    async () =>
      new Response(JSON.stringify({ error: 'conflict' }), { status: 409 }),
  );
  await assert.rejects(
    new PortalAPI('admin').request(
      '/submissions/example/decision',
      'POST',
      {},
      true,
    ),
    (error: unknown) => error instanceof PortalError && error.status === 409,
  );
  await assert.rejects(new PortalAPI('developer').request('/../admin/session'));
});
