import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { integrationActions } from './integration-releases.ts';
import { portalView } from './surfaces.ts';

void test('integration review and signing actions respect surface, role, and lifecycle', () => {
  assert.deepEqual(integrationActions('admin', 'reviewer', 'submitted'), [
    'approved',
    'rejected',
  ]);
  assert.deepEqual(integrationActions('admin', 'reviewer', 'approved'), []);
  assert.deepEqual(integrationActions('admin', 'administrator', 'approved'), [
    'signed',
    'suspended',
  ]);
  assert.deepEqual(integrationActions('admin', 'administrator', 'signed'), [
    'suspended',
  ]);
  for (const status of [
    'submitted',
    'approved',
    'rejected',
    'signed',
    'suspended',
  ] as const) {
    assert.deepEqual(
      integrationActions('developer', 'administrator', status),
      [],
    );
    assert.deepEqual(integrationActions('admin', 'operator', status), []);
  }
  assert.deepEqual(
    integrationActions('admin', 'administrator', 'rejected'),
    [],
  );
  assert.deepEqual(
    integrationActions('admin', 'administrator', 'suspended'),
    [],
  );
  assert.equal(
    portalView('developer', '?view=integration-releases'),
    'integration-releases',
  );
  assert.equal(
    portalView('admin', '?view=integration-releases'),
    'integration-releases',
  );
});
void test('display naming changes without rewriting runtime contracts', () => {
  const portal = readFileSync(
    new URL('../components/portal.tsx', import.meta.url),
    'utf8',
  );
  assert.doesNotMatch(portal, /remote app/i);
  assert.match(portal, /Aplikasi Integrasi/);
  const policy = readFileSync(
    new URL('../../../pkg/integrationmanifest/manifest.go', import.meta.url),
    'utf8',
  );
  assert.match(policy, /integration-configuration\/v1/);
});
