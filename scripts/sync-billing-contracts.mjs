// Source of truth for the billing additions; no service or payment calls.
import { readFileSync, writeFileSync } from 'node:fs';

const ref = (name) => ({ $ref: `#/components/schemas/${name}` });
const text = (description = '') => ({ type: 'string', ...(description ? { description } : {}) });
const id = { type: 'string', format: 'uuid' };
const date = { type: 'string', format: 'date-time' };
const nullableDate = { type: ['string', 'null'], format: 'date-time' };
const money = { type: 'integer', minimum: 0, maximum: 9000000000000, description: 'Integer minor units: IDR whole rupiah; USD cents. Excludes invoice tax. No currency conversion.' };
const currency = { type: 'string', enum: ['IDR', 'USD'] };
const strings = { type: 'array', maxItems: 20, items: { type: 'string', minLength: 1, maxLength: 160 } };
const object = (properties, required = Object.keys(properties)) => ({ type: 'object', additionalProperties: false, properties, required });
const array = (name) => ({ type: 'array', items: ref(name) });
const createPlan = object({ name: { type: 'string', minLength: 2, maxLength: 80 }, description: { type: 'string', maxLength: 1000 }, amountMinor: { ...money, maximum: 1000000000000 }, currency, interval: { type: 'string', enum: ['free', 'monthly'], description: 'free requires amountMinor=0; monthly requires amountMinor>0.' }, features: strings }, ['name', 'amountMinor', 'currency', 'interval']);
const schemas = {
  CreateAppPlan: createPlan,
  AppPlan: object({ id, appId: id, appName: text('App name at plan creation.'), ...createPlan.properties, status: { type: 'string', enum: ['active', 'archived'] }, createdAt: date }),
  AppSubscriptionQuote: object({ id, installationId: id, plan: ref('AppPlan'), amountMinor: money, periodStart: date, periodEnd: { ...date, description: 'Initial paid period end; zero time 0001-01-01T00:00:00Z for a Free plan.' }, expiresAt: date, accountRevision: { type: 'integer', minimum: 0 }, termsVersion: { type: 'string', const: 'monthly-prepaid-v1' } }),
  AppSubscription: object({ id, installationId: id, quoteId: id, plan: ref('AppPlan'), status: { type: 'string', enum: ['active', 'pending_payment', 'past_due', 'cancelled'] }, approvedBy: text('Authenticated merchant user; redacted to an empty string in provider responses.'), approvedAt: date, termsVersion: text(), paidThrough: nullableDate, cancelledAt: nullableDate, cancellationReason: text() }, ['id', 'installationId', 'quoteId', 'plan', 'status', 'approvedBy', 'approvedAt', 'termsVersion', 'paidThrough', 'cancelledAt']),
  InstallationBilling: object({ plans: array('AppPlan'), subscription: { oneOf: [ref('AppSubscription'), { type: 'null' }] }, paidAccess: { type: 'boolean', description: 'Rechecked using installation/org lifecycle and paidThrough. Status active alone is insufficient. Provider must check this before granting paid features; this is not a new resource scope.' }, paidBillingAvailable: { type: 'boolean' }, test: { type: 'boolean', description: 'true means development test records. Never forward these charges to a live payment provider.' } }),
  QuoteAppSubscription: object({ planId: id }),
  ApproveAppSubscription: object({ quoteId: id, acceptRecurringCharge: { type: 'boolean', const: true, description: 'Must follow explicit merchant approval of the displayed immutable quote. No automatic subscription during app installation.' } }),
  SyncAppBillingAccount: object({ currency, cycleStart: date, cycleEnd: date, enabled: { type: 'boolean' }, revision: { type: 'integer', minimum: 0, description: '0 for creation; current server revision otherwise. Current monthly cycle only. Advance consecutively; do not pass an annual store cycle.' } }),
  AppBillingAccount: object({ merchantId: text(), environment: { type: 'string', enum: ['sandbox', 'production'] }, currency, cycleStart: date, cycleEnd: date, enabled: { type: 'boolean' }, revision: { type: 'integer', minimum: 1 } }),
  IssueAppInvoice: object({ invoiceId: text('Stable Emisell bill ID. Reuse the same ID and cycle after a timeout; do not create another bill.'), currency, cycleStart: date, cycleEnd: date }),
  AppSubscriptionCharge: object({ id, subscriptionId: id, installationId: id, appId: id, planId: id, description: text(), amountMinor: money, currency, periodStart: date, periodEnd: date, status: { type: 'string', enum: ['unbilled', 'invoiced', 'paid', 'failed', 'void'] }, invoiceId: text() }, ['id', 'subscriptionId', 'installationId', 'appId', 'planId', 'description', 'amountMinor', 'currency', 'periodStart', 'periodEnd', 'status']),
  AppBillingInvoice: object({ invoiceId: text(), merchantId: text(), environment: { type: 'string', enum: ['sandbox', 'production'] }, currency, cycleStart: date, cycleEnd: date, lines: array('AppSubscriptionCharge'), amountMinor: money, status: { type: 'string', enum: ['issued', 'paid', 'failed'] }, paymentId: text(), createdAt: date }, ['invoiceId', 'merchantId', 'environment', 'currency', 'cycleStart', 'cycleEnd', 'lines', 'amountMinor', 'status', 'createdAt']),
  AppInvoicePayment: object({ eventId: { type: 'string', pattern: '^[A-Za-z0-9._:-]{16,128}$', description: 'Stable event identity. Same identity with different body is rejected.' }, status: { type: 'string', enum: ['paid', 'failed'] }, paymentId: text('Persisted, verified payment reference in Emisell. Not supplied by the browser.'), currency, amountMinor: { ...money, description: 'The exact APP portion before invoice tax, not the combined Emisell invoice amount. Emisell must verify payment of the entire invoice before reporting paid.' } }),
};
const gate = 'Default off: APP_BILLING_ENABLED=false; live fees also require APP_BILLING_LIVE_ENABLED=true in production. Gated implementation, not an activated payment workflow. See docs/app-billing.md. ';
const backend = 'Server-to-server only. Authenticated merchant/environment are derived from the short-lived RS256 Emisell assertion with apps.billing.write permission, never from request JSON. Cookies, Origin and query parameters are rejected. ';
const merchant = 'Merchant HttpOnly session; unsafe requests require X-CSRF-Token. Merchant identity cannot be overridden in the body. ';
const path = '/v1/merchant/installations/{installationId}/billing';
const definitions = [
  ['get', '/v1/apps/{appId}/plans', 'listAppPlans', 'List immutable pricing plans', 'App plans', 'AppPlan', null, [{ sessionCookie: [] }, { bearerAuth: [] }], 'Organization-scoped app.read. Includes archived plans. No example plans are seeded.', true],
  ['post', '/v1/apps/{appId}/plans', 'createAppPlan', 'Create a Free or monthly Paid plan', 'App plans', 'AppPlan', 'CreateAppPlan', [{ sessionCookie: [] }, { bearerAuth: [] }], 'Organization-scoped app.manage (owner/admin). Price and features are immutable; new prices require new plans and new merchant consent. Idempotency-Key is body-bound.', false],
  ['delete', '/v1/apps/{appId}/plans/{planId}', 'archiveAppPlan', 'Archive a plan for new subscriptions', 'App plans', null, null, [{ sessionCookie: [] }, { bearerAuth: [] }], 'Organization-scoped app.manage. Existing approved subscriptions continue at their captured price. Idempotent archive; no hard deletion.'],
  ['get', path, 'getMerchantInstallationBilling', 'Read available plans and current subscription', 'Merchant app billing', 'InstallationBilling', null, [{ merchantSessionCookie: [] }], merchant],
  ['post', `${path}/quotes`, 'quoteAppSubscription', 'Quote the first app charge and recurring price', 'Merchant app billing', 'AppSubscriptionQuote', 'QuoteAppSubscription', [{ merchantSessionCookie: [] }], merchant + 'Quote expires after 10 minutes or cycle end. Integer-ceiling prorata for the remaining current billing period. Does not subscribe or charge.'],
  ['post', `${path}/approve`, 'approveAppSubscription', 'Approve the displayed app subscription quote', 'Merchant app billing', 'AppSubscription', 'ApproveAppSubscription', [{ merchantSessionCookie: [] }], merchant + 'Quote ID deduplicates consent and charge creation. Free activates immediately. Paid starts pending_payment; no paid access before verified payment. Late payment does not extend the quoted period. Mid-period plan switching is not supported.'],
  ['delete', `${path}/subscriptions/{subscriptionId}`, 'cancelAppSubscription', 'Cancel app subscription renewal', 'Merchant app billing', null, null, [{ merchantSessionCookie: [] }], merchant + 'Unbilled charges are voided. Issued invoices stay payable. Existing prepaid access lasts until paidThrough if installation remains active. No automatic refunds. Deactivation does not cancel billing; uninstall does.'],
  ['put', '/v1/integrations/emisell/billing/account', 'syncAppBillingAccount', 'Synchronize the merchant monthly app-billing cycle', 'Emisell app billing', 'AppBillingAccount', 'SyncAppBillingAccount', [{ emisellBackendBearer: [] }], backend + 'Current 27–32-day monthly interval only, whole seconds, same currency, consecutive revisions. Annual store plans need a separately supplied monthly app-billing cycle.'],
  ['post', '/v1/integrations/emisell/billing/invoices', 'issueAppBillingInvoice', 'Freeze app line items for an Emisell invoice', 'Emisell app billing', 'AppBillingInvoice', 'IssueAppInvoice', [{ emisellBackendBearer: [] }], backend + 'Returns only the app portion. No provider request is sent. Invoice ID/cycle/currency are immutable; retry returns the same lines. One charge per subscription/period; another invoice cannot claim the same charge. Call before creating the combined provider payment request.'],
  ['post', '/v1/integrations/emisell/billing/invoices/{invoiceId}/payment', 'recordAppInvoicePayment', 'Reconcile a verified Emisell invoice payment', 'Emisell app billing', 'AppBillingInvoice', 'AppInvoicePayment', [{ emisellBackendBearer: [] }], backend + 'Persisted Emisell payment only. Exact app amount and currency required. Stable event IDs deduplicate callbacks. Failed can become paid, paid cannot become failed. Payment of an old invoice never reactivates a cancelled subscription or uninstalled app.'],
  ['get', '/v1/installation-billing', 'getInstallationBilling', 'Read paid-feature entitlement for the current installation', 'Provider app billing', 'InstallationBilling', null, [{ installationToken: [] }], 'Installation bearer token only, lifecycle rechecked. No identity selectors, cookies or Origin. This endpoint does not grant additional resource scopes and does not itself enforce features inside an external developer app.'],
];

