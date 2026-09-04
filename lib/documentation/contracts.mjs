// This module receives OpenAPI documents from the server. Never import the
// source documents in a client component: internal contracts are operator-only.
export const methods = new Set(['get', 'post', 'put', 'patch', 'delete', 'options', 'head']);

export const contracts = [
  { id: 'admin', title: 'Admin Control Plane', audience: 'Operator Emisell', source: 'docs/openapi.json', planned: false, visibility: 'operator',
    description: 'Intake developer, review, invitation, organisasi, publikasi katalog, dan pengelolaan managed extension connection (default-off). Memerlukan session Admin khusus; token Developer dan role organisasi tidak diterima.' },
  { id: 'developer', title: 'Developer Dashboard API', audience: 'UI Developer Console', source: 'docs/openapi.json', planned: false, visibility: 'operator',
    description: 'Mengelola app, version snapshot, extensions, scopes, credentials, webhook, dan instalasi milik organisasi. Bukan API runtime provider.' },
  { id: 'emisell', title: 'Internal Emisell Gateway', audience: 'Backend Emisell ↔ App Platform', source: 'docs/openapi.json', planned: false, visibility: 'primary',
    description: 'Kontrak internal untuk menjembatani identitas merchant, App Store, consent, Connected Apps, uninstall, billing, dan rate calculation default-off melalui API Kurir. Merchant ID selalu berasal dari identitas Emisell yang terverifikasi.' },
  { id: 'provider', title: 'Partner API', audience: 'Backend milik developer app', source: 'docs/provider-openapi.json', planned: false, visibility: 'primary',
    description: 'Kontrak publik untuk backend app: OAuth exchange, installation context, scope, webhook catalog, merchant profile, entitlement, dan resource yang memang sudah diaktifkan. Payment dan Shipping tetap memakai kontrak runtime gateway masing-masing.' },
  { id: 'managed', title: 'Managed Extension Runtime', audience: 'Runtime internal Emisell', source: 'docs/openapi.json', planned: false, visibility: 'operator',
    description: 'Credential runtime terikat ke satu instalasi–extension. Nonaktif secara default. Bukan token OAuth provider eksternal dan belum menjalankan Payment/Shipping. Provision/rotate/revoke tersedia untuk operator di Admin Control Plane; panduan docs/managed-extensions.md.' },
  { id: 'identity', title: 'Identity & Local Tooling', audience: 'Integrasi login dan pengujian internal', source: 'docs/openapi.json', planned: false, visibility: 'operator',
    description: 'Login, session organisasi, penerimaan undangan, health checks, dan simulator internal. Endpoint development bukan jalur onboarding atau instalasi production.' },
  { id: 'resource', title: 'Resource API · pilot', audience: 'App Gateway → Backend Emisell', source: 'docs/emisell-resource-openapi.json', planned: false, pilot: true, visibility: 'roadmap',
    description: 'Baca produk dasar sudah diuji lintas repo dengan PostgreSQL terisolasi. Pilot nonaktif secara default; deployment belum diverifikasi. Perlu konfigurasi kedua service, signing key, dan daftar merchant uji. Bukan endpoint langsung untuk provider.' },
  { id: 'resource-blueprint', title: 'Resource API · Backend blueprint', audience: 'Tim implementasi Backend Emisell', source: 'docs/emisell-resource-blueprint.openapi.json', planned: true, visibility: 'roadmap',
    description: 'Kontrak acuan untuk 12 endpoint baca berikutnya: variant, lokasi/inventory, order, customer, dan fulfillment. Field dipetakan ke model api-service; belum ada route berjalan. Dua endpoint produk pilot tetap terpisah. Bukan salinan API Shopify dan bukan SDK yang sudah diterbitkan.' },
  { id: 'shipping-provider', title: 'Shipping Provider · reference', audience: 'Developer shipping app', source: 'docs/shipping-provider.openapi.json', planned: true, visibility: 'roadmap',
    description: 'Kontrak contoh ongkir yang mengikuti hosted connector API Kurir. Hanya fixture lokal teruji; dispatch App Platform, approval connector, credential ownership dan aktivasi live belum tersambung. Bukan endpoint di App Gateway.' },
];

