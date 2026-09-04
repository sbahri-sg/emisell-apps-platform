// Design-only expansion contract. Never merge these routes into the running
// gateway/provider contract before both implementations and their tests exist.
import { readFileSync, writeFileSync } from 'node:fs';

const root = new URL('../', import.meta.url);
const pilot = JSON.parse(readFileSync(new URL('docs/emisell-resource-openapi.json', root), 'utf8'));
const ref = (name) => ({ $ref: `#/components/schemas/${name}` });
const param = (name) => ({ $ref: `#/components/parameters/${name}` });
const response = (name) => ({ $ref: `#/components/responses/${name}` });
const field = (type, source, description, extra = {}) => ({ type, description, 'x-emisell-source': source, ...extra });
const string = (source, description = 'Source value; plain text, never trusted HTML.') => field('string', source, description);
const nullable = (source, description = 'Source value; null is preserved.') => field(['string', 'null'], source, description);
const decimal = (source, isNullable = false) => field(isNullable ? ['string', 'null'] : 'string', source,
  'Exact source decimal as a string; no floating-point conversion, tax calculation or currency conversion. Trailing zeros are not guaranteed.', { pattern: '^-?[0-9]+(?:\\.[0-9]+)?$' });
const integer = (source, description = 'Source integer; not a computed balance.') => field('integer', source, description);
const boolean = (source) => field('boolean', source, 'Source boolean.');
const timestamp = (source) => field('string', source, 'UTC RFC 3339 timestamp.', { format: 'date-time' });
const opaque = (source) => field('string', source, 'Opaque Emisell ID. Never parse it or substitute a Shopify GID.', { minLength: 1, maxLength: 128 });
const object = (properties, description) => ({ type: 'object', additionalProperties: false, required: Object.keys(properties), properties, description });
const status = (source, values) => field('string', source, `Existing source values: ${values.join(', ')}. Preserve source spelling; consumers must tolerate new values. Not a Shopify status mapping.`, { 'x-extensible-enum': values });

