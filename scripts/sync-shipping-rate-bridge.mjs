// Generates the implemented-but-default-off Internal Emisell Gateway contract.
// It never contacts API Kurir and never reads a service key.
import { readFileSync, writeFileSync } from 'node:fs';

const location = new URL('../docs/openapi.json', import.meta.url);
const before = readFileSync(location, 'utf8');
const spec = JSON.parse(before);
const ref = (name) => ({ $ref: `#/components/schemas/${name}` });
const string = { type: 'string' };
const object = (properties, required = Object.keys(properties)) => ({ type: 'object', additionalProperties: false, properties, required });

spec.components.schemas.ShippingRateCalculateRequest = object({
  origin: { ...string, minLength: 1, maxLength: 128, pattern: '^[A-Za-z0-9_-]+$', description: 'API Kurir district/location identifier. Merchant, courier, provider and credential selectors are deliberately absent.' },
  destination: { ...string, minLength: 1, maxLength: 128, pattern: '^[A-Za-z0-9_-]+$', description: 'API Kurir district/location identifier different from origin.' },
  weight: { type: 'integer', minimum: 1, maximum: 100000000, description: 'Final chargeable weight from Emisell Backend, in grams.' },
});
spec.components.schemas.ShippingRateMeta = object({ message: string, code: { type: 'integer', const: 200 }, status: { ...string, const: 'success' } });
spec.components.schemas.ShippingRateOption = object({
  name: string, code: string, logo: { ...string, format: 'uri' }, service: string,
  canonicalService: string, serviceGroup: string, serviceType: string,
  description: string, cost: { type: 'integer', minimum: 0 }, etd: string,
});
spec.components.schemas.ShippingRateCalculateResponse = object({ meta: ref('ShippingRateMeta'), data: { type: 'array', maxItems: 200, items: ref('ShippingRateOption') } });

spec.paths['/v1/integrations/emisell/shipping/rates/calculate'] = { post: {
  tags: ['Emisell Integration'], operationId: 'calculateEmisellShippingRates', summary: 'Calculate merchant shipping rates through API Kurir',
  description: 'Internal Emisell Backend → App Platform bridge, disabled by default. Merchant and environment come only from the verified short-lived Emisell Backend token; the body cannot select a merchant, courier, app, installation, extension, credential, provider or runtime URL. App Platform requires exactly one active installed shipping extension whose immutable version declares shipping.rates.calculate. API Kurir owns service/provider selection, rate card and exact snapshot/cache lookup, request coalescing, quota ledger and optional provider quote. One request does not imply one RajaOngkir hit. No automatic retry.',
  security: [{ emisellBackendBearer: [] }], 'x-emisell-shipping-rate-bridge': true,
  requestBody: { required: true, content: { 'application/json': { schema: ref('ShippingRateCalculateRequest'), example: { origin: '442', destination: '1354', weight: 1200 } } } },
  responses: {
    200: { description: 'Normalized API Kurir result. data may be empty; it never means a free rate.', content: { 'application/json': { schema: ref('ShippingRateCalculateResponse') } } },
    400: { description: 'Invalid signed merchant context or rate input.', content: { 'application/json': { schema: ref('ErrorEnvelope') } } },
    401: { $ref: '#/components/responses/Unauthorized' },
    403: { $ref: '#/components/responses/Forbidden' },
    409: { description: 'Shipping/extension unavailable or more than one eligible extension; no arbitrary extension is selected.', content: { 'application/json': { schema: ref('ErrorEnvelope') } } },
    422: { description: 'API Kurir has no eligible rate for this request.', content: { 'application/json': { schema: ref('ErrorEnvelope') } } },
    429: { description: 'Temporary internal rate limit; honor Retry-After.', headers: { 'Retry-After': { schema: { type: 'integer', minimum: 1, maximum: 60 } } }, content: { 'application/json': { schema: ref('ErrorEnvelope') } } },
    502: { description: 'Provider or API Kurir upstream authentication failed; secret details are redacted.', content: { 'application/json': { schema: ref('ErrorEnvelope') } } },
    503: { description: 'Bridge disabled/unavailable or provider daily quota exhausted. Do not retry automatically.', content: { 'application/json': { schema: ref('ErrorEnvelope') } } },
  },
} };