const flowByContract = {
  'shipping-provider': {
    status: 'Planned', outcome: 'Developer memahami payload ongkir dan menguji receiver memakai fixture lokal. Bukan instalasi atau ongkir live.',
    actors: ['API Kurir hosted consumer · existing', 'Provider-owned /rates', 'Synthetic example · local only'],
    quickStart: ['quoteShippingProviderRates'],
    guardrails: ['Capability ID bukan OAuth scope; belum ada execution grant App Platform.', 'Credential dan pemilihan provider saat ini masih di API Kurir; jangan membuat pemilik kedua.', 'Gunakan data sintetis; rate quote bukan booking/fulfillment quote.'],
  },
  admin: {
    status: 'Internal',
    outcome: 'Operator menyeleksi developer, mengaktifkan jalur development, memeriksa kesiapan app, lalu memutuskan publikasi katalog.',
    actors: ['Operator Emisell', 'Admin Control Plane', 'Developer ecosystem'],
    quickStart: ['loginAdmin', 'createDeveloperApplication', 'approveDeveloperApplication', 'getInternalIntegrationReadiness', 'updateCatalogListing'],
    guardrails: [
      'Gunakan session Admin khusus; credential Developer atau Merchant tidak boleh menjadi fallback.',
      'Approval developer tidak memberikan production access, dan release version tidak otomatis memublikasikan listing.',
      'Mutation yang memakai revision harus ditinjau ulang ketika server mengembalikan konflik 409.',
    ],
  },
  developer: {
    status: 'Active',
    outcome: 'Developer terundang membangun konfigurasi app, membuat snapshot version, lalu menguji instalasi pada Merchant ID yang dipilih.',
    actors: ['Developer terundang', 'App configuration', 'Merchant test'],
    quickStart: ['createApp', 'replaceAppScopes', 'createAppVersion', 'releaseAppVersion', 'createDevelopmentInstallRequest', 'getAppIntegrationReadiness'],
    guardrails: [
      'Semua app dan turunannya terisolasi oleh organisasi yang sudah diverifikasi dari session atau token.',
      'Merchant ID menentukan target pengujian, bukan bukti izin; merchant tetap login dan menyetujui consent.',
      'Client secret, webhook secret, dan credential provider disimpan App Platform dan hanya ditampilkan saat memang diperlukan.',
    ],
  },
  provider: {
    status: 'Active',
    outcome: 'Backend app menukar authorization code, menerima installation token, lalu mengakses resource sesuai effective scopes.',
    actors: ['Backend developer app', 'OAuth & installation', 'Merchant resources'],
    quickStart: ['listProviderScopeCatalog', 'exchangeProviderAuthorizationCode', 'getProviderInstallationContext', 'getProviderMerchantProfile', 'listProviderProducts'],
    guardrails: [
      'Token ditukar server-to-server melalui HTTPS; client secret dan PKCE verifier tidak boleh berada di browser.',
      'Merchant, installation, environment, dan scopes dibaca dari installation token, bukan dari header buatan caller.',
      'Products masih pilot default-off dan tidak boleh dianggap resource production umum.',
    ],
  },
  emisell: {
    status: 'Active',
    outcome: 'Backend Emisell membuka session merchant yang terverifikasi, menampilkan katalog, meminta consent, lalu mengelola connected apps dan billing yang digate.',
    actors: ['Backend Emisell', 'App Platform bridge', 'Browser merchant'],
    quickStart: ['createEmisellMerchantSessionGrant', 'exchangeEmisellMerchantSessionGrant', 'listCatalogApps', 'previewMerchantOAuthConsent', 'authorizeMerchantOAuthConsent', 'listMerchantInstallations', 'calculateEmisellShippingRates'],
    guardrails: [
      'Service JWT hanya dipakai backend-to-backend; browser merchant memakai session dan CSRF yang terpisah.',
      'One-time session grant harus pendek umur, single-use, dan tidak ditempatkan di log atau URL yang bisa bocor.',
      'App billing telah teruji sebagai modul default-off; aktivasi live memerlukan keputusan dan konfigurasi terpisah.',
      'Rate calculation default-off memakai API Kurir: rate card/snapshot/cache lebih dulu, provider quote hanya bila diperlukan. Jangan mengirim courier atau credential selector.',
    ],
  },
  identity: {
    status: 'Internal',
    outcome: 'Identitas yang terverifikasi menghasilkan session organisasi, menerima undangan, dan memulai authorization instalasi.',
    actors: ['Identity provider', 'App Platform session', 'Organization context'],
    quickStart: ['startOIDCLogin', 'completeOIDCLogin', 'getSession', 'acceptDeveloperInvitation', 'authorizeAppInstallation'],
    guardrails: [
      'Callback OIDC harus memverifikasi state, issuer, audience, expiry, dan email yang sudah diverifikasi.',
      'Invitation code hanya dapat dipakai sekali dan email identitas harus sama dengan penerima undangan.',
      'Development login dan simulator internal bukan jalur login atau instalasi production.',
    ],
  },
  resource: {
    status: 'Pilot · default off',
    outcome: 'App Gateway mengirim assertion bertanda tangan dan Backend Emisell mengembalikan proyeksi produk dasar untuk merchant uji yang diizinkan.',
    actors: ['App Gateway', 'Signed assertion', 'Backend Emisell'],
    quickStart: ['listAppPlatformProducts', 'getAppPlatformProduct'],
    guardrails: [
      'Endpoint ini hanya dipanggil App Gateway; provider tidak menerima key atau assertion untuk Backend Emisell.',
      'Signing key, audience, issuer, waktu, dan allowlist Merchant ID harus diverifikasi di kedua service.',
      'Harga dan stok masih field produk dasar; variant, inventory siap jual, orders, dan webhook resource belum termasuk.',
    ],
  },
  'resource-blueprint': {
    status: 'Planned',
    outcome: 'Tim backend mengikuti proyeksi, scope, dan aturan tenant per endpoint; App Gateway menyambungkan slice hanya setelah implementasi dan contract test lulus.',
    actors: ['Backend developer app', 'App Gateway · policy', 'api-service · resource owner'],
    quickStart: ['listAppPlatformProductVariants', 'listAppPlatformLocations', 'listAppPlatformOrders', 'listAppPlatformOrderFulfillments', 'getAppPlatformCustomer'],
    guardrails: [
      'Seluruh endpoint di blueprint ini belum diimplementasikan. OpenAPI dan Postman adalah acuan pengerjaan, bukan bukti layanan aktif.',
      'Merchant ID tetap berasal dari instalasi terverifikasi. api-service menerima assertion gateway, bukan credential developer atau token instalasi.',
      'Kerjakan baca per slice terlebih dahulu. Mutasi, tracking terstruktur, resource webhooks dan SDK terpublikasi belum termasuk.',
    ],
  },
  managed: {
    status: 'Private · default off',
    outcome: 'Runtime extension internal mengambil credential yang terikat tepat ke satu installation dan extension tanpa mengekspos secret ke browser atau Backend Emisell.',
    actors: ['Internal extension runtime', 'Scoped runtime token', 'Credential vault'],
    quickStart: ['resolveExtensionCredential'],
    guardrails: [
      'Jalur ini hanya untuk runtime internal yang dikelola Emisell; app eksternal tetap menggunakan OAuth.',
      'Token runtime terikat ke installation dan extension serta harus dicabut atau dirotasi bersama lifecycle koneksi.',
      'Kontrak ini tidak menjalankan Payment Gateway atau Shipping Gateway; keduanya tetap extension terpisah.',
    ],
  },
};