const fulfillmentStatuses = ['FULFILLED', 'UNFULFILLED', 'IN_PROGRESS', 'PARTIALLY_FULFILLED', 'SCHEDULED', 'ON_HOLD', 'REQUEST_DECLINED', 'PENDING', 'REMOVED'];
const schemas = {
  Error: structuredClone(pilot.components.schemas.Error),
  ErrorResponse: structuredClone(pilot.components.schemas.ErrorResponse),
  PageMeta: object({
    nextCursor: { type: ['string', 'null'], maxLength: 1024, description: 'Opaque signed continuation cursor; null ends traversal. No total count or snapshot guarantee.' },
  }, 'Forward-only cursor pagination. Reuse the same filters and parent resource.'),
  Variant: object({
    id: opaque('Variant.id'), productId: opaque('Variant.productId'), name: string('Variant.name'), sku: string('Variant.sku'),
    price: decimal('Variant.price'), compareAtPrice: decimal('Variant.compareAtPrice', true),
    stock: integer('Variant.stock', 'Raw variant stock, NOT available-to-sell or per-location inventory. Currency is not supplied in this slice.'),
    barcode: nullable('Variant.barcode'), trackInventory: boolean('Variant.trackInventory'),
    continueSellingWhenOutOfStock: boolean('Variant.continueSellingWhenOutOfStock'),
  }, 'Base variant projection. Variant has no createdAt/updatedAt in the inspected Prisma schema; do not fabricate timestamps or incremental filters. No cost, raw shipping JSON, options or media.'),
  Location: object({
    id: opaque('StoreLocation.id'), name: string('StoreLocation.name'), isActive: boolean('StoreLocation.isActive'),
    isPrimary: boolean('StoreLocation.isPrimary'), isFulfillment: boolean('StoreLocation.isFulfillment'),
  }, 'Location identity and state only. No address, phone or timestamps. Both active and inactive locations are returned.'),
  InventoryLevel: object({
    id: opaque('StoreLocationItem.id'), locationId: opaque('StoreLocationItem.storeLocationId'), itemId: opaque('StoreLocationItem.itemId'),
    itemType: field('string', 'StoreLocationItem.itemType', 'Determines the tenant-checked Product or Variant join.', { enum: ['PRODUCT', 'VARIANT'] }),
    available: integer('StoreLocationItem.available', 'Stored location available quantity; not recomputed from Product.stock or Variant.stock. May be negative. Not a checkout reservation guarantee.'),
    onHand: integer('StoreLocationItem.onHand', 'Stored physical on-hand quantity. Do not equate it with available.'),
  }, 'Location ledger projection, subject to inventory reconciliation before rollout. The source has no merchantId or timestamps; both location ownership AND referenced item ownership must be checked.'),
  Order: object({
    id: opaque('Order.id'), number: integer('Order.number'), numberFormat: string('Order.numberFormat'),
    status: status('Order.status', ['UNPAID', 'PAID', 'PENDING', 'PROCESSING', 'REJECTED', 'SHIPPING', 'FAILED', 'REFUNDED', 'COMPLETED', 'CANCELED']),
    fulfillmentStatus: status('Order.fulfillmentStatus', fulfillmentStatuses),
    subtotal: decimal('Order.subtotal'), totalTax: decimal('Order.totalTax'), shippingFee: decimal('Order.shippingFee'),
    currencyCode: field(['string', 'null'], 'Order.currencySnapshot.code', 'Return only a validated ISO 4217 code from the stored snapshot; null for absent/invalid legacy data. Never default to IDR. No total/payment amount is inferred from these partial fields.', { pattern: '^[A-Z]{3}$' }),
    createdAt: timestamp('Order.createdAt'), updatedAt: timestamp('Order.updatedAt'),
  }, 'ONLINE_STORE orders only; exclude DRAFT_ORDER and ABANDONED_ORDER. No customer identity, contact, addresses, notes, raw snapshots, payment credentials, or guessed grandTotal. Order.status is NOT a separate financial/payment status.'),
  OrderItem: object({
    id: opaque('OrderItem.id'), orderId: opaque('OrderItem.orderId'), productId: nullable('OrderItem.productId'), variantId: nullable('OrderItem.variantId'),
    name: nullable('OrderItem.productSnapshot.name', 'Allowlisted string from the stored snapshot; null if absent or wrong type. No fallback lookup of current Product.'),
    sku: nullable('OrderItem.productSnapshot.sku', 'Base product snapshot SKU, not a variant SKU. Null if absent or wrong type.'),
    variantName: nullable('OrderItem.variantSnapshot.name', 'Allowlisted variant snapshot name, null if absent or wrong type.'),
    variantSku: nullable('OrderItem.variantSnapshot.sku', 'Allowlisted variant snapshot SKU, null if absent or wrong type.'),
    quantity: integer('OrderItem.quantity', 'Ordered quantity; not fulfillable, returned, refunded or remaining quantity.'),
    price: decimal('OrderItem.price'), lineTotal: decimal('OrderItem.lineTotal'), isCustom: boolean('OrderItem.isCustom'),
  }, 'Order-time projection, not current catalog data. No cost, raw snapshots, additionalProps or timeline. Currency follows the parent order. Paginated separately to bound response size.'),
  Customer: object({
    id: opaque('Customer.id'), firstName: nullable('Customer.firstName'), lastName: nullable('Customer.lastName'),
    email: nullable('Customer.email', 'Protected contact data; explicit read_customers consent and Emisell review required.'),
    phone: nullable('Customer.phone', 'Protected contact data; preserve source string, not a verified phone claim.'),
    createdAt: timestamp('Customer.createdAt'), updatedAt: timestamp('Customer.updatedAt'),
  }, 'Only Customer.merchantId ownership and deletedAt = null. Never use customerMerchants to broaden access. No User data, addresses, notes, spend, marketing permission inference or automatic enrichment. Guest profiles may have null fields.'),
  Fulfillment: object({
    id: opaque('OrderFulfillment.id'), orderId: opaque('OrderFulfillment.orderId'), number: nullable('OrderFulfillment.number'),
    fulfillmentStatus: status('OrderFulfillment.fulfillmentStatus', fulfillmentStatuses),
    shipmentStatus: status('OrderFulfillment.shipmentStatus', ['PENDING', 'PROCESSING', 'SHIPPED', 'IN_TRANSIT', 'DELIVERED', 'CANCELED', 'RETURNED', 'FAILED']),
    carrier: nullable('OrderFulfillment.carrier'), carrierName: nullable('OrderFulfillment.carrierName'),
    createdAt: timestamp('OrderFulfillment.createdAt'), updatedAt: timestamp('OrderFulfillment.updatedAt'),
  }, 'Fulfillment belongs to an ONLINE_STORE order and signed merchant. No cost, Shipment address/contact, raw trackingList or holdReasons. Tracking normalization and shipment booking remain a separately reviewed slice.'),
  FulfillmentItem: object({
    id: opaque('OrderFulfillmentItem.id'), fulfillmentId: opaque('OrderFulfillmentItem.fulfillmentId'), orderItemId: opaque('OrderFulfillmentItem.orderItemId'),
    quantity: integer('OrderFulfillmentItem.quantity', 'Quantity allocated to this fulfillment; not a remaining-to-ship calculation.'),
  }, 'Join the fulfillment AND order item to the same tenant-owned order. Refund/restock ledgers are excluded.'),
};

