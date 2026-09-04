import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { developerGuide } from '../lib/documentation/developer-guide.mjs';
import { serveDeveloperDocumentation } from '../lib/documentation/developer-access.mjs';
import { developerContracts, renderDeveloperDocumentation } from '../lib/documentation/developer-content.mjs';
import { buildPostman, buildReference, operations, selectSpec } from '../lib/documentation/contracts.mjs';

const read = (path) => readFileSync(new URL(`../${path}`, import.meta.url), 'utf8');
const sources = {
  platform: JSON.parse(read('docs/openapi.json')),
  provider: JSON.parse(read('docs/provider-openapi.json')),
  resource: JSON.parse(read('docs/emisell-resource-openapi.json')),
  resourceBlueprint: JSON.parse(read('docs/emisell-resource-blueprint.openapi.json')),
  shippingProvider: JSON.parse(read('docs/shipping-provider.openapi.json')),
};
const actor = { userId: 'fixture-developer', organizationId: 'fixture-org', role: 'developer', authenticationMethod: 'session', activeOrganization: { organizationId: 'fixture-org', role: 'developer' } };
const request = (headers = {}, query = '') => new Request(`https://console.example.com/docs/content${query}`, { headers });
const sourceLoaders = { loadGuide: async () => developerGuide, loadSources: async () => sources };
const options = (overrides = {}) => ({ gatewayUrl: 'https://gateway.example.com', fetcher: async () => Response.json({ data: actor }), render: () => renderDeveloperDocumentation(new URLSearchParams(), sourceLoaders), ...overrides });
const mustNotRender = () => assert.fail('Unauthenticated or invalid requests must not load documentation');

test('eight developer chapters have stable navigation and link only to Partner API operations', () => {
  const expected = ['getting-started', 'authentication-installation', 'merchant-data', 'webhooks', 'app-surfaces', 'shipping-rates', 'testing-release', 'api-reference'];
  assert.deepEqual(developerGuide.chapters.map((chapter) => chapter.id), expected);
  assert.deepEqual(developerContracts, ['provider']);
  for (const chapter of developerGuide.chapters) {
    for (const field of ['title', 'label', 'summary', 'purpose']) assert.ok(chapter[field].length > 0, `${chapter.id}: ${field}`);
    for (const field of ['prerequisites', 'sections', 'verify', 'limits']) assert.ok(chapter[field].length > 0, `${chapter.id}: ${field}`);
    const ids = chapter.sections.map((section) => section.id);
    assert.equal(new Set(ids).size, ids.length);
    assert.ok(ids.every((id) => /^[a-z][a-z-]+$/.test(id) && id !== 'verify'));
    for (const section of chapter.sections) {
      for (const link of section.links ?? []) assert.ok(['/apps', '/accept-invitation'].includes(link.href));
      for (const link of section.api ?? []) {
        assert.ok(developerContracts.includes(link.contract));
        assert.ok(operations(selectSpec(sources, link.contract)).some(({ operation }) => operation.operationId === link.operationId), `${chapter.id}: ${link.operationId}`);
      }
    }
  }
});