export function operations(spec) {
  return Object.entries(spec.paths).flatMap(([path, item]) =>
    Object.entries(item).filter(([method]) => methods.has(method))
      .map(([method, operation]) => ({ path, method, operation, item })),
  );
}

export function audienceFor(path, method, provider) {
  if (path.startsWith('/v1/runtime/')) return 'managed';
  if (provider.paths[path]?.[method]) return 'provider';
  if (path.startsWith('/v1/internal/') || path.startsWith('/auth/admin/')) return 'admin';
  if (path === '/v1/apps' || path.startsWith('/v1/apps/')) return 'developer';
  if (path.startsWith('/v1/catalog/') || path.startsWith('/v1/integrations/emisell/') ||
      path.startsWith('/v1/merchant/') || path === '/auth/emisell-merchant/exchange' ||
      path === '/auth/sandbox-merchant-login') return 'emisell';
  if (path === '/v1/session' || path.startsWith('/v1/session/') ||
      ['/healthz', '/readyz', '/auth/login', '/auth/callback', '/auth/development-login',
        '/v1/developer-invitations/accept', '/v1/oauth/authorizations'].includes(path)) return 'identity';
  throw new Error(`Unclassified OpenAPI operation: ${method.toUpperCase()} ${path}`);
}

export function resolve(spec, value) {
  if (!value?.$ref) return value ?? {};
  if (!value.$ref.startsWith('#/')) throw new Error('Only local OpenAPI references are supported');
  const result = value.$ref.slice(2).split('/').reduce((node, key) => node?.[key.replaceAll('~1', '/').replaceAll('~0', '~')], spec);
  if (!result) throw new Error(`Unresolved OpenAPI reference: ${value.$ref}`);
  return { ...result, ...Object.fromEntries(Object.entries(value).filter(([key]) => key !== '$ref')) };
}