// Synthetic, non-executable illustrations. Never derive documentation examples
// from a merchant database, deployment or real customer records.
const examples = {
  Variant: { id: 'cmvariantdemo000000000001', productId: 'cmproductdemo000000000001', name: 'Blue / Medium', sku: 'DEMO-BLUE-M', price: '65000.5', compareAtPrice: null, stock: 8, barcode: null, trackInventory: true, continueSellingWhenOutOfStock: false },
  Location: { id: 'cmlocationdemo00000000001', name: 'Demo warehouse', isActive: true, isPrimary: true, isFulfillment: true },
  InventoryLevel: { id: 'cminventorydemo0000000001', locationId: 'cmlocationdemo00000000001', itemId: 'cmvariantdemo000000000001', itemType: 'VARIANT', available: 6, onHand: 8 },
  Order: { id: 'cmorderdemo00000000000001', number: 1001, numberFormat: '#1001', status: 'PAID', fulfillmentStatus: 'UNFULFILLED', subtotal: '65000.5', totalTax: '0', shippingFee: '10000', currencyCode: 'IDR', createdAt: '2026-09-03T08:00:00Z', updatedAt: '2026-09-03T08:05:00Z' },
  OrderItem: { id: 'cmorderitemdemo0000000001', orderId: 'cmorderdemo00000000000001', productId: 'cmproductdemo000000000001', variantId: 'cmvariantdemo000000000001', name: 'Demo product', sku: 'DEMO', variantName: 'Blue / Medium', variantSku: 'DEMO-BLUE-M', quantity: 1, price: '65000.5', lineTotal: '65000.5', isCustom: false },
  Customer: { id: 'cmcustomerdemo00000000001', firstName: 'Sample', lastName: 'Customer', email: 'customer@example.invalid', phone: null, createdAt: '2026-09-03T08:00:00Z', updatedAt: '2026-09-03T08:05:00Z' },
  Fulfillment: { id: 'cmfulfillmentdemo00000001', orderId: 'cmorderdemo00000000000001', number: '#1001-F1', fulfillmentStatus: 'UNFULFILLED', shipmentStatus: 'PENDING', carrier: null, carrierName: null, createdAt: '2026-09-03T08:00:00Z', updatedAt: '2026-09-03T08:05:00Z' },
  FulfillmentItem: { id: 'cmfulfillmentitemdemo0001', fulfillmentId: 'cmfulfillmentdemo00000001', orderItemId: 'cmorderitemdemo0000000001', quantity: 1 },
};
for (const [name, example] of Object.entries(examples)) schemas[name].example = example;
schemas.PageMeta.example = { nextCursor: null };

