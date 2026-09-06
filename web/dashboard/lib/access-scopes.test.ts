import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import {
  declarationError,
  filterScopes,
  selectScope,
  verifiedScopeState,
  type ScopeVerification,
  type ScopeCatalog,
  type AccessScope,
} from './access-scopes.ts';
import { portalView } from './surfaces.ts';
import { blankDocument } from './portal.ts';

// Small synthetic catalog tests behavior; production handles come only from the API.
const scope = (
  handle: string,
  implies: string[] = [],
  requiresAny: string[] = [],
): AccessScope => ({
  handle,
  resource: 'Resource',
  action: handle.startsWith('read_') ? 'read' : 'write',
  implies,
  requiresAny,
  status: 'planned',
  grantable: false,
  review: 'standard',
  notes: 'Not active',
});
const catalog: ScopeCatalog = {
  profile: 'test-profile',
  source: '',
  checkedAt: '',
  grantable: false,
  scopes: [
    scope('read_orders'),
    scope('write_orders', ['read_orders']),
    scope('read_all_orders', [], ['read_orders', 'write_orders']),
  ],
};

void test('required/optional editing is explicit and does not rewrite legacy scopes', () => {
  const original = JSON.stringify(blankDocument());
  let value = selectScope(catalog, undefined, 'write_orders', 'required');
  assert.equal(declarationError(catalog, value), '');
  value = selectScope(catalog, value, 'read_orders', 'optional');
  assert.match(declarationError(catalog, value), /tercakup required/);
  value = selectScope(catalog, value, 'read_orders', 'none');
  assert.equal(declarationError(catalog, value), '');
  assert.equal(selectScope(catalog, value, 'write_orders', 'none'), undefined);
  assert.throws(() => selectScope(catalog, value, 'orders.read', 'required'));
  assert.throws(() =>
    selectScope(
      catalog,
      { ...value!, profile: 'old' },
      'read_orders',
      'required',
    ),
  );
  assert.equal(JSON.stringify(blankDocument()), original);
  assert.doesNotMatch(original, /accessScopes/);
});
void test('dependencies and unknown profile cannot pass local validation', () => {
  assert.match(
    declarationError(catalog, {
      profile: 'test-profile',
      required: ['read_all_orders'],
      optional: ['read_orders'],
    }),
    /Dependensi required/,
  );
  assert.equal(
    declarationError(catalog, {
      profile: 'test-profile',
      required: ['write_orders', 'read_all_orders'],
      optional: [],
    }),
    '',
  );
  assert.match(
    declarationError(catalog, {
      profile: 'test-profile',
      required: ['unknown'],
      optional: [],
    }),
    /tidak dikenal/,
  );
  assert.match(
    declarationError(catalog, {
      profile: 'old',
      required: ['read_orders'],
      optional: [],
    }),
    /migration path/,
  );
});
void test('scope discovery is searchable and available only within portal navigation', () => {
  assert.equal(filterScopes(catalog, '  WRITE_ORDERS ', 'all').length, 1);
  assert.equal(filterScopes(catalog, 'no-match', 'all').length, 0);
  assert.equal(
    filterScopes(catalog, '', 'selected', {
      profile: catalog.profile,
      required: ['read_orders'],
      optional: [],
    }).length,
    1,
  );
  assert.equal(portalView('admin', '?view=scopes'), 'scopes');
  assert.equal(portalView('developer', '?view=scopes'), 'scopes');
  const source = readFileSync(
    new URL('../components/access-scopes.tsx', import.meta.url),
    'utf8',
  );
  assert.match(source, /useScopeReadiness\(api\)/);
  assert.doesNotMatch(
    source,
    /localStorage|sessionStorage|document.cookie|\/workspaces|dangerouslySetInnerHTML/,
  );
});

void test('scope table status fails closed without coherent verification evidence', () => {
  const report: ScopeVerification = {
    profile: catalog.profile,
    checkedAt: '2026-09-05T00:00:00Z',
    verification: 'platform_build_inventory',
    coreChecked: false,
    environment: 'local',
    contractRevision: 'a'.repeat(64),
    operations: [],
    scopes: catalog.scopes.map((s) => ({
      handle: s.handle,
      status: 'planned',
      grantable: false,
      contractStatus: 'missing',
      operations: [],
      blockers: ['Not implemented'],
    })),
  };
  assert.equal(
    filterScopes(catalog, '', 'planned', undefined, report).length,
    3,
  );
  assert.equal(
    filterScopes(catalog, '', 'active', undefined, report).length,
    0,
  );
  assert.equal(filterScopes(catalog, '', 'unknown').length, 3);
  assert.equal(
    verifiedScopeState(catalog, { ...report, profile: 'wrong' }, 'read_orders'),
    'unknown',
  );
  const unsafe = structuredClone(report);
  unsafe.scopes[0].status = 'active';
  unsafe.scopes[0].grantable = true;
  assert.equal(verifiedScopeState(catalog, unsafe, 'read_orders'), 'unknown');
  const complete = structuredClone(unsafe);
  complete.coreChecked = true;
  complete.scopes[0].contractStatus = 'complete';
  complete.scopes[0].operations = ['/test/Read'];
  complete.scopes[0].blockers = [];
  // Even an explicitly active scope cannot bypass a missing/planned operation.
  assert.equal(verifiedScopeState(catalog, complete, 'read_orders'), 'unknown');
  complete.operations = [
    {
      procedure: '/test/Read',
      version: 'v1',
      requiredScope: 'read_orders',
      acceptedScopes: ['read_orders'],
      status: 'active',
      blockers: [],
    },
  ];
  assert.equal(verifiedScopeState(catalog, complete, 'read_orders'), 'active');
  complete.scopes.push(complete.scopes[0]);
  assert.equal(verifiedScopeState(catalog, complete, 'read_orders'), 'unknown');
  const source = readFileSync(
    new URL('../components/access-scopes.tsx', import.meta.url),
    'utf8',
  );
  assert.match(source, /<Table /);
  assert.match(source, /<TableHead>Status/);
  assert.match(source, /useScopeReadiness/);
  assert.doesNotMatch(source, /<article className="access-row"/);
});