export function selectSpec(sources, id) {
  const contract = contracts.find((entry) => entry.id === id);
  if (!contract) throw new Error('Unknown documentation contract');
  const source = id === 'shipping-provider' ? sources.shippingProvider : id === 'provider' ? sources.provider : id === 'resource' ? sources.resource : id === 'resource-blueprint' ? sources.resourceBlueprint : sources.platform;
  const spec = structuredClone(source);
  if (!['provider', 'resource', 'resource-blueprint', 'shipping-provider'].includes(id)) {
    spec.paths = {};
    for (const { path, method, operation, item } of operations(source)) {
      if (audienceFor(path, method, sources.provider) !== id) continue;
      spec.paths[path] ??= Object.fromEntries(Object.entries(item).filter(([key]) => !methods.has(key)));
      spec.paths[path][method] = structuredClone(operation);
    }
  }
  spec.info = { ...spec.info, title: `Emisell App Platform — ${contract.title}`, description: contract.description };
  spec['x-emisell-documentation'] = { audience: contract.audience, source: contract.source, generated: true };
  if (spec['x-emisell-implementation']?.implementedPaths) {
    spec['x-emisell-implementation'].implementedPaths = contract.planned ? [] : Object.keys(spec.paths);
  }
  // Keep only referenced components, including transitive references. Otherwise
  // a provider export could accidentally include internal admin schemas.
  const original = spec.components ?? {};
  spec.components = {};
  const used = new Set();
  const visit = (value) => {
    if (!value || typeof value !== 'object') return;
    if (value.$ref?.startsWith('#/components/') && !used.has(value.$ref)) {
      used.add(value.$ref);
      const [, , group, name] = value.$ref.split('/');
      const component = original[group]?.[name];
      if (!component) throw new Error(`Missing component ${value.$ref}`);
      (spec.components[group] ??= {})[name] = component;
      visit(component);
    }
    if (value.security) for (const alternative of value.security) for (const name of Object.keys(alternative)) {
      if (!original.securitySchemes?.[name]) throw new Error(`Unknown security scheme ${name}`);
      (spec.components.securitySchemes ??= {})[name] = original.securitySchemes[name];
    }
    Object.values(value).forEach(visit);
  };
  visit({ ...spec, components: undefined });
  const tags = new Set(operations(spec).flatMap(({ operation }) => operation.tags ?? []));
  spec.tags = (spec.tags ?? []).filter((tag) => tags.has(tag.name));
  return spec;
}