const parameters = Object.fromEntries(['MerchantIdHeader', 'InstallationIdHeader', 'RequestIdHeader', 'Limit', 'Cursor', 'UpdatedAfter'].map((name) => [name, structuredClone(pilot.components.parameters[name])]));
parameters.Limit.description = 'Maximum records in this page; default 50, range 1–100.';
parameters.UpdatedAfter.description = 'Strictly greater than this UTC Z timestamp, with at most three fractional digits. Only supported on endpoints listing a model with updatedAt. Not a delete feed or an exact incremental sync guarantee.';
parameters.UpdatedAfter.schema.pattern = '^\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}(?:\\.\\d{1,3})?Z$';
parameters.Cursor.description = 'Signed opaque cursor bound to app, merchant, installation, environment, API version, operation, parent ID, filters and ordering. Reject cross-context reuse with 400 invalid_cursor. Never parse on the client.';
for (const name of ['productId', 'variantId', 'locationId', 'orderId', 'customerId', 'fulfillmentId']) {
  parameters[name] = { name, in: 'path', required: true, description: 'Opaque resource ID. Parent and child must belong to the signed merchant.', schema: opaque(name) };
}

const definitions = [
  { path: '/products/{productId}/variants', id: 'listAppPlatformProductVariants', tag: 'Variants', schema: 'Variant', scope: 'read_products', list: true, models: ['Product', 'Variant'], tenant: 'Variant.merchantId = claim.merchant_id AND Product.id = productId AND Product.merchantId = claim.merchant_id AND Variant.productId = Product.id.', summary: 'List variants of a merchant-owned product' },
  { path: '/variants/{variantId}', id: 'getAppPlatformVariant', tag: 'Variants', schema: 'Variant', scope: 'read_products', models: ['Product', 'Variant'], tenant: 'Variant.id = variantId AND Variant.merchantId = claim.merchant_id AND Variant.product.merchantId = claim.merchant_id.', summary: 'Read a merchant-owned variant' },
  { path: '/locations', id: 'listAppPlatformLocations', tag: 'Inventory', schema: 'Location', scope: 'read_inventory', list: true, models: ['StoreLocation'], tenant: 'StoreLocation.merchantId = claim.merchant_id.', summary: 'List inventory locations' },
  { path: '/locations/{locationId}/inventory', id: 'listAppPlatformLocationInventory', tag: 'Inventory', schema: 'InventoryLevel', scope: 'read_inventory', list: true, models: ['StoreLocation', 'StoreLocationItem', 'Product', 'Variant'], tenant: 'StoreLocation.id = locationId AND StoreLocation.merchantId = claim.merchant_id. Join each itemId by itemType to a Product or Variant owned by this merchant (and its Product for variants); exclude orphaned or cross-tenant rows.', summary: 'Read stored inventory quantities at a location' },
  { path: '/orders', id: 'listAppPlatformOrders', tag: 'Orders', schema: 'Order', scope: 'read_orders', list: true, updated: true, models: ['Order'], tenant: 'Order.merchantId = claim.merchant_id AND Order.currentApp = ONLINE_STORE.', summary: 'List online-store orders without customer PII' },
  { path: '/orders/{orderId}', id: 'getAppPlatformOrder', tag: 'Orders', schema: 'Order', scope: 'read_orders', models: ['Order'], tenant: 'Order.id = orderId AND Order.merchantId = claim.merchant_id AND Order.currentApp = ONLINE_STORE.', summary: 'Read an online-store order summary' },
  { path: '/orders/{orderId}/items', id: 'listAppPlatformOrderItems', tag: 'Orders', schema: 'OrderItem', scope: 'read_orders', list: true, models: ['Order', 'OrderItem'], tenant: 'OrderItem.orderId = orderId AND OrderItem.order.merchantId = claim.merchant_id AND OrderItem.order.currentApp = ONLINE_STORE. Expose productId/variantId only if null or tenant-validated; otherwise redact to null.', summary: 'List bounded order-time line-item snapshots' },
  { path: '/customers', id: 'listAppPlatformCustomers', tag: 'Customers', schema: 'Customer', scope: 'read_customers', list: true, updated: true, models: ['Customer'], tenant: 'Customer.merchantId = claim.merchant_id AND Customer.deletedAt IS NULL; never broaden via customerMerchants or User.', summary: 'List approved customer profile fields' },
  { path: '/customers/{customerId}', id: 'getAppPlatformCustomer', tag: 'Customers', schema: 'Customer', scope: 'read_customers', models: ['Customer'], tenant: 'Customer.id = customerId AND Customer.merchantId = claim.merchant_id AND Customer.deletedAt IS NULL.', summary: 'Read an approved customer profile' },
  { path: '/orders/{orderId}/fulfillments', id: 'listAppPlatformOrderFulfillments', tag: 'Fulfillments', schema: 'Fulfillment', scope: 'read_fulfillments', list: true, updated: true, models: ['Order', 'OrderFulfillment'], tenant: 'OrderFulfillment.orderId = orderId AND OrderFulfillment.merchantId = claim.merchant_id AND Order.merchantId = claim.merchant_id AND Order.currentApp = ONLINE_STORE.', summary: 'List fulfillment state for an online-store order' },
  { path: '/fulfillments/{fulfillmentId}', id: 'getAppPlatformFulfillment', tag: 'Fulfillments', schema: 'Fulfillment', scope: 'read_fulfillments', models: ['Order', 'OrderFulfillment'], tenant: 'OrderFulfillment.id = fulfillmentId AND OrderFulfillment.merchantId = claim.merchant_id AND OrderFulfillment.order.merchantId = claim.merchant_id AND OrderFulfillment.order.currentApp = ONLINE_STORE.', summary: 'Read fulfillment state without shipment PII' },
  { path: '/fulfillments/{fulfillmentId}/items', id: 'listAppPlatformFulfillmentItems', tag: 'Fulfillments', schema: 'FulfillmentItem', scope: 'read_fulfillments', list: true, models: ['Order', 'OrderItem', 'OrderFulfillment', 'OrderFulfillmentItem'], tenant: 'Resolve fulfillmentId with signed merchant and ONLINE_STORE order. Join OrderFulfillmentItem.orderItem to that exact order; exclude inconsistent, orphaned or cross-tenant rows.', summary: 'List items allocated to a fulfillment' },
];

