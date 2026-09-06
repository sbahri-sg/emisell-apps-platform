import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

void test('technical shipping mapping stays in documentation, not the scope catalog', () => {
  const view = readFileSync(
    new URL('../components/access-scopes.tsx', import.meta.url),
    'utf8',
  );
  const reference = readFileSync(
    new URL('../../../docs/shipping-scope-mapping.md', import.meta.url),
    'utf8',
  );
  assert.ok(!view.includes('ShippingAccessReference'));
  assert.ok(!view.includes('shipping/v1'));
  assert.ok(!view.includes('rates.read'));
  for (const value of [
    'shipping/v1',
    'shipping.read',
    'rates.read',
    'settings.read',
    'read_shipping',
    'write_shipping',
    'local-isolated',
  ]) {
    assert.ok(reference.includes(value), value);
  }
  const coverage = JSON.parse(
    readFileSync(
      new URL(
        '../../../api/gateway/coverage.v1.generated.json',
        import.meta.url,
      ),
      'utf8',
    ),
  );
  // Validate the authoritative exported resource mapping, not UI labels.
  const rows = Object.values(coverage).find(
    (v) => Array.isArray(v) && v.some((r) => r.scope === 'read_shipping'),
  ) as { scope: string; grantable: boolean; implementation: string }[];
  for (const name of ['read_shipping', 'write_shipping']) {
    const row = rows.find((r) => r.scope === name);
    assert.equal(row?.grantable, false);
    assert.equal(row?.implementation, 'planned');
  }
});
