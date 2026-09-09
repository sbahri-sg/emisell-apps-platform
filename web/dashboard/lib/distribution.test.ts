import assert from 'node:assert/strict';
import test from 'node:test';
import { publicDistributionAllowed } from './distribution.ts';
import { blankDocument, scopesFor } from './portal.ts';
import { readFileSync } from 'node:fs';

void test('new private apps request product reads only, without changing public distribution', () => {
  assert.equal(blankDocument().capability, 'private-products/v1');
  assert.deepEqual(blankDocument().scopes, []);
  assert.deepEqual(blankDocument().accessScopes?.required, ['read_products']);
  assert.deepEqual(blankDocument().accessScopes?.optional, []);
  assert.equal(blankDocument().endpoint, '');
  assert.equal(publicDistributionAllowed('private-products/v1'), false);
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
  assert.deepEqual(
    spec.components.schemas.SaveDraft.properties.document.oneOf,
    [
      { $ref: '#/components/schemas/PublicAppDocument' },
      { $ref: '#/components/schemas/PrivateProductDocument' },
    ],
  );
});
