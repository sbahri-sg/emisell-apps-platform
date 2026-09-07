import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import {
  documents,
  example,
  filterOperations,
  groups,
  operations,
  schemaType,
  endpointAddress,
  gateway,
  filterCoverage,
} from './api-docs.ts';
import { portalView, portalDocsGroup } from './surfaces.ts';

void test('documentation deep links restore every allowed group, including Core Testing', () => {
  for (const group of groups)
    assert.equal(portalDocsGroup(`?api_group=${group.id}`), group.id);
  assert.equal(portalDocsGroup('?api_group=unknown'), 'admin');
  assert.equal(
    portalDocsGroup(
      '?api_group=core&api_operation=%2Femisell.testing.v1.TestDistributionService%2FListAssignments',
    ),
    'core',
  );
});

void test('documentation covers all contract groups with unique, resolved operations', () => {
  assert.equal(new Set(operations.map((o) => o.id)).size, operations.length);
  assert.equal(documents.length, 22);
  for (const group of groups)
    assert.ok(filterOperations(group.id, '').length > 0);
  for (const o of operations) {
    assert.ok(
      documents.some((d) => d.name === o.source),
      o.source,
    );
    assert.ok(o.responses.length > 0);
    assert.doesNotMatch(JSON.stringify(o), /"\$ref"/);
    assert.equal(
      ['core', 'gateway'].includes(o.group) &&
        o.source !== 'core-reviewed-ui.v1.json',
      o.method === 'RPC',
    );
  }
  assert.equal(filterOperations('core', '').length, 21);
  const ui = operations.filter((o) => o.source === 'core-reviewed-ui.v1.json');
  assert.equal(ui.length, 2);
  assert.ok(ui.every((o) => o.group === 'core'));
  assert.match(ui.find((o) => o.method === 'GET')!.description, /loopback/);
  assert.match(
    ui.find((o) => o.method === 'POST')!.description,
    /tanpa token\/SSO/,
  );
  const testing = filterOperations('core', 'TestDistributionService');
  assert.equal(testing.length, 1);
  assert.match(testing[0].auth, /Legacy key dan token aplikasi ditolak/);
  assert.match(testing[0].description, /Bukan consent\/grant/);
  assert.match(
    testing[0].description,
    /Integration runtime umum tetap belum tersedia/,
  );
  assert.match(testing[0].description, /installable=true/);
  const engine = filterOperations('core', 'EngineGrantService');
  assert.equal(engine.length, 1);
  assert.match(engine[0].auth, /independent engine credential/);
  assert.match(engine[0].auth, /Full Core key.*DITOLAK/);
  assert.match(engine[0].description, /tanpa cache/);
  assert.match(engine[0].description, /local-isolated/);
  assert.equal(
    operations.filter((o) => o.source === 'testing.v1.json').length,
    6,
  );
  assert.equal(
    operations.filter((o) => o.source === 'integration-releases.v1.json')
      .length,
    7,
  );
  assert.ok(
    operations.some((o) => o.path === '/api/v1/admin/catalog/{id}/status'),
  );
  assert.ok(
    operations.some((o) => o.path === '/api/v1/developer/apps/{id}/tooling'),
  );
  assert.ok(
    operations.some((o) => o.path === '/api/v1/developer/access-scopes'),
  );
  assert.ok(operations.some((o) => o.path === '/api/v1/admin/access-scopes'));
  assert.ok(
    operations.every(
      (o) => o.group !== 'legacy' && !o.path.includes('/workspaces'),
    ),
  );
  assert.ok(
    documents.every(
      (d) => !['platform.v1.json', 'operations.v1.json'].includes(d.name),
    ),
  );
  assert.doesNotMatch(JSON.stringify(documents), /\/api\/v1\/workspaces/);
});

