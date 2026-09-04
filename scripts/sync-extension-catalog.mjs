// Generate both public catalog contracts from the backend registry; no server
// calls, database access, secrets or capability activation. The specialized
// API Kurir bridge remains a separate default-off internal route.
import { execFileSync } from 'node:child_process';
import { readFileSync, writeFileSync } from 'node:fs';
const sample = JSON.parse(execFileSync('go', ['run', './cmd/export-extension-catalog'], { cwd: new URL('../services/app-gateway/', import.meta.url), encoding: 'utf8' }));
const string = { type: 'string' };
const array = (items) => ({ type: 'array', items });
const object = (properties) => ({ type: 'object', additionalProperties: false, properties, required: Object.keys(properties) });
const ref = (name) => ({ $ref: `#/components/schemas/${name}` });
const schemas = {
  AppCategoryDefinition: object({ id: { ...string, enum: sample.categories.map(x => x.id) }, name: string, description: string, examples: array(string) }),
  ExtensionFamilyDefinition: object({ type: { ...string, enum: sample.families.map(x => x.type) }, name: string, description: string, configurationSupported: { type: 'boolean', description: 'Only configuration/snapshots are supported; this is not operational readiness.' } }),
  AppCapabilityDefinition: object({ id: string, name: string, type: { ...string, enum: sample.families.map(x => x.type) }, availability: { ...string, enum: ['available', 'pilot', 'planned'] }, executionEnabled: { type: 'boolean', description: 'General runtime availability, not caller authorization. False for planned/default-off pilot. Never overrides consent, lifecycle or deployment policy.' }, invocation: { ...string, enum: ['provider_to_platform', 'platform_to_provider'] }, requiredScopes: array(string), endpoints: array(object({ method: string, path: string })), description: string, limitations: array(string), guideChapter: string }),
  AppSurfaceDefinition: object({ id: string, name: string, availability: { ...string, enum: ['available', 'planned'] }, description: string }),
  RuntimeHeaderDefinition: object({ name: string, required: { type: 'boolean' }, description: string }),
  RuntimeOperationDefinition: object({ id: string, capabilityId: string, mode: { ...string, enum: ['synchronous'] }, mutation: { type: 'boolean' }, idempotency: { ...string, enum: ['required', 'not_required'] }, timeoutMs: { type: 'integer', minimum: 1, maximum: 20000 }, maximumAttempts: { type: 'integer', const: 1 }, description: string }),
  ExtensionRuntimeContractDefinition: object({ version: { ...string, const: 'v1-draft' }, status: { ...string, const: 'draft' }, executionEnabled: { type: 'boolean', const: false, description: 'The generic external-provider dispatcher is not activated. The specialized API Kurir rate bridge is a separately gated internal pilot.' }, transport: { ...string, const: 'https_json' }, authentication: { ...string, const: 'emisell_runtime_jwt' }, tokenTtlSeconds: { type: 'integer', minimum: 1, maximum: 60 }, identityClaims: array(string), headers: array(ref('RuntimeHeaderDefinition')), operations: array(ref('RuntimeOperationDefinition')), invariants: array(string) }),
  ExtensionCatalog: object({ version: string, categories: array(ref('AppCategoryDefinition')), families: array(ref('ExtensionFamilyDefinition')), capabilities: array(ref('AppCapabilityDefinition')), surfaces: array(ref('AppSurfaceDefinition')), runtimeContract: ref('ExtensionRuntimeContractDefinition') }),
};
for (const [file, operationId] of [['openapi.json', 'listExtensionCatalog'], ['provider-openapi.json', 'listProviderExtensionCatalog']]) {
  const location = new URL(`../docs/${file}`, import.meta.url);
  const before = readFileSync(location, 'utf8');
  const spec = JSON.parse(before);
  Object.assign(spec.components.schemas, schemas);
  spec.paths['/v1/extension-catalog'] = { get: {
    tags: ['Extension catalog'], operationId, summary: 'Read app categories, extension families, capabilities, surfaces and draft runtime policy',
    description: 'Public, non-tenant metadata from the reviewed Go registry. Category is discovery metadata; family describes configuration; capability status describes contracts. The generic v1 external-provider runtime remains a disabled draft. shipping.rates.calculate is a default-off internal API Kurir pilot and is not a Partner API endpoint. Reading/selecting capabilities never grants scopes or activates execution. Existing payment/shipping/custom wire types and listing categories are unchanged. No merchant data, credentials or internal runtime URLs. Cache-Control: no-store.',
    security: [], responses: { 200: { description: 'Catalog metadata, not installation readiness or authorization.', content: { 'application/json': { schema: object({ data: ref('ExtensionCatalog') }), example: { data: sample } } } } },
  } };
  if (!spec.tags.some(x => x.name === 'Extension catalog')) spec.tags.push({ name: 'Extension catalog', description: 'Category, capability and surface discovery; no authorization side effects.' });
  const after = `${JSON.stringify(spec, null, 2)}\n`;
  if (process.argv.includes('--check')) {
    if (before !== after) { console.error(`${file}: extension catalog drift`); process.exitCode = 1; }
  } else writeFileSync(location, after);
}
