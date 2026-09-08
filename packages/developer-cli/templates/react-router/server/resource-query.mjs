import { productQuery, productPage } from './product-query.mjs';
export function resourceKind(path, search = new URLSearchParams()) {
  if (search.has('view')) {
    const match = /^\/v1\/products(?:\/([A-Za-z0-9_-]{1,128}))?$/.exec(path);
    const reserved = [
      'general-info',
      'analytics',
      'get-categories',
      'categories',
      'vendor',
      'type',
      'export',
      'import',
      'sample-csv',
      'stock',
      'recommendations',
      'first',
      'variants',
    ];
    if (
      search.getAll('view').length === 1 &&
      search.get('view') === 'inventory' &&
      match &&
      !reserved.includes(match[1]?.toLowerCase())
    )
      return 'inventory';
    throw Error('invalid_resource');
  }
  const location =
    /^\/v1\/settings\/location(?:\/([A-Za-z0-9_-]{1,128}))?$/.exec(path);
  if (location && !['countries', 'first'].includes(location[1]?.toLowerCase()))
    return 'locations';
  const grouping =
    /^\/v1\/(catalogs|collections)(?:\/([A-Za-z0-9_-]{1,128}))?$/.exec(path);
  if (
    grouping &&
    !['first', 'bulk-delete', 'export', 'import'].includes(
      grouping[2]?.toLowerCase(),
    )
  )
    return grouping[1];
  if (path === '/v1/products') return 'products';
  if (
    path === '/v1/orders' ||
    /^\/v1\/orders\/[A-Za-z0-9_-]{1,128}$/.test(path)
  )
    return 'orders';
  if (
    path === '/v1/settings/shipping' ||
    /^\/v1\/settings\/shipping\/profile\/[A-Za-z0-9_-]{1,128}$/.test(path)
  )
    return 'shipping';
  throw Error('invalid_resource');
}
export function resourceQuery(path, search = new URLSearchParams()) {
  const kind = resourceKind(path, search);
  if (kind === 'products') return productQuery(search);
  const grouping = ['catalogs', 'collections'].includes(kind);
  const detail = ![
    '/v1/products',
    '/v1/orders',
    '/v1/settings/shipping',
    '/v1/catalogs',
    '/v1/collections',
    '/v1/settings/location',
  ].includes(path);
  for (const [key, value] of search) {
    if (key === 'view' && kind === 'inventory') continue;
    if (
      detail ||
      ![
        'limit',
        'cursor',
        ...(kind === 'orders' ? ['status', 'updatedAfter'] : []),
        ...(grouping || kind === 'locations' ? ['q'] : []),
      ].includes(key) ||
      search.getAll(key).length !== 1 ||
      !value
    )
      throw Error('invalid_query');
  }
  if (detail) return kind === 'inventory' ? { view: 'inventory' } : {};
  const limit = search.get('limit') ?? '5';
  if (
    !Number.isInteger(Number(limit)) ||
    String(Number(limit)) !== limit ||
    Number(limit) < 1 ||
    Number(limit) > 20
  )
    throw Error('invalid_query');
  const cursor = search.get('cursor'),
    status = search.get('status'),
    updatedAfter = search.get('updatedAfter');
  if (
    cursor &&
    (cursor.length > 1024 || !/^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/.test(cursor))
  )
    throw Error('invalid_query');
  if (
    status &&
    ![
      'UNPAID',
      'PAID',
      'PENDING',
      'PROCESSING',
      'REJECTED',
      'SHIPPING',
      'FAILED',
      'REFUNDED',
      'COMPLETED',
      'CANCELED',
    ].includes(status)
  )
    throw Error('invalid_query');
  if (
    updatedAfter &&
    (!/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,3})?Z$/.test(updatedAfter) ||
      !Number.isFinite(Date.parse(updatedAfter)))
  )
    throw Error('invalid_query');
  const q = search.get('q');
  if (
    q &&
    (q !== q.trim() || Buffer.byteLength(q) > 100 || /[\x00-\x1f\x7f]/.test(q))
  )
    throw Error('invalid_query');
  return {
    limit: Number(limit),
    ...(kind === 'inventory' ? { view: 'inventory' } : {}),
    ...(cursor ? { cursor } : {}),
    ...(q ? { q } : {}),
    ...(status ? { status } : {}),
    ...(updatedAfter ? { updatedAfter } : {}),
  };
}
const text = (value) => typeof value === 'string' && value.length <= 2000;
const id = (value) =>
  typeof value === 'string' && /^[A-Za-z0-9_-]{1,128}$/.test(value);
