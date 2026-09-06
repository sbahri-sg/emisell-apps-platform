import assert from 'node:assert/strict';
import { test } from 'node:test';
import { assignmentActions, testingBlockers } from './testing.ts';
import { portalView } from './surfaces.ts';
import { PortalAPI } from './portal.ts';
import { readFileSync } from 'node:fs';

void test('managed releases use the existing assignment workflow with a distinct source kind', () => {
  const source = readFileSync(
    new URL('../components/testing.tsx', import.meta.url),
    'utf8',
  );
  assert.match(source, /\/managed-shipping-releases/);
  assert.match(source, /releaseKind: 'managed_shipping'/);
  assert.match(source, /managedReleases\.map/);
  assert.ok(testingBlockers.managed_installation_not_available);
  assert.ok(testingBlockers.engine_grant_enforcement_not_available);
  assert.doesNotMatch(source, /\/sandbox\/|\.install\(|\.consent\(/);
});

void test('only administrator decides distribution; terminal states never revive', () => {
  assert.deepEqual(assignmentActions('admin', 'administrator', 'requested'), [
    'approved',
    'rejected',
  ]);
  assert.deepEqual(assignmentActions('admin', 'administrator', 'approved'), [
    'revoked',
  ]);
  for (const status of ['rejected', 'revoked'] as const)
    assert.deepEqual(assignmentActions('admin', 'administrator', status), []);
  for (const role of ['reviewer', 'operator', 'developer'])
    assert.deepEqual(assignmentActions('admin', role, 'requested'), []);
  assert.deepEqual(
    assignmentActions('developer', 'administrator', 'requested'),
    [],
  );
  assert.ok(testingBlockers.runtime_not_available);
});
void test('Testing is separate from merchant workspace and available in both portals', () => {
  for (const s of ['admin', 'developer'])
    assert.equal(portalView(s, '?view=testing'), 'testing');
});
void test('pagination is narrow, validated and does not add merchant identity', async () => {
  const previous = globalThis.fetch;
  const calls: unknown[] = [];
  globalThis.fetch = (async (url, options) => {
    calls.push([url, options]);
    return new Response(JSON.stringify({ assignments: [], nextAfterId: '' }), {
      status: 200,
    });
  }) as typeof fetch;
  try {
    const api = new PortalAPI('developer');
    await api.request('/test-assignments', 'GET', undefined, false, {
      afterId: 'testasgn_test',
    });
    assert.equal(
      (calls[0] as string[])[0],
      '/api/v1/developer/test-assignments?afterId=testasgn_test',
    );
    await assert.rejects(
      api.request('/apps', 'GET', undefined, false, { afterId: 'cursor' }),
    );
    await assert.rejects(
      api.request('/test-assignments', 'GET', undefined, false, {
        afterId: 'a&merchantId=spoof',
      }),
    );
    assert.equal(calls.length, 1);
  } finally {
    globalThis.fetch = previous;
  }
});
