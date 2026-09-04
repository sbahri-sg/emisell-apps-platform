// Reviewed wire-shape reference, not a live App Platform dispatch endpoint.
// Based on api-kurir/internal/providers/hosted/rates.go and the v1.0.10
// hosted connector OpenAPI. Never copy service keys or merchant data.
import { readFileSync, writeFileSync } from 'node:fs';
const string = { type: 'string' };
const object = (properties, required = Object.keys(properties)) => ({ type: 'object', additionalProperties: false, properties, required });
const ref = name => ({ $ref: `#/components/schemas/${name}` });
const groups = ['regular', 'next_day', 'economy', 'cargo'];
const request = { origin: { district_id: 'fixture-origin' }, destination: { district_id: 'fixture-destination' }, weight_grams: 1000, courier_codes: ['fixture'], service_groups: ['regular'], price: 'lowest' };
const response = { data: { quotes: [{ provider_code: 'fixture_provider', courier_code: 'fixture', courier_name: 'Fixture Courier', service_code: 'FIXTURE_REG', service_name: 'Synthetic rate — not bookable', service_group: 'regular', price: 10000, currency: 'IDR', etd: '2-3' }] }, meta: { provider_code: 'fixture_provider', upstream: 'synthetic_fixture', product: 'shipping_cost', fixture: true } };
const spec = {
  openapi: '3.1.0', info: { title: 'Emisell Shipping Provider — Reference Contract', version: '2026-09-03', description: 'A developer-facing, narrowed reference profile for the existing API Kurir hosted rate request/response. App Platform dispatch is planned. The runnable Go example serves synthetic fixtures only and is not an approved connector package or live courier engine.' },
  servers: [{ url: 'https://shipping-provider.example.invalid' }],
  'x-emisell-implementation': { status: 'planned', defaultEnabled: false, implementedPaths: [], targetPaths: ['/rates'], verification: { kind: 'local_fixture_http_tests_only', deploymentVerified: false, externalProviderVerified: false } },
  'x-emisell-reference': { reviewedOn: '2026-09-03', consumer: 'api-kurir/internal/providers/hosted/rates.go', transport: 'api-kurir/internal/providers/hosted/client.go', upstreamSchema: 'api-kurir/artifacts/rajaongkir-hosted-v1.0.10/openapi.yaml', note: 'The generic partner starter response data array differs from the actual hosted consumer data.quotes. This profile follows the running consumer. No automatic credential/activation migration.' },
  tags: [{ name: 'Shipping rate reference', description: 'Provider-owned endpoint; not a route on App Gateway or Emisell Backend.' }],
  security: [{ provider_key: [] }],
  paths: { '/rates': { post: {
    tags: ['Shipping rate reference'], operationId: 'quoteShippingProviderRates', summary: 'Reference: return shipping rates from a provider-owned endpoint',
    description: 'App Platform invocation is NOT implemented. Wire shape matches the hosted API Kurir rate consumer, not its merchant calculate endpoint. Hosted origin/destination IDs are provider-mapped district IDs. Weight is integer grams and prices are integer IDR in this narrowed profile; no automatic scaling. API Kurir currently ignores the currency field during hosted-rate ingestion, so non-IDR must not be supplied. This is an estimate, not a fulfillment quote or booking guarantee. The local example rejects live or missing execution mode and any non-fixture route; these are example safety restrictions, not a claim that API Kurir defaults to sandbox. Authentication uses a privately provisioned provider key, NOT a developer/OAuth/merchant token. No tenant selectors in the body.',
    'x-emisell-implementation-status': 'planned',
    parameters: [{ name: 'X-Emisell-Execution-Mode', in: 'header', required: true, description: 'Required sandbox in the local example. Existing API Kurir live/sandbox policy is separate and must be approved before live integration.', schema: { type: 'string', enum: ['sandbox'] } }],
    requestBody: { required: true, content: { 'application/json': { schema: ref('ShippingRateRequest'), example: request } } },
    responses: {
      200: { description: 'Synthetic example / rate result. No installation or checkout readiness is implied.', content: { 'application/json': { schema: ref('ShippingRateResponse'), example: response } } },
      ...Object.fromEntries(Object.entries({ 400: 'Malformed JSON or unknown field', 401: 'Missing/invalid provider key', 403: 'Live/missing execution mode rejected by local fixture', 404: 'Wrong provider endpoint', 405: 'Unsupported method', 413: 'Body exceeds 64 KiB', 415: 'Expected application/json', 422: 'Invalid request or unsupported fixture route', 429: 'Provider quota/rate limit', 502: 'Provider unavailable' }).map(([status, description]) => [status, { description, content: { 'application/json': { schema: ref('ShippingRateError') } } }])),
    },
  } } },
  components: {
    securitySchemes: { provider_key: { type: 'apiKey', in: 'header', name: 'key', description: 'Provider credential supplied only by an approved runtime. Local fixture uses a separately generated example-only key. Never expose service keys to a browser. rates:read in API Kurir is not an App Platform OAuth scope.' } },
    schemas: {
      ShippingDistrict: object({ district_id: { ...string, minLength: 1, maxLength: 128, description: 'Provider-mapped district reference, not merchant ID or a general Emisell location ID.' } }),
      ShippingRateRequest: object({ origin: ref('ShippingDistrict'), destination: ref('ShippingDistrict'), weight_grams: { type: 'integer', minimum: 1, maximum: 1000000 }, courier_codes: { type: 'array', maxItems: 20, items: string }, service_groups: { type: 'array', maxItems: 4, items: { ...string, enum: groups } }, price: { ...string, enum: ['lowest', 'highest'], default: 'lowest' } }, ['origin', 'destination', 'weight_grams']),
      ShippingRateQuote: object({ provider_code: string, courier_code: string, courier_name: string, service_code: string, service_name: string, service_group: { ...string, enum: groups }, price: { type: 'integer', minimum: 0, description: 'Integer IDR amount, not a recalculated/kg tariff; no decimal or currency conversion in App Platform.' }, currency: { type: 'string', const: 'IDR' }, etd: { ...string, description: 'Provider estimate, not guaranteed calendar dates.' } }, ['provider_code', 'courier_code', 'service_code', 'service_group', 'price', 'currency']),
      ShippingRateResponse: object({ data: object({ quotes: { type: 'array', items: ref('ShippingRateQuote') } }), meta: object({ provider_code: string, upstream: string, product: { ...string, const: 'shipping_cost' }, fixture: { type: 'boolean', description: 'True only for local synthetic responses.' } }, ['provider_code', 'upstream', 'product']) }),
      ShippingRateError: object({ error: object({ code: string, message: string }) }),
    },
  },
};
const location = new URL('../docs/shipping-provider.openapi.json', import.meta.url);
const output = JSON.stringify(spec, null, 2) + '\n';
if (process.argv.includes('--check')) {
  if (readFileSync(location, 'utf8') !== output) { console.error('shipping-provider.openapi.json: reference contract drift'); process.exitCode = 1; }
} else writeFileSync(location, output);