test('guides keep UI surfaces unsupported, data/event catalogs live, and example commands real', () => {
  const chapters = developerGuide.chapters;
  const surfaces = chapters.find((chapter) => chapter.id === 'app-surfaces');
  assert.equal(surfaces.status, 'mixed');
  assert.equal(surfaces.catalog, 'extensions');
  assert.equal(chapters.find(chapter => chapter.id === 'shipping-rates').status, 'mixed');
  for (const id of ['app-home', 'admin-extensions', 'checkout-storefront']) assert.match(surfaces.sections.find((section) => section.id === id).title, /belum didukung/);
  assert.equal(chapters.find((chapter) => chapter.id === 'merchant-data').catalog, 'scopes');
  assert.equal(chapters.find((chapter) => chapter.id === 'webhooks').catalog, 'webhooks');
  const scripts = JSON.parse(read('package.json')).scripts;
  const code = chapters.flatMap((chapter) => chapter.sections).map((section) => section.code?.text ?? '').join('\n');
  for (const [, script] of code.matchAll(/npm run ([a-z:-]+)/g)) assert.ok(scripts[script], script);
  assert.doesNotMatch(code, /npm install|@emisell\//);
  const payload = JSON.stringify(developerGuide);
  assert.doesNotMatch(payload, /BEGIN (?:RSA )?PRIVATE KEY|emisell-local-dev-token|<script>|resource-blueprint\.openapi/);
  assert.match(payload, /deduplikasi delivery saja tidak cukup/);
  assert.match(payload, /source=test/);
  assert.match(read('docs/oauth-webhooks.md'), /Do not deduplicate only by `X-Emisell-Delivery-Id`/);
});

test('developer reference publishes only Partner API and no dashboard or internal schemas', () => {
  for (const id of developerContracts) {
    const reference = buildReference(sources, id, developerContracts);
    assert.deepEqual(reference.contracts.map((entry) => entry.id).sort(), [...developerContracts].sort());
    assert.equal(reference.selected.id, id);
    assert.equal(reference.handoff, null);
    assert.ok(reference.operations.length > 0);
    for (const value of [reference, selectSpec(sources, id)]) {
      assert.doesNotMatch(JSON.stringify(value), /\/v1\/internal\/|\/internal\/app-platform\/|\/auth\/admin\/|resource-blueprint|DeveloperApplication|ExtensionRuntimeToken/);
    }
  }
  assert.throws(() => buildReference(sources, 'admin', developerContracts), /does not allow this contract/);
});

test('all developer formats are generated from their selected source with guarded Postman examples', async () => {
  const guideResponse = await renderDeveloperDocumentation(new URLSearchParams(), { ...sourceLoaders, loadSources: mustNotRender });
  assert.deepEqual(await guideResponse.json(), developerGuide);
  for (const id of developerContracts) {
    for (const format of ['reference', 'openapi', 'postman']) {
      const response = await renderDeveloperDocumentation(new URLSearchParams({ contract: id, format }), { ...sourceLoaders, loadGuide: mustNotRender });
      assert.equal(response.status, 200);
      assert.equal(response.headers.has('Content-Disposition'), format !== 'reference');
      const body = await response.json();
      const expected = operations(selectSpec(sources, id)).length;
      assert.equal(format === 'reference' ? body.operations.length : format === 'openapi' ? operations(body).length : body.item.flatMap((group) => group.item).length, expected);
      if (format === 'postman') {
        for (const entry of body.item.flatMap((group) => group.item)) {
          if (/\/v1\/products/.test(entry.request.url.raw)) assert.match(entry.event[0].script.exec.join('\n'), /enable_resource_pilot.*skipRequest/);
          if (/billing|\/plans/.test(entry.request.url.raw)) assert.match(entry.event[0].script.exec.join('\n'), /enable_app_billing.*skipRequest/);
        }
      }
    }
  }
});

test('internal, unknown and malformed contract requests cannot load any source in any format', async () => {
  for (const contract of ['admin', 'developer', 'emisell', 'identity', 'managed', 'resource', 'resource-blueprint', 'shipping-provider', '../admin', 'DEVELOPER', '']) {
    for (const format of ['guide', 'reference', 'openapi', 'postman']) {
      const response = await renderDeveloperDocumentation(new URLSearchParams({ contract, format }), { loadGuide: mustNotRender, loadSources: mustNotRender });
      assert.equal(response.status, 400, `${contract}/${format}`);
    }
  }
  const response = await renderDeveloperDocumentation(new URLSearchParams({ format: 'raw' }), { loadGuide: mustNotRender, loadSources: mustNotRender });
  assert.equal(response.status, 400);
});

test('shipping reference stays operator-only and cannot be mistaken for Partner API', async () => {
  const spec = sources.shippingProvider;
  assert.equal(spec['x-emisell-implementation'].status, 'planned');
  assert.equal(spec['x-emisell-implementation'].defaultEnabled, false);
  assert.deepEqual(spec['x-emisell-implementation'].implementedPaths, []);
  assert.equal(sources.platform.paths['/rates'], undefined);
  assert.equal(sources.provider.paths['/rates'], undefined);
  const response = await renderDeveloperDocumentation(new URLSearchParams({ contract: 'shipping-provider', format: 'postman' }), { loadGuide: mustNotRender, loadSources: mustNotRender });
  assert.equal(response.status, 400);
  const collection = buildPostman(sources, 'shipping-provider');
  assert.match(collection.info.name, /PLANNED/);
  for (const event of [collection.event, ...collection.item.flatMap(group => group.item).map(item => item.event)]) {
    assert.ok(event[0].script.exec.includes('pm.execution.skipRequest();'));
  }
  assert.match(collection.variable.find(variable => variable.key === 'baseUrl').value, /example\.invalid/);
  const { ShippingRateRequest, ShippingRateResponse, ShippingRateQuote } = spec.components.schemas;
  assert.equal(ShippingRateRequest.properties.merchantId, undefined);
  assert.equal(ShippingRateRequest.properties.dimensions, undefined);
  assert.equal(ShippingRateRequest.additionalProperties, false);
  assert.equal(ShippingRateResponse.properties.data.properties.quotes.type, 'array');
  assert.equal(ShippingRateQuote.properties.currency.const, 'IDR');
  assert.equal(ShippingRateQuote.properties.price.type, 'integer');
  assert.equal(spec.components.securitySchemes.provider_key.name, 'key');
});

test('extension registry example distinguishes category, family, scope and unsupported UI', () => {
  const catalog = sources.provider.paths['/v1/extension-catalog'].get.responses['200'].content['application/json'].example.data;
  assert.deepEqual(catalog.families.map(family => family.type), ['payment', 'shipping', 'custom']);
  assert.equal(catalog.categories.length, 6);
  const chapterIds = new Set(developerGuide.chapters.map(chapter => chapter.id));
  for (const capability of catalog.capabilities) {
    assert.ok(chapterIds.has(capability.guideChapter));
    if (capability.id === 'shipping.rates.calculate') {
      assert.equal(capability.availability, 'pilot');
      assert.equal(capability.executionEnabled, false);
      assert.deepEqual(capability.endpoints, []);
      assert.deepEqual(capability.requiredScopes, []);
    } else if (capability.type !== 'custom') {
      assert.equal(capability.availability, 'planned');
      assert.equal(capability.executionEnabled, false);
      assert.deepEqual(capability.endpoints, []);
      assert.deepEqual(capability.requiredScopes, []);
    }
    if (capability.availability !== 'available') assert.equal(capability.executionEnabled, false);
  }
  for (const surface of catalog.surfaces) if (surface.id !== 'server_only') assert.equal(surface.availability, 'planned');
  assert.doesNotMatch(read('app/dashboard.tsx'), /Available capabilities|Authorize, capture, refund, and reconcile transactions/);
});

test('anonymous, Admin/Merchant-only and forged identity requests cannot read developer guides', async () => {
  for (const headers of [
    {}, { Cookie: 'emisell_admin_session=fixture-admin' }, { Cookie: 'emisell_merchant_session=fixture-merchant' },
    { 'X-User-Id': actor.userId, 'X-Organization-Id': actor.organizationId, 'X-Role': 'owner', 'X-Platform-Operator': 'true' },
    { Authorization: 'Basic fixture' }, { Authorization: 'Bearer first,second' },
    { Cookie: 'emisell_session=first; emisell_session=second' },
  ]) {
    const response = await serveDeveloperDocumentation(request(headers), options({ fetcher: mustNotRender, render: mustNotRender }));
    assert.equal(response.status, 401);
    assert.match(response.headers.get('Cache-Control'), /private, no-store/);
  }
});

test('verified developer session forwards only its cookie and never a token or identity override', async () => {
  const response = await serveDeveloperDocumentation(request({
    Cookie: 'emisell_session=fixture-session; emisell_admin_session=fixture-admin; emisell_merchant_session=fixture-merchant; tracking=private',
    Authorization: 'Bearer fixture-token', 'X-Organization-Id': 'forged-org', 'X-Role': 'owner', 'X-Platform-Operator': 'true', 'X-Forwarded-Host': 'attacker.example.com',
  }), options({ fetcher: async (url, init) => {
    assert.equal(String(url), 'https://gateway.example.com/v1/session');
    assert.deepEqual([...init.headers.keys()].sort(), ['accept', 'cookie']);
    assert.equal(init.headers.get('Cookie'), 'emisell_session=fixture-session');
    assert.equal(init.redirect, 'manual');
    assert.equal(init.cache, 'no-store');
    assert.ok(init.signal instanceof AbortSignal);
    return Response.json({ data: actor });
  } }));
  assert.equal(response.status, 200);
  assert.equal(response.headers.get('Vary'), 'Cookie, Authorization, X-Organization-Id');
  assert.equal(response.headers.get('X-Content-Type-Options'), 'nosniff');
  assert.equal(response.headers.get('Referrer-Policy'), 'no-referrer');
  assert.doesNotMatch(await response.text(), /fixture-session|fixture-token|fixture-admin|fixture-merchant/);
});

test('bearer authentication delegates the organization binding to the gateway for every developer role', async () => {
  for (const role of ['owner', 'admin', 'developer', 'analyst']) {
    const response = await serveDeveloperDocumentation(request({ Authorization: 'Bearer fixture-token', 'X-Organization-Id': actor.organizationId, 'X-Role': 'owner' }), options({ fetcher: async (url, init) => {
      assert.equal(String(url), 'https://gateway.example.com/v1/session');
      assert.deepEqual([...init.headers.keys()].sort(), ['accept', 'authorization', 'x-organization-id']);
      assert.equal(init.headers.get('Authorization'), 'Bearer fixture-token');
      assert.equal(init.headers.get('X-Organization-Id'), actor.organizationId);
      return Response.json({ data: { ...actor, role, authenticationMethod: 'bearer' } });
    } }));
    assert.equal(response.status, 200, role);
  }
});

test('missing workspace, mismatched organization or non-developer identity fails closed', async () => {
  for (const identity of [
    {}, { platformOperator: true }, { ...actor, userId: '' }, { ...actor, role: 'merchant' }, { ...actor, authenticationMethod: 'unknown' },
    { ...actor, activeOrganization: null }, { ...actor, organizationId: '' }, { ...actor, activeOrganization: { organizationId: 'different-org' } },
  ]) {
    const response = await serveDeveloperDocumentation(request({ Cookie: 'emisell_session=fixture' }), options({ fetcher: async () => Response.json({ data: identity }), render: mustNotRender }));
    assert.equal(response.status, 403);
  }
});

test('expired sessions never fall back to bearer; identity failures cannot expose guides or downloads', async () => {
  for (const format of ['guide', 'reference', 'openapi', 'postman']) {
    for (const [fetcher, expected] of [
      [async (_url, init) => { assert.equal(init.headers.get('Authorization'), null); return new Response('', { status: 401 }); }, 401],
      [async () => new Response('', { status: 403 }), 403],
      [async () => new Response('', { status: 503 }), 502],
      [async () => new Response('', { status: 302, headers: { Location: 'https://attacker.example.com' } }), 502],
      [async () => new Response('malformed-json'), 502],
      [async () => { throw new Error('sensitive transport message'); }, 502],
    ]) {
      const response = await serveDeveloperDocumentation(request({ Cookie: 'emisell_session=expired', Authorization: 'Bearer fixture-token' }, `?format=${format}`), options({ fetcher, render: mustNotRender }));
      assert.equal(response.status, expected);
      assert.doesNotMatch(await response.text(), /sensitive transport message|fixture-token/);
    }
  }
});

test('custom developer cookie, private validation failures and safe loader failures are supported', async () => {
  const response = await serveDeveloperDocumentation(request({ Cookie: 'custom_developer=fixture' }), options({ sessionCookieName: 'custom_developer', fetcher: async (_url, init) => {
    assert.equal(init.headers.get('Cookie'), 'custom_developer=fixture');
    return Response.json({ data: actor });
  }, render: () => renderDeveloperDocumentation(new URLSearchParams({ contract: 'admin' }), { loadGuide: mustNotRender, loadSources: mustNotRender }) }));
  assert.equal(response.status, 400);
  assert.match(response.headers.get('Cache-Control'), /no-store/);
  const broken = await serveDeveloperDocumentation(request({ Cookie: 'emisell_session=fixture' }), options({ render: () => { throw new Error('private source location'); } }));
  assert.equal(broken.status, 500);
  assert.doesNotMatch(await broken.text(), /private source location/);
  const invalidOrigin = await serveDeveloperDocumentation(request({ Cookie: 'emisell_session=fixture' }), options({ gatewayUrl: 'https://user:password@attacker.example.com', fetcher: mustNotRender, render: mustNotRender }));
  assert.equal(invalidOrigin.status, 502);
});
