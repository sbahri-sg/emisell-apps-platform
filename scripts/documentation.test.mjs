import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { audienceFor, buildPostman, buildReference, contracts, operations, resolve, schemaExample, selectSpec } from '../lib/documentation/contracts.mjs';
import { serveDocumentation } from '../lib/documentation/access.mjs';

const read = (file) => JSON.parse(readFileSync(new URL(`../docs/${file}.json`, import.meta.url), 'utf8'));
const sources = { platform: read('openapi'), provider: read('provider-openapi'), resource: read('emisell-resource-openapi'), resourceBlueprint: read('emisell-resource-blueprint.openapi'), shippingProvider: read('shipping-provider.openapi') };
const flatten = (collection) => collection.item.flatMap((group) => group.item);

test('documentation has exactly two primary integration entry points', () => {
  assert.deepEqual(contracts.filter((contract) => contract.visibility === 'primary').map((contract) => contract.id), ['emisell', 'provider']);
  assert.ok(contracts.filter((contract) => contract.visibility === 'operator').length > 0);
  assert.ok(contracts.filter((contract) => contract.visibility === 'roadmap').length > 0);
  assert.ok(contracts.filter((contract) => contract.planned).every((contract) => contract.visibility === 'roadmap'));
  assert.equal(contracts.find((contract) => contract.id === 'provider').title, 'Partner API');
  assert.equal(contracts.find((contract) => contract.id === 'emisell').title, 'Internal Emisell Gateway');
});

test('integration inspection is separated by caller and never declares end-to-end success', () => {
  const platform = sources.platform;
  const routes = operations(platform).filter(({ path }) => path.endsWith('/integration-readiness'));
  assert.equal(routes.length, 2);
  assert.equal(platform.components.schemas.IntegrationReadiness.properties.endToEndVerified.const, false);
  for (const route of routes) {
    assert.equal(route.method, 'get');
    assert.ok(route.operation.parameters.every((parameter) => parameter.in === 'path'));
    assert.deepEqual(route.operation.security, route.path.startsWith('/v1/internal/') ? [{ adminSessionCookie: [] }] : [{ sessionCookie: [] }, { bearerAuth: [] }]);
    const expected = route.path.startsWith('/v1/internal/') ? 'admin' : 'developer';
    assert.equal(audienceFor(route.path, route.method, sources.provider), expected);
  }
  assert.doesNotMatch(JSON.stringify(sources.provider), /IntegrationReadiness/);
  const schema = platform.components.schemas.UpdateCatalogListingRequest;
  assert.deepEqual(schema.dependentRequired.expectedAppRevision, ['expectedActiveVersionId']);
  assert.ok(schema.properties.expectedAppRevision.minimum >= 1);
});

test('billing reference separates consent, service assertions and provider access; Postman is guarded', () => {
  const billing = operations(sources.platform).filter(({ operation }) => operation['x-emisell-app-billing']);
  assert.equal(billing.length, 11);
  for (const contract of ['developer', 'emisell', 'provider']) {
    const entries = flatten(buildPostman(sources, contract)).filter((entry) => entry.request.url.raw.includes('/billing') || entry.request.url.raw.includes('/plans') || entry.request.url.raw.includes('/installation-billing'));
    assert.ok(entries.length > 0);
    for (const entry of entries) assert.match(entry.event[0].script.exec.join('\n'), /enable_app_billing.*skipRequest/);
  }
  const specs = sources.platform.components.schemas;
  assert.equal(specs.ApproveAppSubscription.properties.acceptRecurringCharge.const, true);
  assert.equal(specs.QuoteAppSubscription.properties.merchantId, undefined);
  assert.equal(specs.SyncAppBillingAccount.properties.merchantId, undefined);
  assert.equal(specs.AppInvoicePayment.properties.merchantId, undefined);
  assert.deepEqual(sources.platform.paths['/v1/integrations/emisell/billing/invoices'].post.security, [{ emisellBackendBearer: [] }]);
  assert.deepEqual(sources.provider.paths['/v1/installation-billing'].get.security, [{ installationBearer: [] }]);
  assert.equal(sources.provider.components.schemas.AppBillingInvoice, undefined);
});