const commonHeaders = {
  'Cache-Control': { schema: { type: 'string', const: 'no-store' }, description: 'All success and error responses must be non-cacheable.' },
  'X-Request-ID': { schema: { type: 'string' }, description: 'Safe correlation identifier, no secrets or customer data.' },
};
const errors = {
  BadRequest: [400, 'validation_error / invalid_cursor: reject unknown or duplicate query parameters, malformed IDs, unsupported filters and cursor context mismatch.'],
  Unauthorized: [401, 'unauthorized: missing, expired or invalid App Gateway RS256 assertion.'],
  Forbidden: [403, 'insufficient_scope / context_forbidden / merchant_unavailable: invalid exact scope, claim/header mismatch, environment or allowlist failure, or missing/inactive/suspended merchant.'],
  NotFound: [404, 'not_found: resource or parent not visible in the signed merchant. Same response for foreign-tenant, deleted, excluded draft/abandoned or missing resources.'],
  RateLimited: [429, 'rate_limited: per app + installation + merchant + operation quota. Bounded backoff with jitter and Retry-After. Aggregate limit policy must be approved before general availability.'],
  Unavailable: [503, 'resource_disabled / resource_unavailable: capability disabled, dependency timeout, invalid projection or unavailable backend. Never return a fake empty success for failure.'],
};
const responses = Object.fromEntries(Object.entries(errors).map(([name, [code, description]]) => [name, {
  description, headers: { ...commonHeaders, ...(code === 429 ? { 'Retry-After': { schema: { type: 'integer', minimum: 1, maximum: 60 } } } : {}), ...(code === 401 ? { 'WWW-Authenticate': { schema: { type: 'string' } } } : {}) },
  content: { 'application/json': { schema: ref('ErrorResponse') } },
}]));
const paths = {};
for (const definition of definitions) {
  const { path, id, tag, schema, scope, list, updated, models, tenant, summary } = definition;
  const responseName = `${schema}${list ? 'List' : ''}Response`;
  schemas[responseName] = object({ data: list ? { type: 'array', maxItems: 100, items: ref(schema) } : ref(schema), ...(list ? { meta: ref('PageMeta') } : {}) }, 'Only explicitly projected fields may be returned.');
  const sorting = list ? updated ? 'updatedAt DESC, id DESC' : 'id ASC' : 'Not applicable (single resource).';
  paths[`/internal/app-platform/v1${path}`] = { get: {
    operationId: id, tags: [tag], summary,
    description: `DESIGN ONLY — not implemented in api-service or App Gateway. Requires exact assertion scope [${scope}]. ${schemas[schema].description} Tenant rule: ${tenant} ${list ? `Pagination: ${sorting}; limit 1–100, default 50. ${updated ? 'updatedAfter is supported.' : 'updatedAfter is not supported.'} Empty owned collections return data: [] and nextCursor: null; a missing/foreign parent returns 404. Concurrent writes can affect traversal; not a point-in-time export or deletion feed.` : 'A missing or foreign-tenant record returns the same 404.'}`,
    'x-emisell-implementation-status': 'planned',
    'x-emisell-required-scopes': [scope],
    'x-emisell-backend': { models, tenantRule: tenant, pagination: sorting, providerTargetPath: `/v1${path}` },
    security: [{ appGatewayAssertion: [] }],
    parameters: [...[...path.matchAll(/\{([^}]+)\}/g)].map((match) => param(match[1])), ...['MerchantIdHeader', 'InstallationIdHeader', 'RequestIdHeader'].map(param), ...(list ? ['Limit', 'Cursor'].map(param) : []), ...(updated ? [param('UpdatedAfter')] : [])],
    responses: { '200': { description: `Projected ${list ? 'page' : 'resource'} after merchant policy and scope validation.`, headers: commonHeaders, content: { 'application/json': { schema: ref(responseName) } } }, ...Object.fromEntries(Object.entries(errors).map(([name, [code]]) => [code, response(name)])) },
  } };
}

