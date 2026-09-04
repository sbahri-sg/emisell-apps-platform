// Keeps the password login and admin-session boundary explicit in OpenAPI.
import { readFileSync, writeFileSync } from 'node:fs';
const location = new URL('../docs/openapi.json', import.meta.url);
const before = readFileSync(location, 'utf8');
const spec = JSON.parse(before);
const ref = (name) => ({ $ref: `#/components/schemas/${name}` });
const errorRef = (name) => ({ $ref: `#/components/responses/${name}` });
const uuid = { type: 'string', format: 'uuid' };
const json = (schema) => ({ 'application/json': { schema } });
const response = (schema, description) => ({ description, content: json({ type: 'object', additionalProperties: false, required: ['data'], properties: { data: schema } }) });
spec.info.version = '0.22.0';
spec.info.description = 'Emisell App Platform API with a separately authenticated Admin Console, manually provisioned admin accounts, Developer Console, merchant surfaces, managed extension credentials, and integration contracts.';
spec.tags = (spec.tags ?? []).filter((tag) => tag.name !== 'Admin identity');
spec.tags.unshift({ name: 'Admin identity', description: 'Password login and a dedicated server-side session for manually provisioned Emisell operators. No public signup and no developer-token fallback.' });
spec.components.securitySchemes.adminSessionCookie = { type: 'apiKey', in: 'cookie', name: 'emisell_admin_session', description: 'Opaque HttpOnly SameSite=Strict Admin Console session. Accepted only by Admin endpoints and never by Developer or Merchant APIs.' };
spec.components.schemas.AdminLoginRequest = { type: 'object', additionalProperties: false, required: ['email', 'password'], properties: {
  email: { type: 'string', format: 'email', maxLength: 254, example: 'admin@example.com' },
  password: { type: 'string', format: 'password', writeOnly: true, maxLength: 256, example: '<admin-password>', description: 'Never put this value in documentation environments, URLs, logs, tickets, or command arguments.' },
} };
spec.components.schemas.AdminSession = { type: 'object', additionalProperties: false,
  required: ['userId', 'organizationId', 'role', 'email', 'displayName', 'platformOperator', 'authenticationMethod', 'sessionExpiresAt', 'activeOrganization'],
  properties: { userId: uuid, organizationId: uuid, role: { type: 'string', const: '' }, email: { type: 'string', format: 'email' }, displayName: { type: 'string' },
    platformOperator: { type: 'boolean', const: true }, authenticationMethod: { type: 'string', const: 'session' }, sessionExpiresAt: { type: 'string', format: 'date-time' }, activeOrganization: { type: 'object', additionalProperties: false, required: ['organizationId','name','slug','role'], properties: { organizationId: uuid, name: { type: 'string', const: '' }, slug: { type: 'string', const: '' }, role: { type: 'string', const: '' } } } },
};
spec.paths['/auth/admin/login'] = { post: { tags: ['Admin identity'], operationId: 'loginAdmin', summary: 'Create an Admin Console session',
  description: 'For accounts created manually with the server-side admin-user CLI. Requires the configured frontend Origin. Uses durable per-email and transport-IP limits. Invalid, missing, and disabled accounts return the same generic credential failure. No public signup.',
  security: [], requestBody: { required: true, content: json(ref('AdminLoginRequest')) }, responses: {
    200: response({ type: 'object', additionalProperties: false, required: ['status'], properties: { status: { type: 'string', const: 'authenticated' } } }, 'Session and CSRF cookies issued. Password and tokens are not returned.'),
    403: errorRef('Forbidden'), 422: errorRef('ValidationError'), 429: { description: 'Shared login rate limit reached. Retry-After is 900 seconds.' }, 503: { description: 'Admin login is unavailable because durable storage is not configured.' },
  }, parameters: [{ name: 'Origin', in: 'header', required: true, description: 'The exact configured Admin Console frontend origin. Browsers set this automatically.', schema: { type: 'string', format: 'uri' } }] } };
spec.paths['/auth/admin/session'] = { get: { tags: ['Admin identity'], operationId: 'getAdminSession', summary: 'Read the authenticated admin identity',
  description: 'Accepts only emisell_admin_session. Developer bearer/JWT and emisell_session are rejected.', security: [{ adminSessionCookie: [] }], responses: { 200: response(ref('AdminSession'), 'Current admin identity.'), 401: errorRef('Unauthorized') } } };
spec.paths['/auth/admin/logout'] = { post: { tags: ['Admin identity'], operationId: 'logoutAdmin', summary: 'Revoke the current Admin Console session',
  description: 'Requires the matching X-CSRF-Token from emisell_admin_csrf. Revokes the database session before expiring browser cookies.', security: [{ adminSessionCookie: [] }],
  parameters: [{ name: 'X-CSRF-Token', in: 'header', required: true, schema: { type: 'string' } }], responses: { 204: { description: 'Admin session revoked.' }, 401: errorRef('Unauthorized'), 403: errorRef('Forbidden') } } };
for (const [path, item] of Object.entries(spec.paths)) {
  if (!path.startsWith('/v1/internal/')) continue;
  for (const [method, operation] of Object.entries(item)) {
    if (!['get','post','put','patch','delete','head','options'].includes(method)) continue;
    operation.security = [{ adminSessionCookie: [] }];
    const boundary = 'Admin Console only. Requires the dedicated admin session; developer JWT/bearer, organization role, and forged platform_operator headers are not accepted. The platform organization comes from the server-side admin account. ';
    if (!operation.description?.startsWith(boundary)) operation.description = boundary + (operation.description ?? '');
    operation.parameters = (operation.parameters ?? []).filter((parameter) => parameter.name !== 'X-Organization-Id');
  }
}
const after = `${JSON.stringify(spec, null, 2)}\n`;
if (process.argv.includes('--check')) {
  if (before !== after) { console.error('openapi.json: admin login contract is out of sync'); process.exitCode = 1; }
} else writeFileSync(location, after);