// Examples are schema illustrations, not recorded responses. Sensitive fields
// always use placeholders, including fields with writeOnly annotations.
export function schemaExample(spec, input, name = '', direction = 'response', depth = 0) {
  if (depth > 12) return null;
  const schema = resolve(spec, input);
  if (name === 'secret' && schema.type === 'object' && schema.additionalProperties?.type === 'string') return { apiKey: '<provider-api-key>' };
  if (schema.writeOnly || /secret|token|password|verifier|authorizationcode|exchangeurl|signature/i.test(name)) return `<${name || 'secret'}>`;
  if (schema.example !== undefined) return schema.example;
  if (schema.examples?.length) return schema.examples[0];
  if (schema.const !== undefined) return schema.const;
  if (schema.default !== undefined) return schema.default;
  if (schema.enum?.length) return schema.enum[0];
  const variant = schema.oneOf ?? schema.anyOf;
  if (variant?.length) return schemaExample(spec, variant.find((item) => item.type !== 'null') ?? variant[0], name, direction, depth + 1);
  if (schema.allOf) return Object.assign({}, ...schema.allOf.map((item) => schemaExample(spec, item, name, direction, depth + 1)));
  const type = Array.isArray(schema.type) ? schema.type.find((entry) => entry !== 'null') : schema.type;
  if (type === 'object' || schema.properties) return Object.fromEntries(
    Object.entries(schema.properties ?? {}).filter(([, value]) => !(direction === 'request' && resolve(spec, value).readOnly))
      .map(([key, value]) => [key, schemaExample(spec, value, key, direction, depth + 1)]),
  );
  if (type === 'array') return schema.maxItems === 0 ? [] : [schemaExample(spec, schema.items, name, direction, depth + 1)];
  if (type === 'integer' || type === 'number') return schema.minimum ?? 0;
  if (type === 'boolean') return false;
  if (type === 'null') return null;
  if (schema.format === 'uuid') return '00000000-0000-4000-8000-000000000001';
  if (schema.format === 'date-time') return '2026-01-01T00:00:00Z';
  if (schema.format === 'email') return 'developer@example.com';
  if (schema.format === 'uri' || schema.format === 'url') return 'https://app.example.com/callback';
  if (schema.format === 'hostname') return 'app.example.com';
  return `<${name || 'string'}>`;
}

function field(spec, name, input, required, location, description = '') {
  const schema = resolve(spec, input);
  return { name, location, required, type: Array.isArray(schema.type) ? schema.type.join(' | ') : schema.type ?? input?.$ref?.split('/').at(-1) ?? 'union',
    description: description || schema.description || '',
    constraints: [schema.enum && `Allowed: ${schema.enum.join(', ')}`, schema.pattern && `Pattern: ${schema.pattern}`,
      schema.minimum !== undefined && `Min: ${schema.minimum}`, schema.maximum !== undefined && `Max: ${schema.maximum}`,
      schema.minLength !== undefined && `Min length: ${schema.minLength}`, schema.maxLength !== undefined && `Max length: ${schema.maxLength}`,
      schema.format && `Format: ${schema.format}`, schema.default !== undefined && `Default: ${JSON.stringify(schema.default)}`].filter(Boolean).join(' · '),
  };
}

function authDetails(spec, operation, method) {
  const alternatives = operation.security ?? spec.security ?? [];
  if (!alternatives.length) return { labels: ['Public / no API credential (lihat prasyarat endpoint)'], headers: [], auth: { type: 'noauth' } };
  const labels = alternatives.map((alternative) => Object.entries(alternative).map(([key, scopes]) => {
    const scheme = spec.components.securitySchemes[key];
    return `${key}${scopes.length ? ` · scopes: ${scopes.join(', ')}` : ''}: ${scheme.description ?? scheme.type}`;
  }).join(' + '));
  const headers = [];
  let auth = { type: 'noauth' };
  for (const [key] of Object.entries(alternatives[0])) {
    const scheme = spec.components.securitySchemes[key];
    if (scheme.scheme === 'basic') {
      auth = { type: 'basic', basic: [{ key: 'username', value: '{{client_id}}', type: 'string' }, { key: 'password', value: '{{client_secret}}', type: 'string' }] };
    } else if (scheme.scheme === 'bearer' || scheme.type === 'oauth2') {
      const variable = /ExtensionRuntime/i.test(key) ? 'extension_runtime_token' : /installation/i.test(key) ? 'installation_token' : /emisellBackend/i.test(key) ? 'emisell_backend_jwt' : /appGateway/i.test(key) ? 'app_gateway_assertion' : 'control_plane_jwt';
      auth = { type: 'bearer', bearer: [{ key: 'token', value: `{{${variable}}}`, type: 'string' }] };
    } else if (scheme.in === 'cookie') {
      const merchant = /merchant/i.test(key);
      const scope = merchant ? 'merchant' : /admin/i.test(key) ? 'admin' : 'developer';
      headers.push({ key: 'Cookie', value: `${scheme.name}={{${scope}_session}}` });
      if (!['get', 'head', 'options'].includes(method)) headers.push({ key: 'X-CSRF-Token', value: `{{${scope}_csrf}}` });
    } else if (scheme.in === 'header') headers.push({ key: scheme.name, value: `{{${key}}}` });
    else throw new Error(`Unsupported authentication scheme: ${key}`);
  }
  return { labels, headers, auth };
}

