import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { clientActions, type ClientView } from './app-clients.ts';
import { portalView } from './surfaces.ts';
import { filterOperations, operations } from './api-docs.ts';

void test('client actions enforce roles, proof freshness and cooldown', () => {
  const now = Date.now();
  const v = {
    client: {
      status: 'pending',
      challengeExpiresAt: new Date(now + 60000).toISOString(),
      lastAttemptAt: null,
      verifiedUntil: null,
    },
  } as ClientView;
  assert.deepEqual(clientActions('admin', 'operator', v, now), []);
  assert.deepEqual(clientActions('admin', 'reviewer', v, now), []);
  assert.deepEqual(clientActions('admin', 'administrator', v, now), ['revoke']);
  assert.deepEqual(clientActions('developer', 'developer', v, now), [
    'challenge',
    'verify',
    'revoke',
  ]);
  v.client.lastAttemptAt = new Date(now - 1000).toISOString();
  assert.deepEqual(clientActions('developer', 'developer', v, now), [
    'challenge',
    'revoke',
  ]);
  v.client.status = 'verified';
  v.client.verifiedUntil = new Date(now + 1000).toISOString();
  assert.ok(
    clientActions('developer', 'developer', v, now).includes('rotate_secret'),
  );
  assert.ok(
    !clientActions('developer', 'developer', v, now + 2000).includes(
      'rotate_secret',
    ),
  );
  v.client.status = 'revoked';
  assert.deepEqual(clientActions('developer', 'developer', v, now), []);
  assert.equal(portalView('admin', '?view=app-clients'), 'app-clients');
  assert.equal(portalView('developer', '?view=app-clients'), 'app-clients');
});
void test('client proof and secret stay in explicit backend-managed flow', () => {
  const src = readFileSync(
    new URL('../components/app-clients.tsx', import.meta.url),
    'utf8',
  );
  assert.doesNotMatch(
    src,
    /localStorage|sessionStorage|fetch\(|console\.|document\.cookie/,
  );
  assert.match(src, /setTimeout\([\s\S]*5 \* 60 \* 1000/);
  assert.match(src, /AlertDialog/);
  assert.match(src, /Tanpa|tanpa/);
  const auth = filterOperations('app-access', 'client-check');
  assert.equal(auth.length, 1);
  assert.deepEqual(auth[0].request?.properties, {});
  assert.equal(
    auth[0].responses[0].schema?.properties?.oauthEnabled?.const,
    false,
  );
  assert.equal(
    operations.filter((o) => o.source === 'app-clients.v1.json').length,
    8,
  );
});
