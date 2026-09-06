import assert from 'node:assert/strict';
import { test } from 'node:test';
import { readFileSync } from 'node:fs';
import {
  appLifecycleStatus,
  loadAppLifecycle,
  type AppLifecycle,
} from './app-lifecycle.ts';
import type { Draft, PortalAPI, Submission } from './portal.ts';
import type { Assignment } from './testing.ts';
import type { IntegrationRelease } from './integration-releases.ts';

const draft = {
  id: 'app-kurir',
  organizationId: 'dev-demo',
  revision: 2,
  updatedAt: '',
  document: {
    name: 'Kurir',
    summary: '',
    description: '',
    version: '1.0.0',
    capability: 'shipping/v1',
    scopes: ['shipping.read'],
    endpoint: '',
  },
} satisfies Draft;
const release = {
  id: 'release-a',
  status: 'signed' as const,
  manifest: {
    appId: draft.id,
    name: 'Kurir',
    version: '1.0.0',
    capability: 'shipping/v1',
    binding: { engine: 'api-kurir' as const, providerCode: 'emisell' as const },
  },
};
const assignment = {
  id: 'assignment-a',
  organizationId: 'dev-demo',
  releaseSha256: 'a'.repeat(64),
  merchantId: 'merchant-a',
  revision: 2,
  createdAt: '2026-09-06T00:00:00Z',
  updatedAt: '2026-09-06T00:00:00Z',
  releaseId: release.id,
  releaseKind: 'managed_shipping',
  status: 'approved',
  app: {
    assignmentId: 'assignment-a',
    appName: 'Kurir',
    capability: 'shipping/v1',
    appId: draft.id,
    version: '1.0.0',
    readiness: {
      configurationReady: true,
      requiredScopesReady: true,
      installable: true,
      blockers: [],
    },
  },
} as Assignment;
function data(): AppLifecycle {
  return {
    managed: [structuredClone(release)],
    integrations: [],
    catalog: [],
    submissions: [],
    assignments: [structuredClone(assignment)],
  };
}
function label(value: AppLifecycle) {
  return appLifecycleStatus(draft, value).label;
}

void test('managed signed release with ready approved assignment displays Testing, not Draft', () => {
  assert.deepEqual(appLifecycleStatus(draft, data()), {
    label: 'Testing',
    version: '1.0.0',
    tone: 'approved',
  });
  const raja = { ...draft, id: 'raja', revision: 1 };
  assert.equal(appLifecycleStatus(raja, data()).label, 'Draft');
});
void test('app/version/source identity must match; readiness alone does not prove testing', () => {
  for (const change of [
    (d: AppLifecycle) => {
      d.assignments[0].releaseId = 'other';
    },
    (d: AppLifecycle) => {
      delete d.assignments[0].releaseKind;
    },
    (d: AppLifecycle) => {
      d.managed[0].manifest.version = '2.0.0';
    },
    (d: AppLifecycle) => {
      d.managed[0].manifest.appId = 'other';
    },
    (d: AppLifecycle) => {
      d.assignments[0].app.version = '2.0.0';
    },
    (d: AppLifecycle) => {
      d.assignments[0].app.appId = 'other';
    },
  ]) {
    const value = data();
    change(value);
    assert.notEqual(label(value), 'Testing');
  }
});
void test('suspension, revoked/rejected/requested assignment never remain Testing', () => {
  const suspended = data();
  suspended.managed[0].status = 'suspended';
  assert.equal(label(suspended), 'Rilis ditangguhkan');
  for (const [status, expected] of [
    ['revoked', 'Testing dicabut'],
    ['rejected', 'Testing ditolak'],
    ['requested', 'Menunggu izin testing'],
  ] as const) {
    const value = data();
    value.assignments[0].status = status;
    assert.equal(label(value), expected);
  }
});
void test('approval without runtime/scope/configuration readiness is explicitly limited testing', () => {
  for (const key of [
    'configurationReady',
    'requiredScopesReady',
    'installable',
  ] as const) {
    const value = data();
    value.assignments[0].app.readiness[key] = false;
    assert.equal(label(value), 'Testing belum siap');
  }
  const value = data();
  value.assignments[0].app.readiness.blockers.push('release_not_ready');
  assert.equal(label(value), 'Testing belum siap');
});
void test('one revoked merchant does not hide another approved and ready assignment', () => {
  const value = data();
  value.assignments.unshift({ ...assignment, status: 'revoked' });
  assert.equal(label(value), 'Testing');
});
void test('publication is distinct from testing and cannot be inherited by a new draft version', () => {
  const value = data();
  value.catalog.push({
    status: 'published',
    package: { manifest: { appId: draft.id, version: '1.0.0' } },
  });
  assert.equal(label(value), 'Dipublikasikan');
  assert.equal(
    appLifecycleStatus(
      { ...draft, document: { ...draft.document, version: '2.0.0' } },
      value,
    ).label,
    'Draft',
  );
});
void test('signed release and metadata approval do not imply public publication or install readiness', () => {
  const value = data();
  value.assignments = [];
  assert.equal(label(value), 'Rilis ditandatangani');
  value.managed = [];
  value.submissions = [
    {
      appId: draft.id,
      version: '1.0.0',
      draftRevision: 2,
      status: 'approved',
    } as Submission,
  ];
  assert.equal(label(value), 'Metadata disetujui');
  assert.equal(
    appLifecycleStatus({ ...draft, revision: 3 }, value).label,
    'Draft',
  );
});
void test('generic integration assignments use their own signed release, not managed binding', () => {
  const value = data();
  value.managed = [];
  value.integrations = [
    {
      id: 'release-a',
      status: 'signed',
      manifest: { metadata: { appId: draft.id, version: '1.0.0' } },
    } as IntegrationRelease,
  ];
  delete value.assignments[0].releaseKind;
  assert.equal(label(value), 'Testing');
});