function requestFor(spec, entry) {
  const { operation, item, path, method } = entry;
  const parameters = [...(item.parameters ?? []), ...(operation.parameters ?? [])].map((value) => resolve(spec, value));
  const body = resolve(spec, operation.requestBody);
  const content = Object.entries(body.content ?? {})[0];
  const auth = authDetails(spec, operation, method);
  const headers = [{ key: 'Accept', value: 'application/json' }, ...auth.headers];
  for (const parameter of parameters.filter((p) => p.in === 'header')) {
    if (!headers.some((header) => header.key.toLowerCase() === parameter.name.toLowerCase()))
      headers.push({ key: parameter.name, value: `{{${parameter.name === 'X-Organization-Id' ? 'organization_id' : parameter.name === 'Idempotency-Key' ? 'idempotency_key' : parameter.name}}}`, disabled: !parameter.required });
  }
  const request = { method: method.toUpperCase(), header: headers, auth: auth.auth,
    url: { raw: `{{baseUrl}}${path.replace(/\{([^}]+)\}/g, '{{$1}}')}`, host: ['{{baseUrl}}'], path: path.slice(1).split('/').map((part) => part.replace(/\{([^}]+)\}/g, '{{$1}}')),
      query: parameters.filter((p) => p.in === 'query').map((p) => ({ key: p.name, value: `{{${p.name}}}`, disabled: !p.required, description: p.description ?? '' })) },
    description: `${operation.summary}\n\n${operation.description ?? ''}\n\nAuthentication alternatives:\n${auth.labels.join('\nOR\n')}\n\nSchema examples only. Replace IDs/revisions with current data. No secrets are supplied. Do not run an entire collection against production.`,
  };
  const enabledQuery = request.url.query.filter((p) => !p.disabled);
  if (enabledQuery.length) request.url.raw += `?${enabledQuery.map((p) => `${p.key}=${p.value}`).join('&')}`;
  let requestBody = null;
  if (content) {
    const [contentType, media] = content;
    const example = media.example ?? schemaExample(spec, media.schema, '', 'request');
    headers.push({ key: 'Content-Type', value: contentType });
    requestBody = { contentType, example, schema: resolve(spec, media.schema) };
    if (contentType === 'application/x-www-form-urlencoded') request.body = { mode: 'urlencoded', urlencoded: Object.entries(example ?? {}).map(([key, value]) => ({ key, value: typeof value === 'string' ? value : JSON.stringify(value), type: 'text' })) };
    else request.body = { mode: 'raw', raw: JSON.stringify(example, null, 2), options: { raw: { language: 'json' } } };
  }
  return { request, requestBody, parameters, authentication: auth.labels };
}

function curlFor(request, planned) {
  const lines = [`curl --request ${request.method} '${request.url.raw}'`];
  if (request.auth.type === 'bearer') lines.push(`  --header 'Authorization: Bearer ${request.auth.bearer[0].value}'`);
  if (request.auth.type === 'basic') lines.push("  --user '{{client_id}}:{{client_secret}}'");
  for (const header of request.header.filter((item) => !item.disabled)) lines.push(`  --header '${header.key}: ${header.value}'`);
  if (request.body?.mode === 'raw') lines.push(`  --data-raw '${request.body.raw.replaceAll("'", "'\\''")}'`);
  if (request.body?.mode === 'urlencoded') for (const item of request.body.urlencoded) lines.push(`  --data-urlencode '${item.key}=${item.value.replaceAll("'", "'\\''")}'`);
  return `${planned ? '# PLANNED — not implemented; do not execute.\n' : ''}# Replace {{variables}} before use; mutations change real data.\n${lines.join(' \\\n')}`;
}

