import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import {
  catalogPreview,
  catalogPreviewCode,
  initialCatalogConfiguration,
} from './catalog-preview.ts';

void test('visual filters remain mock-only and never become invented backend parameters', () => {
  const configuration = {
    ...initialCatalogConfiguration,
    buyerCountry: 'Indonesia',
    color: 'Blue',
    limit: 7,
  };
  const result = catalogPreview('Retail', 'my-example', configuration);
  assert.equal(result.request.mock, true);
  assert.equal(result.response.mock, true);
  assert.equal(result.request.preview.color, 'Blue');
  assert.equal(result.response.data[0].title, 'Katalog Retail');
  assert.equal(result.response.meta.limit, 7);
  assert.doesNotMatch(
    result.path,
    /color|buyerCountry|catalog_id|search_catalog/,
  );
  assert.match(result.path, /\/v1\/catalogs\?/);
  assert.equal(
    catalogPreview('missing', 'my-example', configuration).response.data.length,
    0,
  );
});
void test('request format switching and copied examples use the same generated request', () => {
  const result = catalogPreview("a'&b", 'example', initialCatalogConfiguration);
  assert.deepEqual(
    JSON.parse(catalogPreviewCode(result, 'request', 'json')),
    result.request,
  );
  assert.deepEqual(
    JSON.parse(catalogPreviewCode(result, 'response', 'js')),
    result.response,
  );
  assert.ok(
    catalogPreviewCode(result, 'request', 'curl').includes(result.path),
  );
  assert.ok(catalogPreviewCode(result, 'request', 'js').includes(result.path));
  assert.doesNotMatch(catalogPreviewCode(result, 'request', 'curl'), /a'&b/);
});
void test('reference layout keeps configuration groups, inspector formats and example-only actions', () => {
  const component = readFileSync(
    new URL('../components/developer-catalogs.tsx', import.meta.url),
    'utf8',
  );
  for (const label of [
    'Configuration',
    'Search preview',
    'SOURCE',
    'QUERY',
    'REGION',
    'ATTRIBUTES',
    'LISTING',
    'API KEY',
    'Request',
    'Response',
    'JSON',
    'cURL',
    'JS',
  ])
    assert.ok(component.includes(label), label);
  assert.match(component, /RadioGroup/);
  assert.match(component, /Checkbox/);
  assert.match(component, /Mock \/ Contoh/);
  assert.match(component, /Tidak dikirim/);
  assert.match(component, /Restore example catalog/);
  assert.doesNotMatch(
    component,
    /fetch\(|api\.request|document\.cookie|localStorage|sessionStorage|dangerouslySetInnerHTML/,
  );
});