function operation([method, route, operationId, summary, tag, response, request, security, description, list]) {
  const parameters = [...route.matchAll(/\{([^}]+)\}/g)].map((m) => ({ name: m[1], in: 'path', required: true, schema: m[1] === 'invoiceId' ? text() : id }));
  if (operationId === 'createAppPlan') parameters.push({ name: 'Idempotency-Key', in: 'header', required: true, schema: { type: 'string', pattern: '^[A-Za-z0-9._:-]{16,128}$' } });
  if (method !== 'get' && security.some((s) => s.merchantSessionCookie)) parameters.push({ name: 'X-CSRF-Token', in: 'header', required: true, schema: text() });
  const responses = { [response ? '200' : '204']: response ? { description: summary, content: { 'application/json': { schema: object({ data: list ? array(response) : ref(response) }) } } } : { description: 'Operation completed without a response body.' } };
  for (const [status, name] of Object.entries({ 401: 'Unauthorized', 403: 'Forbidden', 404: 'NotFound', 409: 'Conflict', 422: 'ValidationError' })) responses[status] = { $ref: `#/components/responses/${name}` };
  responses['503'] = { description: 'Billing module disabled; app_billing_disabled. Do not fall back to paid access or zero app fees when billing is expected.', content: { 'application/json': { schema: object({ error: object({ code: text(), message: text(), requestId: text() }) }) } } };
  return { tags: [tag], operationId, summary, description: gate + description, security, parameters, 'x-emisell-app-billing': true, ...(request ? { requestBody: { required: true, content: { 'application/json': { schema: ref(request) } } } } : {}), responses };
}

