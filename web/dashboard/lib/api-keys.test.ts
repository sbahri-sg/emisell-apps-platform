import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import { portalView } from './surfaces.ts';
import { operations } from './api-docs.ts';

void test('API key UI is Admin-only and never persists or re-fetches secrets', () => {
  assert.equal(portalView('admin', '?view=api-keys'), 'api-keys');
  assert.equal(portalView('developer', '?view=api-keys'), 'apps');
  const source = readFileSync(
    new URL('../components/api-keys.tsx', import.meta.url),
    'utf8',
  );
  assert.match(source, /role === 'administrator'/);
  assert.match(source, /secretAvailable/);
  assert.match(source, /setIssued\(null\)/);
  assert.match(source, /AlertDialogDescription/);
  assert.doesNotMatch(
    source,
    /localStorage|sessionStorage|document\.cookie|console\.|\/workspaces|\/reveal/,
  );
  const create = operations.find(
    (o) => o.path === '/api/v1/admin/platform-keys' && o.method === 'POST',
  )!;
  assert.ok(
    create.parameters.some((p) => p.name === 'Idempotency-Key' && p.required),
  );
  assert.deepEqual(Object.keys(create.request!.properties!), ['name']);
  assert.match(source, /Full access/);
  assert.match(source, /Berlaku sampai dicabut/);
  assert.doesNotMatch(
    source,
    /setTenantId|setDays|setScopes|NativeSelect|Checkbox|validDays|expiresAt/,
  );
  assert.ok(operations.some((o) => o.path.endsWith('ConnectionService/Check')));
  assert.ok(
    operations.some(
      (o) => o.path === '/api/v1/admin/access-scopes/verification',
    ),
  );
});