test('shipping rate bridge is internal, selector-free and guarded in Postman', () => {
  const path = '/v1/integrations/emisell/shipping/rates/calculate';
  const operation = sources.platform.paths[path]?.post;
  assert.ok(operation);
  assert.equal(operation.operationId, 'calculateEmisellShippingRates');
  assert.equal(operation['x-emisell-shipping-rate-bridge'], true);
  assert.deepEqual(operation.security, [{ emisellBackendBearer: [] }]);
  assert.equal(audienceFor(path, 'post', sources.provider), 'emisell');
  const schema = resolve(sources.platform, operation.requestBody.content['application/json'].schema);
  assert.deepEqual(Object.keys(schema.properties).sort(), ['destination', 'origin', 'weight']);
  assert.equal(schema.additionalProperties, false);
  for (const selector of ['merchantId', 'courier', 'provider', 'credentialId', 'installationId', 'extensionId', 'runtimeUrl']) {
    assert.equal(schema.properties[selector], undefined);
  }
  const postman = flatten(buildPostman(sources, 'emisell')).find((entry) => entry.request.url.raw.endsWith(path));
  assert.ok(postman);
  assert.match(postman.event[0].script.exec.join('\n'), /enable_api_kurir_rate_bridge.*skipRequest/);
  const provider = selectSpec(sources, 'provider');
  assert.equal(provider.paths[path], undefined);
  assert.equal(provider.components.schemas.ShippingRateCalculateRequest, undefined);
});

test('gateway integration map is generated from OpenAPI and exposed only to the Emisell reference', () => {
  const integrations = sources.platform['x-emisell-gateway-integrations'];
  assert.equal(integrations.length, 1);
  const integration = integrations[0];
  assert.equal(integration.id, 'shipping-rate-calculation');
  assert.equal(integration.status, 'pilot_default_off');
  assert.equal(integration.operationId, 'calculateEmisellShippingRates');
  assert.deepEqual(integration.hops.map((hop) => hop.path), [
    '/v1/integrations/emisell/shipping/rates/calculate',
    '/api/v1/calculate/district/domestic-cost',
  ]);
  assert.deepEqual(integration.hops[0].fields, ['origin', 'destination', 'weight']);
  assert.ok(integration.hops[0].forbiddenSelectors.includes('merchantId'));
  assert.ok(integration.hops[1].forbiddenSelectors.includes('courier'));
  assert.deepEqual(integration.ownership.map((item) => item.owner), [
    'Backend Emisell', 'App Platform', 'API Kurir', 'API Kurir', 'API Kurir', 'Backend Emisell',
  ]);
  assert.equal(buildReference(sources, 'emisell').gatewayIntegrations.length, 1);
  for (const contract of contracts.filter((entry) => entry.id !== 'emisell')) {
    assert.deepEqual(buildReference(sources, contract.id).gatewayIntegrations, []);
  }
  assert.doesNotMatch(JSON.stringify(integration), /api-kurir-sandbox-service-key|BEGIN .*PRIVATE KEY|provider-api-key|Authorization: Bearer/);
});