const amount = (value) =>
  typeof value === 'string' && /^-?\d{1,24}(?:\.\d{1,8})?$/.test(value);
function order(v) {
  if (
    !v ||
    !id(v.id) ||
    !text(v.numberFormat) ||
    !text(v.status) ||
    !amount(v.subtotal)
  )
    throw Error('invalid_resource');
  return {
    id: v.id,
    numberFormat: v.numberFormat,
    status: v.status,
    subtotal: v.subtotal,
    currency:
      v.currency === null ||
      (typeof v.currency === 'string' &&
        v.currency.length === 3 &&
        /^[A-Z]{3}$/.test(v.currency))
        ? v.currency
        : null,
  };
}
function profile(v) {
  if (!v || !id(v.id) || !text(v.name) || typeof v.isPrimary !== 'boolean')
    throw Error('invalid_resource');
  return { id: v.id, name: v.name, isPrimary: v.isPrimary };
}
function groupingRow(v, kind, detail) {
  const catalog = kind === 'catalogs';
  if (
    !v ||
    !id(v.id) ||
    !text(catalog ? v.title : v.name) ||
    !Number.isSafeInteger(v.productCount) ||
    v.productCount < 0
  )
    throw Error('invalid_resource');
  const row = {
    id: v.id,
    ...(catalog ? { title: v.title } : { name: v.name }),
    productCount: v.productCount,
  };
  if (!detail) return row;
  if (
    !Array.isArray(v.products) ||
    v.products.length > 100 ||
    v.products.length !== v.productCount
  )
    throw Error('invalid_resource');
  return {
    ...row,
    products: v.products.map((ref) => {
      if (
        !id(ref.productId) ||
        (catalog && ref.variantId !== null && !id(ref.variantId))
      )
        throw Error('invalid_resource');
      return {
        productId: ref.productId,
        ...(catalog ? { variantId: ref.variantId } : {}),
      };
    }),
  };
}
function locationRow(v) {
  const flags = [
    'isPrimary',
    'isActive',
    'isPhysicalStorefront',
    'isFulfillment',
    'shippingIsActive',
    'deliveryIsActive',
    'pickUpIsActive',
  ];
  if (
    !v ||
    !id(v.id) ||
    !text(v.name) ||
    flags.some((key) => typeof v[key] !== 'boolean')
  )
    throw Error('invalid_resource');
  return {
    id: v.id,
    name: v.name,
    ...Object.fromEntries(flags.map((key) => [key, v[key]])),
  };
}
function inventoryRow(v) {
  if (
    !v ||
    !id(v.id) ||
    typeof v.trackInventory !== 'boolean' ||
    typeof v.continueSellingWhenOutOfStock !== 'boolean' ||
    !Number.isSafeInteger(v.stock) ||
    !Array.isArray(v.items) ||
    !v.items.length ||
    v.items.length > 100
  )
    throw Error('invalid_resource');
  const seen = new Set();
  let count = 0;
  const items = v.items.map((item) => {
    if (
      !id(item.id) ||
      seen.has(item.id) ||
      !['PRODUCT', 'VARIANT'].includes(item.type) ||
      !Number.isSafeInteger(item.stock) ||
      !Array.isArray(item.levels) ||
      (item.type === 'PRODUCT' && (item.id !== v.id || v.items.length !== 1))
    )
      throw Error('invalid_resource');
    seen.add(item.id);
    const locations = new Set();
    const levels = item.levels.map((level) => {
      count++;
      if (
        count > 100 ||
        !id(level.locationId) ||
        locations.has(level.locationId) ||
        !Number.isInteger(level.available) ||
        level.available < -2147483648 ||
        level.available > 2147483647
      )
        throw Error('invalid_resource');
      locations.add(level.locationId);
      return { locationId: level.locationId, available: level.available };
    });
    if (levels.reduce((sum, row) => sum + row.available, 0) !== item.stock)
      throw Error('invalid_resource');
    return { id: item.id, type: item.type, stock: item.stock, levels };
  });
  if (items.reduce((sum, item) => sum + item.stock, 0) !== v.stock)
    throw Error('invalid_resource');
  return {
    id: v.id,
    trackInventory: v.trackInventory,
    continueSellingWhenOutOfStock: v.continueSellingWhenOutOfStock,
    stock: v.stock,
    items,
  };
}
export function resourcePage(path, value, limit = 5, query = {}) {
  const kind = resourceKind(path, new URLSearchParams(query));
  if (kind === 'products') return productPage(value, limit);
  if (!value || !Object.hasOwn(value, 'data')) throw Error('invalid_resource');
  const grouping = ['catalogs', 'collections'].includes(kind);
  const detail = ![
    '/v1/products',
    '/v1/orders',
    '/v1/settings/shipping',
    '/v1/catalogs',
    '/v1/collections',
    '/v1/settings/location',
  ].includes(path);
  if (detail) {
    const v = value.data;
    if (v?.id !== path.split('/').at(-1)) throw Error('invalid_resource');
    if (kind === 'inventory' || kind === 'locations')
      return { data: (kind === 'inventory' ? inventoryRow : locationRow)(v) };
    if (grouping) return { data: groupingRow(v, kind, true) };
    if (kind === 'orders') {
      if (!Array.isArray(v.items) || v.items.length > 100)
        throw Error('invalid_resource');
      return {
        data: {
          ...order(v),
          items: v.items.map((item) => {
            if (
              !id(item.id) ||
              !Number.isInteger(item.quantity) ||
              item.quantity < 0 ||
              !amount(item.price) ||
              !amount(item.lineTotal)
            )
              throw Error('invalid_resource');
            return {
              id: item.id,
              name: text(item.name) ? item.name : null,
              quantity: item.quantity,
              price: item.price,
              lineTotal: item.lineTotal,
            };
          }),
        },
      };
    }
    if (!Array.isArray(v.zones) || v.zones.length > 100)
      throw Error('invalid_resource');
    return {
      data: {
        ...profile(v),
        zones: v.zones.map((zone) => {
          if (!id(zone.id) || !text(zone.name)) throw Error('invalid_resource');
          return { id: zone.id, name: zone.name };
        }),
      },
    };
  }
  const cursor = value.meta?.nextCursor;
  if (
    cursor !== null &&
    (typeof cursor !== 'string' || !cursor || cursor.length > 1024)
  )
    throw Error('invalid_resource');
  const meta = { nextCursor: cursor };
  if (kind === 'inventory' || kind === 'locations') {
    if (!Array.isArray(value.data) || value.data.length > limit)
      throw Error('invalid_resource');
    return {
      data: value.data.map(kind === 'inventory' ? inventoryRow : locationRow),
      meta,
    };
  }
  if (grouping) {
    if (!Array.isArray(value.data) || value.data.length > limit)
      throw Error('invalid_resource');
    return { data: value.data.map((v) => groupingRow(v, kind, false)), meta };
  }
  if (kind === 'orders') {
    if (!Array.isArray(value.data) || value.data.length > limit)
      throw Error('invalid_resource');
    return { data: value.data.map(order), meta };
  }
  if (value.data === null) {
    if (cursor !== null) throw Error('invalid_resource');
    return { data: null, meta };
  }
  const v = value.data;
  if (
    !id(v.id) ||
    typeof v.splitShipping !== 'boolean' ||
    !text(v.estimatedShow) ||
    !Array.isArray(v.profiles) ||
    v.profiles.length > limit
  )
    throw Error('invalid_resource');
  return {
    data: {
      id: v.id,
      splitShipping: v.splitShipping,
      estimatedShow: v.estimatedShow,
      profiles: v.profiles.map(profile),
    },
    meta,
  };
}