void test('sources offered for download are the exact allowlisted contracts, never local credentials', () => {
  for (const d of documents) {
    const path =
      d.format === 'OpenAPI'
        ? `../../../api/openapi/${d.name}`
        : d.format === 'Protobuf'
          ? `../../../api/proto/${d.name}`
          : d.format === 'Markdown'
            ? `../../../${d.name}`
            : d.name === 'gateway-coverage.v1.generated.json'
              ? '../../../api/gateway/coverage.v1.generated.json'
              : assert.fail(`Unknown documentation source: ${d.name}`);
    assert.equal(
      d.content,
      readFileSync(new URL(path, import.meta.url), 'utf8'),
    );
    assert.doesNotMatch(d.name, /\.local|credential|\.\./);
  }
});

void test('local installation lifecycle is separate from resource grants and Core credentials', () => {
  const lifecycle = filterOperations('core', 'InstallationService ·');
  assert.equal(lifecycle.length, 6);
  assert.ok(
    lifecycle.every((o) =>
      o.auth.includes('Legacy key dan token aplikasi ditolak'),
    ),
  );
  const activate = lifecycle.find((o) => o.path.endsWith('/Activate'))!;
  assert.ok(activate.request?.properties?.target?.properties?.merchantId);
  assert.ok(
    activate.responses[0].schema?.properties?.result?.properties?.installation,
  );
  assert.equal(filterOperations('app-access', '').length, 2);
  const app = filterOperations('app-access', 'installation-access');
  assert.equal(app.length, 1);
  assert.equal(app[0].path, '/api/v1/app/installation-access');
  assert.equal(app[0].parameters.filter((p) => p.required).length, 3);
  assert.equal(
    app[0].responses[0].schema?.properties?.resourceGatewayAllowed?.const,
    false,
  );
  assert.match(app[0].description, /bukan delegasi reusable/);
});

void test('managed release alone never grants installation; local lifecycle is separately gated', () => {
  const managed = operations.filter(
    (o) => o.source === 'managed-shipping.v1.json',
  );
  assert.equal(managed.length, 6);
  assert.ok(managed.every((o) => ['admin', 'developer'].includes(o.group)));
  const submit = managed.find(
    (o) => o.method === 'POST' && o.group === 'developer',
  )!;
  assert.equal(
    submit.request!.properties!.binding.properties!.engine.const,
    'api-kurir',
  );
  assert.equal(
    submit.request!.properties!.binding.properties!.providerCode.const,
    'emisell',
  );
  assert.equal(submit.request!.properties!.endpoint, undefined);
  assert.equal(
    submit.responses[0].schema!.properties!.readiness.properties!.installable
      .const,
    false,
  );
  assert.match(submit.description, /bukan OAuth/);
  assert.match(submit.description, /Release bukan consent\/grant/);
  assert.match(submit.description, /engine lokal terisolasi/);
  assert.match(submit.description, /assignment approved tetap diperlukan/);
});

