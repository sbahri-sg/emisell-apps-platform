// Generate provider product projections from the internal source contract.
// --check is read-only and fails on drift; no credentials or deployment data are generated.
import { readFileSync, writeFileSync } from 'node:fs';
const root = new URL('../', import.meta.url);
const read = (path) => JSON.parse(readFileSync(new URL(path, root), 'utf8'));
const resource = read('docs/emisell-resource-openapi.json');
const copy = (value) => JSON.parse(JSON.stringify(value).replaceAll('#/components/schemas/', '#/components/schemas/Resource').replaceAll('#/components/responses/', '#/components/responses/Resource').replaceAll('#/components/parameters/', '#/components/parameters/Resource'));
for (const [file, auth] of [['docs/openapi.json', 'installationToken'], ['docs/provider-openapi.json', 'installationBearer']]) {
  const spec = read(file);
  for (const group of ['schemas', 'responses', 'parameters']) {
    spec.components[group] ??= {};
    for (const [name, value] of Object.entries(resource.components[group])) {
      if (group === 'parameters' && name.endsWith('Header')) continue;
      spec.components[group][`Resource${name}`] = copy(value);
    }
  }
  spec.components.responses.ResourceUnauthorized.description = 'The provider installation token is missing, invalid, expired or revoked. Internal service assertion failures instead return 503 resource_unavailable.';
  spec.components.responses.ResourceForbidden.description = 'The active installation token lacks read_products or installation access is forbidden.';
  spec.components.responses.ResourceUnavailable.description = '503 resource_disabled: gateway pilot is off. 503 resource_unavailable: backend disabled, unreachable, invalid service configuration, merchant missing/inactive/suspended, timeout or invalid upstream response. No upstream error body is exposed; ask the operator to correlate requestId before retrying policy failures.';
  for (const suffix of ['', '/{productId}']) {
    const operation = copy(resource.paths[`/internal/app-platform/v1/products${suffix}`].get);
    operation.parameters = operation.parameters.filter((p) => !p.$ref.endsWith('Header'));
    operation.operationId = suffix ? 'getProviderProduct' : 'listProviderProducts';
    operation.tags = ['Products'];
    operation.security = [{ [auth]: auth === 'installationBearer' ? ['read_products'] : [] }];
    operation.description = 'Operator-enabled read_products pilot, disabled by default. Authenticate using an installation token from /oauth/token. Merchant comes exclusively from the verified installation; do not send merchantId, merchant_id, store_id or environment selectors. Unknown/duplicate query parameters are rejected. Public scope availability remains planned until rollout review; do not publish apps depending on this scope. ' + (suffix ? 'Cross-tenant products return 404.' : 'Lists base product fields only; no variants or product webhooks. updatedAfter accepts UTC Z timestamps with up to three fractional digits. Cursor is tied to installation and unchanged filters; concurrent edits are not snapshot-isolated.');
    operation.responses['200'].headers = { 'Cache-Control': { schema: { const: 'no-store' } } };
    spec.paths[`/v1/products${suffix}`] = { get: operation };
  }
  spec.tags ??= [];
  if (!spec.tags.some((t) => t.name === 'Products')) spec.tags.push({ name: 'Products', description: 'Implemented product pilot, disabled by default; not general availability' });
  if (auth === 'installationBearer') spec.components.securitySchemes.installationBearer.flows.authorizationCode.scopes.read_products = 'Base products — operator-enabled pilot only, public availability planned';
  if (spec['x-emisell-implementation']?.implementedPaths) {
    for (const path of ['/v1/products', '/v1/products/{productId}']) if (!spec['x-emisell-implementation'].implementedPaths.includes(path)) spec['x-emisell-implementation'].implementedPaths.push(path);
  }
  const expected = `${JSON.stringify(spec, null, 2)}\n`;
  if (process.argv.includes('--check')) {
    if (readFileSync(new URL(file, root), 'utf8') !== expected) throw new Error(`Product contract drift: ${file}; run npm run sync:product-contracts`);
  } else writeFileSync(new URL(file, root), expected);
}