test('managed credentials have separate runtime authentication and guarded operator-only provisioning examples', () => {
  const managed = selectSpec(sources, 'managed');
  const entries = operations(managed);
  assert.equal(entries.length, 1);
  assert.equal(entries[0].path, '/v1/runtime/extension-credentials/resolve');
  assert.deepEqual(entries[0].operation.security, [{ ExtensionRuntimeToken: [] }]);
  assert.deepEqual(Object.keys(managed.components.schemas.ResolveExtensionCredentialRequest.properties), ['scope']);
  assert.equal(managed.components.schemas.ResolveExtensionCredentialRequest.additionalProperties, false);
  const runtime = flatten(buildPostman(sources, 'managed'))[0];
  assert.equal(runtime.request.auth.bearer[0].value, '{{extension_runtime_token}}');
  assert.match(runtime.event[0].script.exec.join('\n'), /enable_managed_extensions/);
  const controls = flatten(buildPostman(sources, 'admin')).filter((entry) => entry.request.url.raw.includes('/connection'));
  assert.equal(controls.length, 4);
  for (const entry of controls) {
    assert.equal(entry.request.auth.type, 'noauth');
    assert.ok(entry.request.header.some((header) => header.key === 'Cookie' && header.value === 'emisell_admin_session={{admin_session}}'));
    assert.match(entry.event[0].script.exec.join('\n'), /pm.execution.skipRequest/);
  }
  const requestBody = JSON.parse(controls.find((entry) => entry.request.method === 'PUT').request.body.raw);
  assert.deepEqual(requestBody.secret, { apiKey: '<provider-api-key>' });
  assert.equal(requestBody.revision, 0);
  assert.doesNotMatch(JSON.stringify(selectSpec(sources, 'provider')), /ExtensionConnection|ExtensionRuntimeToken|\/v1\/runtime\//);
  const metadata = sources.platform.components.schemas.ExtensionConnection;
  for (const key of ['secret', 'tokenHash', 'ciphertext', 'runtimeToken']) assert.equal(metadata.properties[key], undefined);
  const resolved = buildReference(sources, 'managed').operations[0].responses.find((entry) => entry.status === '200').example;
  assert.deepEqual(resolved.data.secret, { apiKey: '<provider-api-key>' });
});

test('product contracts preserve base-field semantics and distinguish code verification from deployment', () => {
  const verification = sources.resource['x-emisell-implementation'].verification;
  assert.equal(verification.deploymentVerified, false);
  assert.equal(verification.gatewayStorage, 'in_memory_test_repository');
  assert.equal(verification.resourceStorage, 'current_api_service_prisma_schema_postgresql_17');
  const product = sources.resource.components.schemas.Product;
  assert.match(product.properties.price.description, /base Product.price/);
  assert.match(product.properties.stock.description, /Not stock minus soldCount/);
  assert.match(product.properties.description.description, /untrusted HTML/);
  for (const source of [sources.platform, sources.provider]) assert.deepEqual(source.components.schemas.ResourceProduct, product);
  assert.match(sources.resource.components.responses.Forbidden.description, /merchant_unavailable/);
});

test('resource blueprint stays design-only, separate from runtime and public scope activation', () => {
  const spec = selectSpec(sources, 'resource-blueprint');
  const implementation = spec['x-emisell-implementation'];
  assert.equal(implementation.status, 'planned');
  assert.equal(implementation.defaultEnabled, false);
  assert.deepEqual(implementation.implementedPaths, []);
  assert.deepEqual(implementation.targetPaths.sort(), Object.keys(spec.paths).sort());
  const entries = operations(spec);
  assert.equal(entries.length, 12);
  const catalog = readFileSync(new URL('../services/app-gateway/internal/application/scope_catalog.go', import.meta.url), 'utf8');
  for (const entry of entries) {
    const operation = entry.operation;
    assert.equal(entry.method, 'get');
    assert.equal(operation['x-emisell-implementation-status'], 'planned');
    assert.deepEqual(operation.security, [{ appGatewayAssertion: [] }]);
    const scope = operation['x-emisell-required-scopes'];
    assert.equal(scope.length, 1);
    assert.match(scope[0], /^read_/);
    const availability = catalog.match(new RegExp(`Scope: "${scope[0]}"[\\s\\S]*?Availability: (domain.ScopeAvailability\\w+)`));
    assert.equal(availability?.[1], 'domain.ScopeAvailabilityPlanned');
    assert.equal(sources.resource.paths[entry.path], undefined);
    assert.equal(sources.provider.paths[operation['x-emisell-backend'].providerTargetPath], undefined);
    assert.equal(sources.platform.paths[operation['x-emisell-backend'].providerTargetPath], undefined);
    assert.match(operation['x-emisell-backend'].tenantRule, /merchant/);
  }
  assert.equal(operations(sources.resource).length, 2);
});

test('blueprint reference exposes source mappings, scope and official research from the contract', () => {
  const reference = buildReference(sources, 'resource-blueprint');
  assert.equal(reference.selected.planned, true);
  assert.equal(reference.handoff.decisions.length, 6);
  assert.ok(reference.handoff.acceptance.length > 0);
  for (const decision of reference.handoff.decisions) {
    assert.match(decision.source, /^https:\/\/(shopify\.dev\/|github\.com\/Shopify\/)/);
  }
  for (const operation of reference.operations) {
    assert.equal(operation.backend.scopes.length, 1);
    assert.ok(operation.backend.models.length > 0);
    assert.ok(operation.backend.fields.length > 0);
    for (const field of operation.backend.fields) assert.match(field.source, /^[A-Z][A-Za-z]+\.[A-Za-z]+/);
    assert.match(operation.curl, /^# PLANNED/);
    for (const status of ['400', '401', '403', '404', '429', '503']) assert.ok(operation.responses.some((entry) => entry.status === status));
  }
  assert.equal(buildReference(sources, 'provider').handoff, null);
});

test('blueprint preserves source semantics and does not invent commerce fields or timestamp filters', () => {
  const spec = sources.resourceBlueprint;
  const { Variant, InventoryLevel, Order, Customer, Fulfillment, OrderItem } = spec.components.schemas;
  assert.equal(Variant.properties.updatedAt, undefined);
  assert.equal(Variant.properties.price.type, 'string');
  assert.equal(InventoryLevel.properties.available['x-emisell-source'], 'StoreLocationItem.available');
  assert.equal(InventoryLevel.properties.onHand['x-emisell-source'], 'StoreLocationItem.onHand');
  assert.equal(Order.properties.grandTotal, undefined);
  assert.equal(Order.properties.paymentStatus, undefined);
  assert.deepEqual(Order.properties.currencyCode.type, ['string', 'null']);
  assert.equal(Order.properties.currencyCode.default, undefined);
  assert.equal(OrderItem.properties.name['x-emisell-source'], 'OrderItem.productSnapshot.name');
  for (const schema of [Variant, InventoryLevel, Order, Customer, Fulfillment, OrderItem]) {
    assert.equal(schema.additionalProperties, false);
    for (const field of ['cost', 'userId', 'merchantId', 'address', 'note', 'trackingList', 'currencySnapshot', 'productSnapshot', 'credentials']) assert.equal(schema.properties[field], undefined);
  }
  for (const { operation } of operations(spec)) {
    const schema = resolve(spec, operation.responses['200'].content['application/json'].schema);
    const data = schema.properties.data;
    const projection = resolve(spec, data.items ?? data);
    const hasUpdatedAfter = operation.parameters.some((entry) => entry.$ref.endsWith('/UpdatedAfter'));
    assert.equal(hasUpdatedAfter, Boolean(data.items && projection.properties.updatedAt));
    if (data.items) {
      assert.equal(data.maxItems, 100);
      assert.equal(operation['x-emisell-backend'].pagination, hasUpdatedAfter ? 'updatedAt DESC, id DESC' : 'id ASC');
    }
  }
  const inventory = spec.paths['/internal/app-platform/v1/locations/{locationId}/inventory'].get;
  assert.match(inventory['x-emisell-backend'].tenantRule, /itemId.*itemType/);
  assert.match(spec.paths['/internal/app-platform/v1/customers'].get['x-emisell-backend'].tenantRule, /deletedAt IS NULL/);
  assert.match(spec.paths['/internal/app-platform/v1/orders'].get['x-emisell-backend'].tenantRule, /ONLINE_STORE/);
});

test('blueprint Postman skips requests even when an individual item is copied from its collection', () => {
  const collection = buildPostman(sources, 'resource-blueprint');
  assert.match(collection.info.name, /PLANNED/);
  assert.match(collection.variable.find((item) => item.key === 'baseUrl').value, /example\.invalid$/);
  for (const script of [collection.event[0].script, ...flatten(collection).map((entry) => entry.event[0].script)]) {
    assert.ok(script.exec.includes('pm.execution.skipRequest();'));
    assert.doesNotMatch(script.exec.join('\n'), /enable_resource_pilot|pm.environment|if\s*\(/);
  }
  assert.equal(flatten(collection).length, 12);
});

test('every implemented gateway method/path is documented and assigned to one audience', () => {
  const server = readFileSync(new URL('../services/app-gateway/internal/httpapi/server.go', import.meta.url), 'utf8');
  const routes = [...server.matchAll(/HandleFunc\("(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS) ([^"]+)"/g)].map((match) => `${match[1]} ${match[2]}`).sort();
  const documented = operations(sources.platform).map(({ method, path }) => `${method.toUpperCase()} ${path}`).sort();
  assert.deepEqual(documented, routes);
  const assigned = operations(sources.platform).map(({ path, method }) => audienceFor(path, method, sources.provider));
  assert.equal(assigned.length, routes.length);
  assert.throws(() => audienceFor('/v1/new-unclassified-route', 'get', sources.provider), /Unclassified/);
});

test('references and Postman methods/paths are generated from the selected spec', () => {
  for (const contract of contracts) {
    const spec = selectSpec(sources, contract.id);
    const reference = buildReference(sources, contract.id);
    const collection = buildPostman(sources, contract.id);
    const expected = operations(spec).map(({ method, path }) => `${method.toUpperCase()} ${path}`).sort();
    assert.deepEqual(reference.operations.map((operation) => `${operation.method} ${operation.path}`).sort(), expected);
    assert.deepEqual(flatten(collection).map(({ request }) => `${request.method} /${request.url.path.join('/').replace(/\{\{([^}]+)\}\}/g, '{$1}')}`).sort(), expected);
    assert.equal(reference.selected.count, expected.length);
    assert.equal(collection.info.schema, 'https://schema.getpostman.com/json/collection/v2.1.0/collection.json');
    for (const variable of collection.variable) if (variable.key !== 'baseUrl') assert.equal(variable.value, '', variable.key);
  }
});

test('each API contract has an OpenAPI-backed journey, contents and valid quick-start links', () => {
  for (const contract of contracts) {
    const reference = buildReference(sources, contract.id);
    const operationsById = new Map(reference.operations.map((operation) => [operation.id, operation]));
    assert.ok(reference.flow.status);
    assert.equal(reference.selected.status, reference.flow.status);
    for (const summary of reference.contracts) assert.ok(summary.status, `${summary.id}: missing visible contract status`);
    assert.ok(reference.flow.outcome);
    assert.ok(reference.flow.actors.length >= 3);
    assert.ok(reference.flow.routeFamilies.length > 0);
    assert.ok(reference.flow.quickStart.length > 0);
    assert.equal(new Set(reference.flow.quickStart.map((step) => step.id)).size, reference.flow.quickStart.length);
    for (const step of reference.flow.quickStart) {
      const operation = operationsById.get(step.id);
      assert.ok(operation, `${contract.id}: ${step.id}`);
      assert.deepEqual(step, { id: operation.id, method: operation.method, path: operation.path, summary: operation.summary });
    }
    const groupedIds = reference.flow.groups.flatMap((group) => {
      assert.equal(group.count, group.operationIds.length);
      assert.ok(group.id.startsWith('group-'));
      return group.operationIds;
    });
    assert.deepEqual(groupedIds.sort(), [...operationsById.keys()].sort());
    assert.equal(new Set(groupedIds).size, reference.operations.length);
  }
});

test('all filtered OpenAPI refs and security schemes resolve, with no internal schemas in provider export', () => {
  for (const contract of contracts) {
    const spec = selectSpec(sources, contract.id);
    function visit(value) {
      if (!value || typeof value !== 'object') return;
      if (value.$ref) assert.ok(resolve(spec, value));
      for (const alternative of value.security ?? []) for (const key of Object.keys(alternative)) assert.ok(spec.components.securitySchemes[key]);
      Object.values(value).forEach(visit);
    }
    visit(spec);
  }
  const provider = JSON.stringify(selectSpec(sources, 'provider'));
  assert.doesNotMatch(provider, /DeveloperApplication|\/v1\/internal\/|CreateAppRequest/);
  assert.ok(operations(selectSpec(sources, 'admin')).every(({ path }) => path.startsWith('/v1/internal/') || path.startsWith('/auth/admin/')));
});

test('Postman chooses correct credentials for each caller and cookie-only requests include CSRF', () => {
  const provider = flatten(buildPostman(sources, 'provider'));
  const profile = provider.find(({ request }) => request.url.raw.endsWith('/v1/merchant/profile')).request;
  assert.equal(profile.auth.type, 'bearer');
  assert.equal(profile.auth.bearer[0].value, '{{installation_token}}');
  const token = provider.find(({ request }) => request.url.raw.endsWith('/oauth/token')).request;
  assert.equal(token.auth.type, 'basic');
  assert.equal(token.body.mode, 'urlencoded');
  assert.ok(token.body.urlencoded.some((entry) => entry.key === 'code_verifier'));
  const merchant = flatten(buildPostman(sources, 'emisell'));
  const consent = merchant.find(({ request }) => request.url.raw.endsWith('/v1/merchant/oauth/authorize')).request;
  assert.ok(consent.header.some((header) => header.key === 'Cookie' && header.value === 'emisell_merchant_session={{merchant_session}}'));
  assert.ok(consent.header.some((header) => header.key === 'X-CSRF-Token' && header.value === '{{merchant_csrf}}'));
  const grant = merchant.find(({ request }) => request.url.raw.endsWith('/v1/integrations/emisell/merchant-session-grants')).request;
  assert.equal(grant.auth.bearer[0].value, '{{emisell_backend_jwt}}');
});

test('resource pilot is truthfully documented, gated in Postman and free of live secrets', () => {
  const spec = selectSpec(sources, 'resource');
  assert.equal(spec['x-emisell-implementation'].status, 'implemented_gated');
  assert.equal(spec['x-emisell-implementation'].defaultEnabled, false);
  assert.deepEqual(spec['x-emisell-implementation'].implementedPaths.sort(), Object.keys(spec.paths).sort());
  const postman = buildPostman(sources, 'resource');
  assert.match(postman.event[0].script.exec.join('\n'), /pm\.execution\.skipRequest\(\)/);
  assert.match(postman.event[0].script.exec.join('\n'), /enable_resource_pilot/);
  const providerProducts = flatten(buildPostman(sources, 'provider')).filter((item) => item.request.url.raw.includes('/v1/products'));
  assert.equal(providerProducts.length, 2);
  for (const item of providerProducts) assert.match(item.event[0].script.exec.join('\n'), /pm\.execution\.skipRequest\(\)/);
  assert.match(postman.variable.find((v) => v.key === 'baseUrl').value, /example\.invalid/);
  assert.equal(schemaExample(sources.platform, { type: 'string', example: 'do-not-leak' }, 'clientSecret'), '<clientSecret>');
  assert.equal(schemaExample(sources.platform, { type: 'string', writeOnly: true, example: 'do-not-leak' }, 'code'), '<code>');
  for (const contract of contracts) {
    const generated = JSON.stringify({ reference: buildReference(sources, contract.id), postman: buildPostman(sources, contract.id) });
    assert.doesNotMatch(generated, /emisell-local-dev-token|emisell-backend-local-token|BEGIN (?:RSA )?PRIVATE KEY|eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+/);
  }
});

test('admin request examples come from OpenAPI and lifecycle mutations document authentication/validation failures', () => {
  const reference = buildReference(sources, 'admin');
  const create = reference.operations.find((entry) => entry.id === 'createDeveloperApplication');
  assert.deepEqual(create.request.example, sources.platform.components.schemas.CreateDeveloperApplicationRequest.example);
  assert.deepEqual(create.request.example.requestedScopes, ['read_merchant']);
  for (const entry of reference.operations) {
    if (entry.path.startsWith('/auth/admin/')) continue;
    assert.ok(entry.responses.some((response) => response.status === '401'), entry.id);
    assert.ok(entry.responses.some((response) => response.status === '403'), entry.id);
    if (entry.request) assert.ok(entry.responses.some((response) => response.status === '422'), entry.id);
  }
  const login = reference.operations.find((entry) => entry.id === 'loginAdmin');
  assert.ok(login.request.example.password.startsWith('<'));
  assert.ok(login.responses.some((response) => response.status === '429'));
  assert.deepEqual(sources.platform.paths['/auth/admin/session'].get.security, [{ adminSessionCookie: [] }]);
  assert.deepEqual(sources.platform.paths['/auth/admin/logout'].post.security, [{ adminSessionCookie: [] }]);
});

const request = (headers = {}) => new Request('http://localhost:3003/admin/docs/content?format=openapi', { headers });
const options = (overrides = {}) => ({ gatewayUrl: 'https://gateway.example.com', render: async () => Response.json({ internal: 'contract' }), ...overrides });

test('anonymous docs requests fail before loading or querying any data', async () => {
  const response = await serveDocumentation(request(), options({ fetcher: () => assert.fail('No identity request expected'), render: () => assert.fail('No docs should be loaded') }));
  assert.equal(response.status, 401);
  assert.match(response.headers.get('Cache-Control'), /no-store/);
});

test('developer tokens, sessions and forged operator headers cannot read internal docs', async () => {
  const response = await serveDocumentation(request({ Cookie: 'emisell_session=developer', Authorization: 'Bearer test-credential', 'X-Platform-Operator': 'true' }), options({
    fetcher: () => assert.fail('Developer credential must not reach the admin identity endpoint'), render: () => assert.fail('Non-admin must not load docs'),
  }));
  assert.equal(response.status, 401);
});

test('operator auth forwards only the admin session to the fixed gateway', async () => {
  const response = await serveDocumentation(request({ Cookie: 'analytics=private; emisell_admin_session=test-session; emisell_session=developer; emisell_merchant_session=merchant-private', Authorization: 'Bearer test-credential', 'X-Organization-Id': 'org-test', 'X-Platform-Operator': 'true', 'X-Forwarded-Host': 'attacker.example' }), options({
    fetcher: async (url, init) => {
      assert.equal(String(url), 'https://gateway.example.com/auth/admin/session');
      assert.equal(init.headers.get('Cookie'), 'emisell_admin_session=test-session');
      assert.equal(init.headers.get('Authorization'), null);
      assert.equal(init.headers.get('X-Organization-Id'), null);
      assert.equal(init.headers.get('X-Platform-Operator'), null);
      assert.equal(init.redirect, 'manual');
      assert.equal(init.cache, 'no-store');
      return Response.json({ data: { platformOperator: true } });
    },
  }));
  assert.equal(response.status, 200);
  assert.match(response.headers.get('Cache-Control'), /private, no-store/);
  assert.equal(response.headers.get('X-Content-Type-Options'), 'nosniff');
  assert.doesNotMatch(await response.text(), /test-credential|test-session|merchant-private/);
});

test('expired auth, unavailable gateway and malformed identity fail closed for downloads too', async () => {
  for (const [fetcher, expected] of [
    [async () => new Response('', { status: 401 }), 401],
    [async () => new Response('', { status: 403 }), 403],
    [async () => new Response('', { status: 500 }), 502],
    [async () => new Response('', { status: 302, headers: { Location: 'https://untrusted.example' } }), 502],
    [async () => { throw new Error('Private transport detail'); }, 502],
    [async () => new Response('invalid-json'), 502],
  ]) {
    const response = await serveDocumentation(request({ Cookie: 'emisell_admin_session=expired' }), options({ fetcher, render: () => assert.fail('Must not render') }));
    assert.equal(response.status, expected);
    assert.doesNotMatch(await response.text(), /Private transport detail/);
  }
});

test('custom admin cookie is supported without accepting merchant-only identity', async () => {
  const custom = await serveDocumentation(request({ Cookie: 'custom_admin=example' }), options({ sessionCookieName: 'custom_admin', fetcher: async (_url, init) => {
    assert.equal(init.headers.get('Cookie'), 'custom_admin=example');
    return Response.json({ data: { platformOperator: true } });
  } }));
  assert.equal(custom.status, 200);
  const merchant = await serveDocumentation(request({ Cookie: 'emisell_merchant_session=example' }), options({ fetcher: () => assert.fail('No developer credential') }));
  assert.equal(merchant.status, 401);
});