void test('gateway handoff never advertises local live endpoints or active grants', () => {
  const planned = filterOperations('gateway', '');
  assert.equal(planned.length, 2);
  assert.equal(operations.length, 94);
  assert.equal(gateway.coverage.length, 108);
  assert.equal(gateway.live, false);
  assert.ok(
    gateway.coverage.every(
      (c) => !c.grantable && c.implementation === 'planned',
    ),
  );
  assert.equal(filterCoverage('', 'partial').length, 2);
  assert.deepEqual(
    filterCoverage('read_products', 'all').map((c) => c.scope),
    ['read_products'],
  );
  assert.equal(filterCoverage('no-such-resource', 'all').length, 0);
  for (const o of planned) {
    assert.equal(o.target, 'emisell-core');
    assert.equal(o.availability, 'planned');
    assert.match(endpointAddress(o), /^CORE_GATEWAY_BASE_URL\//);
    assert.doesNotMatch(endpointAddress(o), /localhost|127.0.0.1|8088/);
    assert.match(o.auth, /read_products.*write_products/);
    assert.ok(gateway.operations.some((g) => g.procedure === o.path));
  }
  const list = planned.find((o) => o.path.endsWith('/List'))!;
  assert.match(list.request!.properties!.pageSize.description!, /1\.\.100/);
  assert.equal(
    list.request!.properties!.access.properties!.grantRevision.format,
    'int64',
  );
  assert.ok(
    documents.some((d) => d.name === 'docs/emisell-gateway-handoff.md'),
  );
  assert.ok(documents.some((d) => d.name.endsWith('/product.proto')));
  assert.match(
    endpointAddress(
      operations.find((o) => o.path.endsWith('PaymentService/Create'))!,
    ),
    /^http:\/\/127.0.0.1:8088\//,
  );
});

void test('search stays in its group and reports empty results honestly', () => {
  assert.ok(
    filterOperations('core', '  APPS.INSTALL_INTENTS.CONSENT ').some((o) =>
      o.path.endsWith('/Decide'),
    ),
  );
  assert.equal(filterOperations('admin', 'InstallIntent').length, 0);
  assert.equal(filterOperations('store', 'missing-endpoint-xyz').length, 0);
  assert.ok(
    filterOperations('admin', 'POST').every((o) => o.method === 'POST'),
  );
});

void test('intent and capability examples use compiler-derived ProtoJSON without implicit access', () => {
  const prepare = operations.find((o) =>
    o.path.endsWith('InstallIntentService/Prepare'),
  )!;
  assert.deepEqual(Object.keys(prepare.request!.properties!), [
    'coreActorId',
    'idempotencyKey',
    'appId',
    'version',
    'merchantId',
  ]);
  assert.match(prepare.auth, /apps.install_intents.write/);
  const decide = operations.find((o) =>
    o.path.endsWith('InstallIntentService/Decide'),
  )!;
  assert.match(decide.auth, /apps.install_intents.consent/);
  assert.equal(
    example(decide.request!.properties!.decision),
    'CONSENT_DECISION_CONSENT',
  );
  const response = decide.responses[0].schema!.properties!.intent;
  assert.equal(
    (example(response) as { executionAllowed: boolean }).executionAllowed,
    false,
  );
  const pay = operations.find((o) => o.path.endsWith('PaymentService/Create'))!;
  assert.equal(pay.request!.properties!.amountMinor.format, 'int64');
  assert.equal(typeof example(pay.request!.properties!.amountMinor), 'string');
  assert.equal(
    schemaType({ type: 'array', items: { type: 'string' } }),
    'string[]',
  );
  const login = operations.find((o) => o.path === '/api/v1/admin/login')!;
  assert.deepEqual(example(login.request!), {
    email: 'admin@example.invalid',
    password: 'YOUR_PASSWORD',
  });
});

void test('navigation deep links are allowlisted and never expose admin docs in developer surface', () => {
  assert.equal(portalView('admin', '?view=api-docs'), 'api-docs');
  assert.equal(portalView('developer', '?view=api-docs'), 'apps');
  assert.equal(
    portalView('admin', '?view=store&workspace=local-store'),
    'overview',
  );
  assert.equal(portalView('admin', '?view=https://evil.invalid'), 'overview');
  assert.equal(portalView('developer', '?view=tooling'), 'tooling');
  const portal = readFileSync(
    new URL('../components/portal.tsx', import.meta.url),
    'utf8',
  );
  assert.ok(portal.indexOf('if (!session)') < portal.indexOf('<APIDocs'));
  assert.match(portal, /view === 'api-docs' && !developer/);
  const docs = readFileSync(
    new URL('../components/api-docs.tsx', import.meta.url),
    'utf8',
  );
  assert.doesNotMatch(
    docs,
    /fetch\(|api\.request|document\.cookie|localStorage|sessionStorage|dangerouslySetInnerHTML/,
  );
  assert.match(docs, /readOnly/);
  assert.match(docs, /clipboard.writeText/);
  assert.match(docs, /useScopeReadiness\(api\)/);
});