function routeFamiliesFor(operationList) {
  const detailed = [...new Set(operationList.map(({ path }) => {
    const parts = path.split('/').filter(Boolean);
    return `/${parts.slice(0, Math.min(2, parts.length)).join('/')}`;
  }))];
  if (detailed.length <= 4) return detailed;
  return [...new Set(operationList.map(({ path }) => {
    const parts = path.split('/').filter(Boolean);
    return parts.length > 1 ? `/${parts[0]}/*` : `/${parts[0]}`;
  }))];
}

function documentationFlow(spec, id, operationList) {
  const blueprint = flowByContract[id];
  if (!blueprint) throw new Error(`Missing documentation flow for ${id}`);
  const byId = new Map(operationList.map((operation) => [operation.id, operation]));
  const quickStart = blueprint.quickStart.map((operationId) => {
    const operation = byId.get(operationId);
    if (!operation) throw new Error(`Documentation flow ${id} references unknown operation ${operationId}`);
    return { id: operation.id, method: operation.method, path: operation.path, summary: operation.summary };
  });
  const tagDescriptions = new Map((spec.tags ?? []).map((tag) => [tag.name, tag.description ?? '']));
  const groups = [];
  for (const operation of operationList) {
    let group = groups.find((entry) => entry.title === operation.tag);
    if (!group) {
      group = { id: `group-${operation.tag.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)/g, '') || 'api'}`,
        title: operation.tag, description: tagDescriptions.get(operation.tag) ?? '', count: 0, operationIds: [] };
      groups.push(group);
    }
    group.count += 1;
    group.operationIds.push(operation.id);
  }
  return { status: blueprint.status, outcome: blueprint.outcome, actors: blueprint.actors,
    routeFamilies: routeFamiliesFor(operationList), quickStart, guardrails: blueprint.guardrails, groups };
}

/** @returns {import('./types').DocumentationReference} */
export function buildReference(sources, id, allowedIds = contracts.map((contract) => contract.id)) {
  if (!allowedIds.includes(id)) throw new Error('Documentation audience does not allow this contract');
  const summaries = contracts.filter((contract) => allowedIds.includes(contract.id)).map((contract) => {
    const spec = selectSpec(sources, contract.id);
    return { ...contract, status: contract.planned ? 'Planned' : flowByContract[contract.id].status, version: spec.info.version, count: operations(spec).length };
  });
  const selected = summaries.find((contract) => contract.id === id);
  if (!selected) throw new Error('Unknown documentation contract');
  const spec = selectSpec(sources, id);
  const operationList = operations(spec).map((entry) => {
    const { request, requestBody, parameters, authentication } = requestFor(spec, entry);
    const fields = parameters.map((p) => field(spec, p.name, p.schema, p.required === true, p.in, p.description));
    if (requestBody) for (const [name, schema] of Object.entries(requestBody.schema.properties ?? {})) {
      fields.push(field(spec, name, schema, requestBody.schema.required?.includes(name) ?? false, 'body'));
    }
    const backend = entry.operation['x-emisell-backend'];
    const success = resolve(spec, entry.operation.responses['200']);
    const envelope = resolve(spec, success.content?.['application/json']?.schema);
    const data = resolve(spec, envelope.properties?.data);
    const projection = resolve(spec, data.type === 'array' ? data.items : data);
    return { id: entry.operation.operationId, method: request.method, path: entry.path, summary: entry.operation.summary,
      backend: backend ? { ...backend, scopes: entry.operation['x-emisell-required-scopes'],
        fields: Object.entries(projection.properties ?? {}).map(([name, schema]) => ({ name, source: schema['x-emisell-source'] ?? '', description: schema.description ?? '' })) } : null,
      description: entry.operation.description ?? '', tag: entry.operation.tags?.[0] ?? 'API', authentication, fields, request: requestBody,
      responses: Object.entries(entry.operation.responses).map(([status, raw]) => {
        const response = resolve(spec, raw);
        const content = Object.values(response.content ?? {})[0];
        return { status, description: response.description ?? '', example: content ? content.example ?? schemaExample(spec, content.schema) : null, schema: content ? resolve(spec, content.schema) : null };
      }), curl: `${entry.operation['x-emisell-pilot'] ? '# PILOT: disabled by default; configure both services and approved test merchants first.\n' : ''}${curlFor(request, selected.planned)}` };
  });
  return { contracts: summaries, selected, operations: operationList, flow: documentationFlow(spec, id, operationList),
    gatewayIntegrations: id === 'emisell' ? spec['x-emisell-gateway-integrations'] ?? [] : [], handoff: spec['x-emisell-handoff'] ?? null };
}

