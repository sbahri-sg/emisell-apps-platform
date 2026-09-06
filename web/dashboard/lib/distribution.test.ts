import assert from 'node:assert/strict';
import test from 'node:test';
import { publicDistributionAllowed } from './distribution.ts';
import { blankDocument, scopesFor } from './portal.ts';
import { readFileSync } from 'node:fs';

void test('authoring defaults to shipping; payment and unknown capabilities fail closed', () => {
  assert.equal(blankDocument().capability, 'shipping/v1');
  assert.deepEqual(blankDocument().scopes, [
    'orders.read',
    'shipping.read',
    'shipping.write',
  ]);
  assert.equal(publicDistributionAllowed('shipping/v1'), true);
  for (const c of ['payment/v1', '', 'shipping/v2', 'unknown'])
    assert.equal(publicDistributionAllowed(c), false);
  assert.deepEqual(scopesFor('unknown'), []);
  assert.deepEqual(scopesFor('payment/v1'), [
    'orders.read',
    'payments.read',
    'payments.write',
  ]);
});

void test('input schema only offers shipping while historical response schemas retain payment', () => {
  const spec = JSON.parse(
    readFileSync(
      new URL('../../../api/openapi/portals.v1.json', import.meta.url),
      'utf8',
    ),
  );
  assert.deepEqual(
    spec.components.schemas.PublicAppDocument.properties.capability.enum,
    ['shipping/v1'],
  );
  assert.ok(
    spec.components.schemas.AppDocument.properties.capability.enum.includes(
      'payment/v1',
    ),
  );
  assert.equal(
    spec.components.schemas.SaveDraft.properties.document.$ref,
    '#/components/schemas/PublicAppDocument',
  );
});
