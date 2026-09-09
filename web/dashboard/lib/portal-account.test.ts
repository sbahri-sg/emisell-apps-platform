import assert from 'node:assert/strict';
import test from 'node:test';
import { endPortalSession, logoutPortal } from './portal-account.ts';
import { readFileSync } from 'node:fs';

void test('switch account revokes the authenticated surface through existing logout', async (t) => {
  const calls: { url: string; options?: RequestInit }[] = [];
  t.mock.method(
    globalThis,
    'fetch',
    async (url: string, options?: RequestInit) => {
      calls.push({ url, options });
      return Response.json({ ok: true });
    },
  );
  await endPortalSession('admin');
  await endPortalSession('developer');
  assert.deepEqual(
    calls.map((call) => call.url),
    ['/api/v1/admin/logout', '/api/v1/developer/logout'],
  );
  for (const { options } of calls) {
    assert.equal(options?.method, 'POST');
    assert.equal(options?.credentials, 'same-origin');
    assert.equal(options?.body, '{}');
  }
});

void test('expired sessions can return to login, but logout failures are not swallowed', async (t) => {
  let status = 401;
  t.mock.method(globalThis, 'fetch', async () =>
    Response.json({ error: 'unavailable' }, { status }),
  );
  await endPortalSession('admin');
  for (status of [403, 500]) await assert.rejects(endPortalSession('admin'));
});

void test('account switch is explicit and never turns the current admin into a developer', () => {
  const source = readFileSync(
    new URL('../components/unified-portal.tsx', import.meta.url),
    'utf8',
  );
  assert.match(source, /session\.user\.surface !== surface/);
  assert.match(
    source,
    /await endPortalSession\(session\.user\.surface\);\s*setSession\(null\)/,
  );
  assert.match(source, /knownSignedOut=\{!session\}/);
  assert.match(source, /setSession\(await request\('login'/);
  assert.doesNotMatch(
    source,
    /user\.surface\s*=(?!=)|localStorage|document\.cookie/,
  );
});

void test('developer logout waits for revocation then leaves without mounting automatic login', async (t) => {
  const events: string[] = [];
  let status = 200;
  t.mock.method(globalThis, 'fetch', async () => {
    events.push('revoke');
    return Response.json({ ok: status === 200 }, { status });
  });
  const navigation = {
    leaveDeveloper: () => {
      events.push('documentation');
    },
    showLogin: () => {
      events.push('login');
    },
  };
  for (status of [200, 401]) {
    events.length = 0;
    await logoutPortal('developer', navigation);
    assert.deepEqual(events, ['revoke', 'documentation']);
  }
  events.length = 0;
  await logoutPortal('admin', navigation);
  assert.deepEqual(events, ['revoke', 'login']);
  for (status of [403, 500]) {
    events.length = 0;
    await assert.rejects(logoutPortal('developer', navigation));
    assert.deepEqual(events, ['revoke']);
  }
});

void test('both portal logout controls share the safe navigation handler', () => {
  const source = readFileSync(
    new URL('../components/portal.tsx', import.meta.url),
    'utf8',
  );
  assert.equal(source.match(/logout=\{logout\}/g)?.length, 2);
  assert.match(source, /logoutPortal\(surface,/);
  assert.match(
    source,
    /leaveDeveloper: \(\) => window\.location\.replace\('\/'\)/,
  );
  assert.doesNotMatch(source, /api\.request\('\/logout'/);
});
