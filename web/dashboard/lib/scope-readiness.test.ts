import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import {
  loadScopeReadiness,
  gatewayOperationURL,
  scopeCatalogURL,
} from './scope-readiness.ts';
import {
  verifiedOperationState,
  verifiedScopeState,
  type ScopeVerification,
  type ScopeCatalog,
} from './access-scopes.ts';
import { gateway, operations, endpointAddress } from './api-docs.ts';

const catalog: ScopeCatalog = {
  profile: gateway.scopeProfile,
  source: 'test',
  checkedAt: '',
  grantable: false,
  scopes: ['read_products', 'write_products'].map((handle) => ({
    handle,
    resource: 'Products',
    action: handle.startsWith('read') ? 'read' : 'write',
    status: 'planned',
    grantable: false,
    implies: [],
    requiresAny: [],
    review: 'standard',
    notes: 'Synthetic',
  })),
};
const report: ScopeVerification = {
  profile: catalog.profile,
  contractRevision: gateway.contractRevision,
  environment: 'local',
  checkedAt: '2026-09-05T00:00:00Z',
  verification: 'platform_build_inventory',
  coreChecked: false,
  scopes: catalog.scopes.map((s) => ({
    handle: s.handle,
    status: 'planned',
    grantable: false,
    contractStatus: 'partial',
    operations: gateway.operations.map((o) => o.procedure),
    blockers: ['Incomplete scope coverage'],
  })),
  operations: gateway.operations.map((o) => ({
    procedure: o.procedure,
    version: o.version,
    requiredScope: o.requiredScope,
    acceptedScopes: o.acceptedScopes,
    status: 'planned',
    blockers: ['Not connected'],
  })),
};
function client(verification: unknown, fail = false) {
  const paths: string[] = [];
  return {
    paths,
    request: async <T>(path: string): Promise<T> => {
      paths.push(path);
      if (path === '/access-scopes') return structuredClone(catalog) as T;
      assert.equal(path, '/access-scopes/verification');
      if (fail) throw new Error('unavailable');
      return structuredClone(verification) as T;
    },
  };
}