spec['x-emisell-gateway-integrations'] = [{
  id: 'shipping-rate-calculation',
  title: 'Shipping rate calculation',
  status: 'pilot_default_off',
  summary: 'Backend Emisell asks App Platform to authorize one merchant-bound shipping calculation, then App Platform forwards the normalized input to API Kurir. App Platform is not the shipping engine.',
  operationId: 'calculateEmisellShippingRates',
  sequence: [
    { actor: 'Backend Emisell', action: 'Authenticate checkout and merchant, then issue a short-lived assertion.' },
    { actor: 'App Platform Gateway', action: 'Verify identity, permission, installation lifecycle and immutable shipping capability.' },
    { actor: 'API Kurir Gateway', action: 'Resolve merchant shipping configuration, rate card, snapshot/cache and quota policy.' },
    { actor: 'Shipping provider', action: 'Called by API Kurir only when its policy requires an optional provider quote.' },
  ],
  hops: [
    {
      id: 'emisell-to-app-platform', label: 'Hop 1 · authorization gateway', from: 'Backend Emisell', to: 'App Platform Gateway',
      method: 'POST', path: '/v1/integrations/emisell/shipping/rates/calculate', contentType: 'application/json',
      authentication: 'Short-lived signed Emisell Backend assertion with shipping.rates.calculate permission.',
      identity: 'Merchant ID and environment come only from the verified assertion.',
      fields: ['origin', 'destination', 'weight'],
      headers: ['Authorization', 'Content-Type', 'X-Request-ID'],
      forbiddenSelectors: ['merchantId', 'courier', 'provider', 'credentialId', 'installationId', 'extensionId', 'runtimeUrl'],
      timeoutMs: 5000,
    },
    {
      id: 'app-platform-to-api-kurir', label: 'Hop 2 · private shipping gateway', from: 'App Platform Gateway', to: 'API Kurir Gateway',
      method: 'POST', path: '/api/v1/calculate/district/domestic-cost', contentType: 'application/x-www-form-urlencoded',
      authentication: 'Server-only API Kurir service key provisioned through the deployment secret manager.',
      identity: 'App Platform forwards the authenticated merchant and sandbox/live mode in private headers.',
      fields: ['origin', 'destination', 'weight', 'include_group=true'],
      headers: ['key', 'X-Emisell-Merchant-ID', 'X-Emisell-Execution-Mode', 'X-Request-ID'],
      forbiddenSelectors: ['courier', 'provider', 'credential_id', 'installation_id', 'extension_id', 'runtime_url'],
      timeoutMs: 5000,
    },
  ],
  ownership: [
    { concern: 'Merchant and checkout authorization', owner: 'Backend Emisell', rule: 'App Platform never accepts merchant identity from request JSON.' },
    { concern: 'App installation and capability lifecycle', owner: 'App Platform', rule: 'Exactly one active installed immutable shipping extension must declare shipping.rates.calculate.' },
    { concern: 'Courier, provider and credential selection', owner: 'API Kurir', rule: 'No caller-controlled selector is forwarded by App Platform.' },
    { concern: 'Rate card, snapshot/cache and request coalescing', owner: 'API Kurir', rule: 'A calculation does not imply one upstream provider hit.' },
    { concern: 'Provider quota and fallback', owner: 'API Kurir', rule: 'App Platform does not retry automatically or bypass quota policy.' },
    { concern: 'Order and checkout state', owner: 'Backend Emisell', rule: 'A returned rate is input to Emisell business logic, not an authoritative order transition.' },
  ],
  failurePolicy: [
    'Fail closed before API Kurir when assertion, permission, installation, version or capability is invalid.',
    'Reject ambiguous eligible shipping extensions instead of choosing one arbitrarily.',
    'Limit upstream response size, validate the normalized schema and redact private upstream messages.',
    'Do not convert timeout, quota exhaustion or provider failure into an empty successful rate list.',
    'Do not retry automatically. Preserve X-Request-ID for correlation without logging credentials.',
  ],
  sandboxChecklist: [
    'Keep API_KURIR_RATES_ENABLED=false until a synthetic sandbox merchant and one eligible extension are ready.',
    'Provision API Kurir origin and service key only in the App Gateway secret environment.',
    'Make api-service mint shipping.rates.calculate only from an authenticated merchant/checkout context.',
    'Verify rate-card/cache hit, optional provider miss, timeout, malformed response, quota, suspend and uninstall paths.',
    'Confirm no API Kurir key, provider credential, customer address or full payload appears in browser, docs, logs or traces.',
    'Enable production separately after egress, rotation, monitoring and entitlement review.',
  ],
}];

const after = `${JSON.stringify(spec, null, 2)}\n`;
if (process.argv.includes('--check')) {
  if (after !== before) { console.error('openapi.json: shipping rate bridge contract is out of sync'); process.exitCode = 1; }
} else writeFileSync(location, after);
