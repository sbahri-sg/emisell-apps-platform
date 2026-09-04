// Generates the read-only inspection contract; does not contact any service.
import { readFileSync, writeFileSync } from 'node:fs';
const location = new URL('../docs/openapi.json', import.meta.url);
const before = readFileSync(location, 'utf8');
const spec = JSON.parse(before);
const ref = (name) => ({ $ref: `#/components/schemas/${name}` });
const text = { type: 'string' };
const id = { type: 'string', format: 'uuid' };
const count = { type: 'integer', minimum: 0 };
const object = (properties) => ({ type: 'object', additionalProperties: false, properties, required: Object.keys(properties) });
spec.components.schemas.IntegrationCheck = object({ code: text, title: text, status: { type: 'string', enum: ['pass', 'attention', 'blocked', 'info'], description: 'pass means observed configuration/evidence only, never integration verified. Checks are diagnostic, not a new publication or authorization policy.' }, detail: text, section: { type: 'string', enum: ['home', 'settings', 'versions', 'api-access', 'extensions', 'webhooks'] } });
spec.components.schemas.IntegrationReadiness = object({
  appId: id, organizationId: id, appName: text, appRevision: { type: 'integer', minimum: 1 },
  activeVersionId: { type: ['string', 'null'], format: 'uuid' }, activeVersion: { type: ['string', 'null'] },
  checkedAt: { type: 'string', format: 'date-time', description: 'Time of this non-atomic read-only inspection, not an integration-test completion timestamp.' },
  checks: { type: 'array', items: ref('IntegrationCheck') },
  scopes: { type: 'array', items: object({ scope: text, access: { type: 'string', enum: ['required', 'optional'] }, availability: { type: 'string', enum: ['available', 'planned'] }, endpoints: { type: 'array', items: object({ method: text, path: text }) } }) },
  installations: object({ sampled: { ...count, maximum: 100 }, hasMore: { type: 'boolean', description: 'If true, counts only cover the inspected page, not all installations.' }, activeCurrentVersion: count, activeOtherVersion: count, inactive: count }),
  listingStatus: { type: 'string', enum: ['draft', 'published', 'hidden'], description: 'draft also represents an absent listing; listingRevision=0 distinguishes absence.' },
  listingRevision: count, endToEndVerified: { type: 'boolean', const: false, description: 'This endpoint never runs or records an end-to-end test. Successful install does not prove resource/runtime/uninstall behavior.' },
});
const description = 'Read-only inspection from persisted app/version configuration, official scope catalog, organization entitlement, credential metadata and the first 100 installations. No external HTTP calls, credentials, raw URLs, extension configuration, merchant identity, webhook payload or secret is returned. No query parameters accepted. Cache-Control: private, no-store. This is not a readiness score, production approval, atomic snapshot or end-to-end test; errors must not be converted into a passing result. App revision/version changes during inspection return 409. ';
for (const [path, operationId, access] of [
  ['/v1/apps/{appId}/integration-readiness', 'getAppIntegrationReadiness', 'app.read within the authenticated active organization; organization cannot be selected via query.'],
  ['/v1/internal/organizations/{organizationId}/apps/{appId}/integration-readiness', 'getInternalIntegrationReadiness', 'Emisell platform_operator required independently of organization membership/role. The path pair must refer to the same app owner.'],
]) {
  spec.paths[path] = { get: {
    tags: ['App integration'], operationId, summary: 'Inspect app configuration and integration evidence', description: (path.startsWith('/v1/internal/') ? 'Admin Console only. Requires the dedicated admin session; developer JWT/bearer, organization role, and forged platform_operator headers are not accepted. The platform organization comes from the server-side admin account. ' : '') + description + access,
    security: path.startsWith('/v1/internal/') ? [{ adminSessionCookie: [] }] : [{ sessionCookie: [] }, { bearerAuth: [] }],
    parameters: [...path.matchAll(/\{([^}]+)\}/g)].map((match) => ({ name: match[1], in: 'path', required: true, schema: id })),
    responses: {
      200: { description: 'Observed configuration; not proof of successful integration.', content: { 'application/json': { schema: object({ data: ref('IntegrationReadiness') }) } } },
      ...Object.fromEntries(Object.entries({401:'Unauthorized',403:'Forbidden',404:'NotFound',409:'Conflict',422:'ValidationError'}).map(([status, name]) => [status, { $ref: `#/components/responses/${name}` }])),
      500: { description: 'An inspection dependency failed. No result is returned; refresh after resolving the failure.' },
    },
  } };
}
if (!spec.tags.some((tag) => tag.name === 'App integration')) spec.tags.push({ name: 'App integration', description: 'Admin and Developer configuration handoff. Read-only; not provider resource access.' });
const update = spec.components.schemas.UpdateCatalogListingRequest;
update.properties.expectedAppRevision = { type: 'integer', minimum: 1, description: 'Optional for legacy callers. Supply with expectedActiveVersionId from the latest inspection to reject stale publication decisions (409).' };
update.properties.expectedActiveVersionId = { ...id, description: 'Provide together with expectedAppRevision. Must still be the active version when publication commits.' };
update.dependentRequired = { expectedAppRevision: ['expectedActiveVersionId'], expectedActiveVersionId: ['expectedAppRevision'] };
spec.paths['/v1/internal/organizations/{organizationId}/apps/{appId}/catalog-listing'].put.description = 'Admin Console only. Requires the dedicated admin session; developer JWT/bearer, organization role, and forged platform_operator headers are not accepted. The platform organization comes from the server-side admin account. Operator-only publication fails unless the app, organization, HTTPS launch URL, available scopes and immutable active version are eligible. Listing revision is checked. Admin UI additionally sends expectedAppRevision and expectedActiveVersionId from inspection; supply both or neither. Stale inspection returns 409. All callers bind service validation to the locked app/version in the transaction. Existing published catalogs still follow the active version; this is not a new version-pinning or production-approval workflow. Changes are audit logged.';
const after = `${JSON.stringify(spec, null, 2)}\n`;
if (process.argv.includes('--check')) {
  if (after !== before) { console.error('openapi.json: integration inspection contract is out of sync'); process.exitCode = 1; }
} else writeFileSync(location, after);