void test('both views consume the same authenticated read model, not static status', async () => {
  const api = client(report);
  const scopeView = await loadScopeReadiness(api);
  const docsView = await loadScopeReadiness(api);
  assert.deepEqual(scopeView, docsView);
  assert.deepEqual(api.paths, [
    '/access-scopes',
    '/access-scopes/verification',
    '/access-scopes',
    '/access-scopes/verification',
  ]);
  assert.equal(
    verifiedScopeState(
      scopeView.catalog!,
      scopeView.verification,
      'read_products',
    ),
    'planned',
  );
  for (const o of gateway.operations)
    assert.equal(
      verifiedOperationState(
        docsView.verification,
        o.procedure,
        gateway.contractRevision,
      ),
      'planned',
    );
  const scopeComponent = readFileSync(
    new URL('../components/access-scopes.tsx', import.meta.url),
    'utf8',
  );
  const docsComponent = readFileSync(
    new URL('../components/api-docs.tsx', import.meta.url),
    'utf8',
  );
  const handoff = readFileSync(
    new URL('../components/gateway-handoff.tsx', import.meta.url),
    'utf8',
  );
  assert.match(scopeComponent, /useScopeReadiness/);
  assert.match(docsComponent, /useScopeReadiness/);
  assert.doesNotMatch(
    handoff,
    /filterCoverage|gateway\.coverage|<Table|gateway-matrix/,
  );
  assert.match(scopeComponent, /gatewayOperationURL/);
  assert.match(handoff, /scopeCatalogURL/);
});
void test('refresh reflects current evidence without elevating a partial scope', async () => {
  const ready = structuredClone(report);
  ready.coreChecked = true;
  ready.operations!.forEach((o) => {
    o.status = 'active';
    o.blockers = [];
  });
  let current = ready;
  const api = {
    request: async <T>(path: string): Promise<T> =>
      structuredClone(path === '/access-scopes' ? catalog : current) as T,
  };
  let v = await loadScopeReadiness(api);
  assert.equal(
    verifiedOperationState(v.verification, ready.operations![0].procedure),
    'active',
  );
  assert.equal(
    verifiedScopeState(v.catalog!, v.verification, 'read_products'),
    'planned',
  );
  ready.scopes[0] = {
    ...ready.scopes[0],
    status: 'active',
    grantable: true,
    contractStatus: 'complete',
    blockers: [],
  };
  const a = await loadScopeReadiness(api),
    b = await loadScopeReadiness(api);
  assert.equal(
    verifiedScopeState(a.catalog!, a.verification, 'read_products'),
    'active',
  );
  assert.equal(
    verifiedScopeState(b.catalog!, b.verification, 'read_products'),
    'active',
  );
  current = structuredClone(report);
  v = await loadScopeReadiness(api);
  assert.equal(
    verifiedScopeState(v.catalog!, v.verification, 'read_products'),
    'planned',
  );
});
void test('missing, malformed, stale-contract and duplicate evidence fail closed', async () => {
  for (const input of [
    null,
    {},
    { ...report, profile: 'wrong' },
    { ...report, environment: 'production' },
    { ...report, checkedAt: 'bad' },
    { ...report, operations: [null] },
    { ...report, scopes: [null] },
    { ...report, coreChecked: 'true' },
  ]) {
    const v = await loadScopeReadiness(client(input));
    assert.equal(v.verification, null);
    assert.ok(v.verificationError);
  }
  const failed = await loadScopeReadiness(client(report, true));
  assert.equal(failed.verification, null);
  assert.ok(failed.catalog);
  assert.equal(
    verifiedScopeState(catalog, failed.verification, 'read_products'),
    'unknown',
  );
  const mismatched = { ...report, contractRevision: 'f'.repeat(64) };
  assert.equal(
    verifiedOperationState(
      mismatched,
      gateway.operations[0].procedure,
      gateway.contractRevision,
    ),
    'unknown',
  );
  const duplicate = structuredClone(report);
  duplicate.operations!.push(duplicate.operations![0]);
  assert.equal(
    verifiedOperationState(duplicate, gateway.operations[0].procedure),
    'unknown',
  );
  const wrongMapping = structuredClone(report);
  wrongMapping.coreChecked = true;
  wrongMapping.scopes[0] = {
    ...wrongMapping.scopes[0],
    status: 'active',
    grantable: true,
    contractStatus: 'complete',
    blockers: [],
  };
  wrongMapping.operations!.forEach((o) => {
    o.status = 'active';
    o.blockers = [];
    o.acceptedScopes = ['write_products'];
  });
  assert.equal(
    verifiedScopeState(catalog, wrongMapping, 'read_products'),
    'unknown',
  );
});
void test('cross-links stay local and a status change never redirects gateway to Platform', () => {
  const procedure = gateway.operations[0].procedure;
  const url = new URL(gatewayOperationURL(procedure), 'http://localhost:4317');
  assert.equal(url.origin, 'http://localhost:4317');
  assert.equal(url.searchParams.get('api_operation'), procedure);
  assert.equal(url.searchParams.get('api_group'), 'gateway');
  for (const handle of [
    'read_products',
    'https://evil.invalid/?x=y&view=admin',
  ]) {
    const link = new URL(scopeCatalogURL(handle), 'http://localhost:4317');
    assert.equal(link.origin, 'http://localhost:4317');
    assert.equal(link.searchParams.get('view'), 'scopes');
    assert.equal(link.searchParams.get('scope'), handle);
  }
  const operation = operations.find((o) => o.path === procedure)!;
  assert.match(
    endpointAddress({ ...operation, availability: 'active' }),
    /^CORE_GATEWAY_BASE_URL\//,
  );
});
