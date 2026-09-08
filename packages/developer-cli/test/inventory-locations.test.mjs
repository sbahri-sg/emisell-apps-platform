import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  resourceKind,
  resourceQuery,
  resourcePage,
} from '../templates/react-router/server/resource-query.mjs';
const inventory = {
  id: 'p',
  trackInventory: true,
  continueSellingWhenOutOfStock: false,
  stock: -2,
  items: [
    {
      id: 'p',
      type: 'PRODUCT',
      stock: -2,
      levels: [{ locationId: 'l', available: -2, private: 'hidden' }],
    },
  ],
  private: 'hidden',
};
const location = {
  id: 'l',
  name: 'Warehouse',
  isActive: false,
  isPrimary: true,
  isPhysicalStorefront: false,
  isFulfillment: true,
  shippingIsActive: true,
  deliveryIsActive: false,
  pickUpIsActive: false,
  address: 'hidden',
  phone: 'hidden',
};
test('inventory selector is explicit and preserved on the existing product URL', () => {
  assert.equal(resourceKind('/v1/products'), 'products');
  assert.equal(
    resourceKind('/v1/products', new URLSearchParams('view=inventory')),
    'inventory',
  );
  assert.deepEqual(
    resourceQuery(
      '/v1/products',
      new URLSearchParams('view=inventory&limit=5'),
    ),
    { limit: 5, view: 'inventory' },
  );
  assert.deepEqual(
    resourceQuery('/v1/products/p', new URLSearchParams('view=inventory')),
    { view: 'inventory' },
  );
  for (const query of [
    'view=other',
    'view=inventory&view=inventory',
    'view=inventory&q=x',
    'view=inventory&merchantId=x',
    'view=inventory&limit=21',
  ])
    assert.throws(() =>
      resourceQuery('/v1/products', new URLSearchParams(query)),
    );
  assert.throws(() =>
    resourceQuery(
      '/v1/products/p',
      new URLSearchParams('view=inventory&limit=1'),
    ),
  );
  for (const path of [
    '/v1/products/stock',
    '/v1/products/FIRST',
    '/v1/settings/location',
  ])
    assert.throws(() =>
      resourceKind(path, new URLSearchParams('view=inventory')),
    );
});
test('inventory projection strips unknown fields and rejects inconsistent or partial balances', () => {
  const project = (row) =>
    resourcePage('/v1/products/p', { data: row }, 5, { view: 'inventory' });
  assert.equal(project(inventory).data.stock, -2);
  assert.equal(JSON.stringify(project(inventory)).includes('hidden'), false);
  for (const row of [
    { ...inventory, stock: 0 },
    { ...inventory, items: [] },
    { ...inventory, trackInventory: null },
    { ...inventory, id: 'other' },
    { ...inventory, items: [{ ...inventory.items[0], levels: null }] },
    {
      ...inventory,
      items: [
        {
          ...inventory.items[0],
          levels: [...inventory.items[0].levels, ...inventory.items[0].levels],
        },
      ],
    },
  ])
    assert.throws(() => project(row));
  assert.deepEqual(
    resourcePage('/v1/products', { data: [], meta: { nextCursor: null } }, 5, {
      view: 'inventory',
    }),
    { data: [], meta: { nextCursor: null } },
  );
  assert.throws(() =>
    resourcePage('/v1/products', { data: [inventory] }, 5, {
      view: 'inventory',
    }),
  );
});
test('locations use existing URLs with metadata only, bounded search and identity-bound details', () => {
  assert.deepEqual(
    resourceQuery(
      '/v1/settings/location',
      new URLSearchParams('q=Warehouse&limit=5'),
    ),
    { limit: 5, q: 'Warehouse' },
  );
  for (const path of [
    '/v1/settings/location/first',
    '/v1/settings/location/countries',
    '/v1/locations',
  ])
    assert.throws(() => resourceKind(path));
  for (const query of ['address=true', 'merchantId=x', 'q=%20x', 'limit=21'])
    assert.throws(() =>
      resourceQuery('/v1/settings/location', new URLSearchParams(query)),
    );
  const result = resourcePage('/v1/settings/location/l', { data: location });
  assert.equal(result.data.isActive, false);
  assert.equal(JSON.stringify(result).includes('hidden'), false);
  assert.throws(() =>
    resourcePage('/v1/settings/location/other', { data: location }),
  );
  assert.throws(() =>
    resourcePage('/v1/settings/location/l', {
      data: { ...location, isActive: null },
    }),
  );
  assert.deepEqual(
    resourcePage('/v1/settings/location', {
      data: [],
      meta: { nextCursor: null },
    }),
    { data: [], meta: { nextCursor: null } },
  );
});