export function buildPostman(sources, id) {
  const contract = contracts.find((entry) => entry.id === id);
  const spec = selectSpec(sources, id);
  const groups = new Map();
  for (const entry of operations(spec)) {
    const { request } = requestFor(spec, entry);
    const tag = entry.operation.tags?.[0] ?? 'API';
    if (!groups.has(tag)) groups.set(tag, []);
    groups.get(tag).push({ name: entry.operation.summary, request, response: [],
      ...(contract.planned ? { event: plannedGuard() } : entry.operation['x-emisell-pilot'] ? { event: pilotGuard() } : entry.operation['x-emisell-managed-extension'] ? { event: managedGuard() } : entry.operation['x-emisell-app-billing'] ? { event: billingGuard() } : entry.operation['x-emisell-shipping-rate-bridge'] ? { event: shippingRateGuard() } : {}) });
  }
  const item = [...groups].map(([name, items]) => ({ name, item: items }));
  const variables = [...new Set([...JSON.stringify(item).matchAll(/\{\{([^}]+)\}\}/g)].map((match) => match[1]))].sort();
  return { info: { name: `${spec.info.title}${contract.planned ? ' (PLANNED — DO NOT RUN)' : contract.pilot ? ' (PILOT — DISABLED BY DEFAULT)' : ''}`, schema: 'https://schema.getpostman.com/json/collection/v2.1.0/collection.json',
    description: `${contract.description}\nGenerated from ${contract.source} v${spec.info.version}. All credentials are empty variables. Use a local/private environment or vault, never shared initial values. Examples are illustrative; mutations must be executed individually after review.` },
    variable: variables.map((key) => ({ key, value: key === 'baseUrl' ? contract.planned || contract.pilot ? 'https://emisell-backend.example.invalid' : 'http://localhost:8081' : '', type: 'string' })),
    ...(contract.planned ? { event: plannedGuard() } : contract.pilot ? { event: pilotGuard() } : {}), item };
}

function plannedGuard() {
  return [{ listen: 'prerequest', script: { type: 'text/javascript', exec: [
    '// PLANNED: never send a request from this design-only collection.',
    'pm.execution.skipRequest();',
  ] } }];
}

function pilotGuard() {
  return [{ listen: 'prerequest', script: { type: 'text/javascript', exec: [
    '// Enable only in a private local environment after configuring both services and merchant allowlist.',
    "if (pm.environment.get('enable_resource_pilot') !== 'true') pm.execution.skipRequest();",
  ] } }];
}

function managedGuard() {
  return [{ listen: 'prerequest', script: { type: 'text/javascript', exec: [
    '// Operator review, migrated database and private runtime credentials required. Never run a collection against production.',
    "if (pm.environment.get('enable_managed_extensions') !== 'true') pm.execution.skipRequest();",
  ] } }];
}

function billingGuard() {
  return [{ listen: 'prerequest', script: { type: 'text/javascript', exec: [
    '// Billing is default-off. Review merchant consent, prices and test/live boundaries before individual requests.',
    "if (pm.environment.get('enable_app_billing') !== 'true') pm.execution.skipRequest();",
  ] } }];
}

function shippingRateGuard() {
  return [{ listen: 'prerequest', script: { type: 'text/javascript', exec: [
    '// API Kurir rate bridge is default-off. Use only a private sandbox with a synthetic merchant and configured service key.',
    "if (pm.environment.get('enable_api_kurir_rate_bridge') !== 'true') pm.execution.skipRequest();",
  ] } }];
}
