import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import {
  developerMainMenu,
  developerLocation,
  developerSearch,
} from './developer-navigation.ts';
import {
  developerStoreURL,
  searchDeveloperStores,
} from './developer-stores.ts';
import { catalogSample } from './developer-catalogs.ts';

const account = {
  sellerOrigin: 'http://localhost:3000',
  profile: {
    name: 'Developer',
    email: 'developer@example.test',
    stores: [{ id: 'merchant_A', commonId: 'store-a', name: 'Toko A' }],
  },
};
void test('personal developer navigation exposes Apps, Stores and Catalogs without admin review navigation', () => {
  assert.deepEqual(
    developerMainMenu.map((item) => item.label),
    ['Apps', 'Stores', 'Catalogs'],
  );
  for (const view of ['stores', 'catalogs']) {
    assert.deepEqual(
      developerLocation(developerSearch('?app=app_A', view, 'app_A')),
      { view, appId: '' },
    );
  }
  const portal = readFileSync(
    new URL('../components/portal.tsx', import.meta.url),
    'utf8',
  );
  assert.match(portal, /developerMainMenu/);
  assert.match(portal, /developer && view === 'stores'/);
  assert.match(portal, /developer && view === 'catalogs'/);
});
void test('store navigation accepts only verified account stores and fixed seller destinations', () => {
  assert.equal(
    developerStoreURL(account, 'merchant_A'),
    'http://localhost:3000/store/store-a',
  );
  assert.equal(
    developerStoreURL(account, 'merchant_A', 'apps'),
    'http://localhost:3000/store/store-a/settings/apps',
  );
  assert.equal(developerStoreURL(account, 'other_merchant'), null);
  for (const sellerOrigin of [
    'javascript:alert(1)',
    'https://user:password@example.test',
    'https://example.test?redirect=evil',
    'https://example.test/path',
    'invalid',
  ]) {
    assert.equal(
      developerStoreURL({ ...account, sellerOrigin }, 'merchant_A'),
      null,
    );
  }
  assert.equal(
    developerStoreURL({ ...account, profile: null }, 'merchant_A'),
    null,
  );
  assert.equal(
    developerStoreURL(
      {
        ...account,
        profile: {
          ...account.profile,
          stores: [{ id: 'merchant_A', name: 'bad', commonId: '../admin' }],
        },
      },
      'merchant_A',
    ),
    null,
  );
  assert.equal(
    searchDeveloperStores(account.profile.stores, ' TOKO a ').length,
    1,
  );
  assert.equal(
    searchDeveloperStores(account.profile.stores, 'missing').length,
    0,
  );
});
void test('catalog examples use existing backend contract, support search and pagination, and never fetch', () => {
  const first = catalogSample('', 1, 1),
    next = catalogSample('', 2, 1);
  assert.equal(first.response.meta.total, 2);
  assert.equal(first.response.meta.totalPages, 2);
  assert.notEqual(first.response.data[0].id, next.response.data[0].id);
  assert.equal(
    catalogSample(' retail ').response.data[0].title,
    'Katalog Retail',
  );
  assert.equal(catalogSample('missing').response.data.length, 0);
  assert.equal(catalogSample('', -10).response.meta.page, 1);
  assert.equal(catalogSample('', 1000, 1).response.meta.page, 2);
  assert.equal(catalogSample('', 1, -10).response.meta.limit, 10);
  assert.match(first.request.path, /^\/v1\/catalogs\?page=1&limit=1/);
  for (const row of first.response.data)
    assert.ok(row.id.startsWith('catalog_example_'));
  const source = readFileSync(
    new URL('../components/developer-catalogs.tsx', import.meta.url),
    'utf8',
  );
  assert.match(source, /Mock \/ Contoh/);
  assert.match(source, /Tidak dikirim/);
  assert.doesNotMatch(
    source,
    /fetch\(|api\.request|document\.cookie|localStorage|sessionStorage/,
  );
});