const spec = {
  openapi: '3.1.0',
  info: { title: 'Emisell Resource API — Backend Blueprint', version: '0.1.0-draft', description: 'Design-only read expansion for api-service. Twelve target operations grounded in inspected Emisell Prisma models; no running routes, migrations, rollout or SDK package are created by this contract. The existing two product pilot operations remain exclusively in emisell-resource-openapi.json.' },
  'x-emisell-implementation': { status: 'planned', defaultEnabled: false, implementedPaths: [], targetPaths: Object.keys(paths), owner: 'emisell-api-service', consumer: 'emisell-app-platform', publicScopeAvailability: 'planned' },
  'x-emisell-handoff': {
    reviewedOn: '2026-09-03',
    source: 'api-service/prisma/models (local source inspection; not a deployed database audit)',
    protocol: 'REST /v1 + OpenAPI 3.1. SDK patterns inspired by Shopify; not wire-compatible with Shopify SDK or GraphQL.',
    decisions: [
      { topic: 'Client SDK', shopify: 'Admin API client configures shop, API version and access token; supports typed requests and bounded retries.', emisell: 'A future server SDK targets App Gateway with one installation token and explicit v1. No SDK package is published here; never send a merchant selector or internal assertion from a provider.', source: 'https://github.com/Shopify/shopify-app-js/blob/main/packages/api-clients/admin-api-client/README.md' },
      { topic: 'Transport', shopify: 'New public Shopify apps use GraphQL; REST Admin API is legacy.', emisell: 'Keep the existing REST product pilot. Do not introduce GraphQL, Shopify GIDs or SDK compatibility without a separate protocol decision.', source: 'https://shopify.dev/docs/api/admin-rest/latest' },
      { topic: 'Scope', shopify: 'Apps request access scopes during authorization; write access includes read access.', emisell: 'Require the exact read scope listed on each operation. Do not infer write → read inheritance: it is not a rule in the Emisell implementation.', source: 'https://shopify.dev/docs/api/usage/access-scopes' },
      { topic: 'Pagination', shopify: 'GraphQL connections use cursors and pageInfo.', emisell: 'Keep data + meta.nextCursor, default 50/max 100. Models without timestamps use id ASC and have no updatedAfter filter.', source: 'https://shopify.dev/docs/api/usage/pagination-graphql' },
      { topic: 'Versioning', shopify: 'Versioned APIs follow a quarterly release schedule.', emisell: 'Separate app configuration version, API major version (/v1), and document draft version. No quarterly compatibility promise or silent fallback.', source: 'https://shopify.dev/docs/api/usage/versioning' },
      { topic: 'Throttling', shopify: 'GraphQL Admin API uses calculated query cost.', emisell: 'Use HTTP 429 and Retry-After; define aggregate per-installation quotas before rollout. Do not claim Shopify limits or query-cost accounting.', source: 'https://shopify.dev/docs/api/usage/limits' },
    ],
    acceptance: [
      'Implement one resource slice in api-service with fail-closed RS256 verification, exact endpoint scope, matching routing headers and active/not-suspended merchant policy on the primary database.',
      'Add allowlisted projections and tenant predicates on every parent/child relation. Never return a Prisma object, raw JSON snapshot, cost, credential or unrelated PII.',
      'Add the matching App Gateway route/projection only after backend tests pass. Provider uses installation token; internal 401/403 must map to safe 503, while missing provider scope stays 403.',
      'Prove A/B merchant isolation, bad signature/audience/kid/expiry, wrong scope, cursor/filter/parent mismatch, revoked installation, disabled app access, suspended merchant and safe errors using disposable fixtures.',
      'Test null/legacy data, source decimal precision, empty pages, foreign parents, orphaned inventory/fulfillment relations, and payload/time limits. Validate response bodies against the OpenAPI schema.',
      'Keep public scopes planned until projection/privacy approval, key rotation, rate-limit capacity, deployment verification and rollback review. Move implemented endpoints into the runtime specs only after evidence exists.',
    ],
    deferred: ['All write_* mutations and idempotency/replay design', 'Customer addresses, marketing permission and privacy event pipeline', 'Shipment booking, normalized tracking and Payment/Shipping execution', 'GraphQL, bulk exports, webhooks for resource changes and published SDK package'],
  },
  servers: [{ url: 'https://emisell-backend.example.invalid', description: 'Design-only placeholder. This origin is not a deployed backend.' }],
  tags: [...new Set(definitions.map((entry) => entry.tag))].map((name) => ({ name, description: `Planned ${name.toLowerCase()} projection; implementation, data review and deployment are still required.` })),
  security: [{ appGatewayAssertion: [] }],
  paths,
  components: {
    securitySchemes: { appGatewayAssertion: { type: 'http', scheme: 'bearer', bearerFormat: 'JWT RS256', description: 'App Gateway-only assertion; NOT an installation token, browser session or merchant ID. Use dedicated kid/public-key verification; iss=emisell-app-platform, aud=emisell-api-service, sub=app-gateway; require iat/nbf/exp (maximum lifetime 60 seconds, zero clock tolerance), jti, app_id, installation_id, merchant_id, configured environment and scope exactly equal to x-emisell-required-scopes. Header merchant/installation must match signed claims. Keep scope arrays empty in OpenAPI security: HTTP bearer is not OAuth; required internal scopes are explicit operation metadata.' } },
    parameters, responses, schemas,
  },
};
const output = new URL('docs/emisell-resource-blueprint.openapi.json', root);
const expected = `${JSON.stringify(spec, null, 2)}\n`;
if (process.argv.includes('--check')) {
  if (readFileSync(output, 'utf8') !== expected) throw new Error('Resource blueprint drift: run npm run sync:resource-blueprint');
} else writeFileSync(output, expected);
