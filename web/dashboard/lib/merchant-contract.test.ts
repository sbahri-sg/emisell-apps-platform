import assert from 'node:assert/strict';
import test from 'node:test';
import {
  documents,
  example,
  filterOperations,
  operations,
} from './api-docs.ts';

void test('primary API schemas and examples need only merchantId, never a second store identifier', () => {
  for (const operation of operations) {
    assert.doesNotMatch(
      JSON.stringify(operation),
      /tenantId|tenant_id|X-Emisell-Tenant-ID/,
    );
    if (operation.request)
      assert.doesNotMatch(
        JSON.stringify(example(operation.request)),
        /tenantId|tenant_id/,
      );
  }
  for (const operation of filterOperations('gateway', '')) {
    assert.ok(operation.request?.properties?.access?.properties?.merchantId);
  }
  const prepare = operations.find((o) =>
    o.path.endsWith('InstallIntentService/Prepare'),
  )!;
  assert.ok(prepare.request?.properties?.merchantId);
  const selfCheck = operations.find(
    (o) => o.path === '/api/v1/app/installation-access',
  )!;
  assert.ok(
    selfCheck.parameters.some(
      (p) => p.name === 'X-Emisell-Merchant-ID' && p.required,
    ),
  );
  assert.ok(selfCheck.responses[0].schema?.properties?.merchantId);
  assert.ok(!documents.some((d) => d.name === 'api-keys.v1.json'));
  for (const document of documents.filter((d) => d.format === 'Protobuf')) {
    // Original downloadable sources retain explicitly deprecated wire aliases.
    assert.doesNotMatch(document.content, /string tenant_id = \d+;/);
  }
});