function mockAPI(respond: (path: string, after: string) => unknown) {
  const calls: {
    path: string;
    method: string;
    body: unknown;
    after: string;
  }[] = [];
  const api = {
    request: async (
      path: string,
      method?: string,
      body?: unknown,
      _idempotent?: boolean,
      query?: { afterId: string },
    ) => {
      calls.push({
        path,
        method: method ?? 'GET',
        body,
        after: query?.afterId || '',
      });
      return respond(path, query?.afterId || '');
    },
  } as Pick<PortalAPI, 'request'>;
  return { api, calls };
}
function response(path: string) {
  if (path === '/submissions') return { submissions: [] };
  if (path === '/test-assignments') return { assignments: [], nextAfterId: '' };
  return { releases: path === '/managed-shipping-releases' ? [release] : [] };
}
void test('loader reads existing owner APIs only, including later testing pages', async () => {
  const { api, calls } = mockAPI((path, after) =>
    path === '/test-assignments'
      ? after
        ? { assignments: [assignment], nextAfterId: '' }
        : { assignments: [], nextAfterId: 'next' }
      : response(path),
  );
  assert.equal(label(await loadAppLifecycle(api)), 'Testing');
  assert.equal(calls.length, 6);
  assert.ok(
    calls.every(
      (c) =>
        c.method === 'GET' &&
        c.body === undefined &&
        !c.path.includes('merchant'),
    ),
  );
});
void test('missing, capped, failed and cyclic data fail instead of falling back to Draft', async () => {
  const scenarios = [
    (path: string) => (path === '/catalog' ? {} : response(path)),
    (path: string) =>
      path === '/managed-shipping-releases'
        ? { releases: Array(200).fill(release) }
        : response(path),
    (path: string) =>
      path === '/test-assignments'
        ? { assignments: [], nextAfterId: 'loop' }
        : response(path),
    () => {
      throw new Error('unavailable');
    },
  ];
  for (const scenario of scenarios)
    await assert.rejects(loadAppLifecycle(mockAPI(scenario).api));
});
void test('developer list keeps draft secondary, ignores unmounted reads and offers read-only refresh', () => {
  const component = readFileSync(
    new URL('../components/developer-apps.tsx', import.meta.url),
    'utf8',
  );
  assert.match(component, /appLifecycleStatus\(draft, state.data\)/);
  assert.match(component, /<small>Draft r\{draft.revision\}<\/small>/);
  assert.match(component, /current = false/);
  assert.match(component, /Belum terverifikasi/);
  assert.match(component, /Perbarui status/);
  assert.doesNotMatch(component, /'POST'|'PUT'|localStorage|sessionStorage/);
});