for (const file of ['openapi.json', 'provider-openapi.json']) {
  const url = new URL(`../docs/${file}`, import.meta.url);
  const existing = readFileSync(url, 'utf8'); const spec = JSON.parse(existing);
  if (file === 'openapi.json') {
    Object.assign(spec.components.schemas, schemas);
    for (const definition of definitions) {
      const [method, path] = definition;
      (spec.paths[path] ??= {})[method] = operation(definition);
      const tag = definition[4]; if (!spec.tags.some((item) => item.name === tag)) spec.tags.push({ name: tag, description: gate });
    }
  } else {
    for (const name of ['AppPlan', 'AppSubscription', 'InstallationBilling']) spec.components.schemas[name] = schemas[name];
    const entry = operation(definitions.at(-1)); entry.security = [{ installationBearer: [] }];
    // Provider contract uses separately named error responses.
    for (const [code, response] of Object.entries(entry.responses)) if (response.$ref && !spec.components.responses[response.$ref.split('/').at(-1)]) entry.responses[code] = { description: code === '409' ? 'State conflict' : code === '422' ? 'Invalid request' : 'Request rejected' };
    spec.paths['/v1/installation-billing'] = { get: entry };
    if (!spec.tags.some((t) => t.name === 'Provider app billing')) spec.tags.push({ name: 'Provider app billing' });
  }
  const updated = `${JSON.stringify(spec, null, 2)}\n`;
  if (process.argv.includes('--check')) { if (updated !== existing) { console.error(`${file}: billing contract is out of sync`); process.exitCode = 1; } }
  else writeFileSync(url, updated);
}
