import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import {
  developerOverview,
  loadApplicationIdentity,
} from './developer-overview.ts';
import type { AppLifecycle } from './app-lifecycle.ts';
import type { Draft, Submission, PortalAPI } from './portal.ts';
import type { Assignment } from './testing.ts';

void test('developer login uses a bounded single column and an inset password control', () => {
  const css = readFileSync(
    new URL('../app/developer-redesign.css', import.meta.url),
    'utf8',
  );
  const login =
    css.match(/\.portal-login\.developer-redesign \{([^}]+)\}/)?.[1] ?? '';
  assert.match(login, /max-width: 480px/);
  assert.match(login, /grid-template-columns: minmax\(0, 1fr\)/);
  assert.match(login, /width: 100%/);
  const password =
    css.match(
      /\.portal-login\.developer-redesign \.login-password-field button \{([^}]+)\}/,
    )?.[1] ?? '';
  assert.match(password, /position: absolute/);
  assert.match(password, /translateY\(-50%\)/);
  assert.match(
    css,
    /\.portal-login\.developer-redesign \.login-password-field input \{\s*padding-right: 48px/,
  );
});

const draft = { id: 'app-a' } as Draft;
const review = (id: string, appId: string, createdAt: string) =>
  ({ id, appId, createdAt }) as Submission;
const assignment = (merchantId: string, status: string, appId = 'app-a') =>
  ({ merchantId, status, app: { appId } }) as Assignment;

void test('overview summarizes only the selected app and unique approved testing stores', () => {
  const data: AppLifecycle = {
    managed: [],
    integrations: [],
    catalog: [],
    submissions: [],
    assignments: [
      assignment('one', 'approved'),
      assignment('one', 'approved'),
      assignment('two', 'requested'),
      assignment('three', 'revoked'),
      assignment('four', 'approved', 'app-b'),
    ],
  };
  const submissions = [
    review('older', 'app-a', '2026-09-01'),
    review('other', 'app-b', '2026-09-09'),
    review('latest', 'app-a', '2026-09-08'),
  ];
  const result = developerOverview(draft, data, submissions);
  assert.equal(result.stores, 1);
  assert.equal(result.latest?.id, 'latest');
  assert.deepEqual(
    result.reviews.map((item) => item.id),
    ['latest', 'older'],
  );
  assert.equal(submissions[0].id, 'older');
  assert.equal('installations' in result, false);
});
void test('unavailable telemetry and unverified assignments never become zero or healthy', () => {
  const result = developerOverview(draft, null, []);
  assert.equal(result.stores, null);
  assert.equal(result.latest, null);
  const component = readFileSync(
    new URL('../components/developer-workspace.tsx', import.meta.url),
    'utf8',
  );
  for (const label of [
    'API health',
    'Webhook failure rate',
    'Removed subscriptions',
    'Function error rate',
    'Activity',
    'Installs',
    'Versions',
    'Preview app with Emisell CLI',
  ]) {
    assert.ok(component.includes(label), label);
  }
  assert.match(component, /No data/);
  assert.match(component, /navigate\('stores'\)/);
  assert.doesNotMatch(component, /summary\.stores/);
  assert.match(component, /memerlukan persetujuan izin di toko/);
  assert.doesNotMatch(
    component,
    /128 ms|0%|Product Sync.*2 instalasi|emisell\.dev/,
  );
});

void test('active app identity requires a matching credential, not review approval or a fake installation', async () => {
  const paths: string[] = [];
  const api = {
    request: async (path: string) => {
      paths.push(path);
      return { credential: { appId: 'app-a', clientId: 'eai_example' } };
    },
  } as unknown as Pick<PortalAPI, 'request'>;
  await loadApplicationIdentity(api, 'app-a');
  assert.deepEqual(paths, ['/apps/app-a/credentials']);
  for (const response of [
    {},
    { credential: null },
    { credential: { appId: 'other', clientId: 'eai_example' } },
    { credential: { appId: 'app-a', clientId: '' } },
    { credential: { appId: 'app-a', clientId: 3 } },
  ]) {
    await assert.rejects(
      loadApplicationIdentity(
        {
          request: async () => response,
        } as unknown as Pick<PortalAPI, 'request'>,
        'app-a',
      ),
    );
  }
  await assert.rejects(
    loadApplicationIdentity(
      {
        request: async () => {
          throw new Error('Unavailable');
        },
      } as unknown as Pick<PortalAPI, 'request'>,
      'app-a',
    ),
  );
});

void test('Versions uses the persisted active configuration, not draft or credential readiness', () => {
  const overview = readFileSync(
    new URL('../components/developer-workspace.tsx', import.meta.url),
    'utf8',
  );
  assert.match(overview, /if \(draft.activeVersion\)/);
  assert.match(overview, /label: 'Active'/);
  assert.match(overview, /draft.activeVersion\?\.document.version/);
  const versions = readFileSync(
    new URL('../components/developer-app-sections.tsx', import.meta.url),
    'utf8',
  );
  assert.match(versions, /app.activeVersion.document.version/);
  assert.match(versions, /app.revision !== app.activeVersion.revision/);
});
void test('developer redesign keeps the existing project, audience, editor and endpoints', () => {
  const source = readFileSync(
    new URL('../components/portal.tsx', import.meta.url),
    'utf8',
  );
  assert.match(source, /view === 'apps' && developer/);
  assert.match(source, /developer \? 'developer-redesign' : 'admin-redesign'/);
  assert.match(source, /new PortalAPI\(surface\)/);
  assert.match(source, /<DraftEditor/);
  assert.match(source, /revision: 0/);
  const css = readFileSync(
    new URL('../app/developer-redesign.css', import.meta.url),
    'utf8',
  );
  assert.match(css, /\.developer-redesign/);
  for (const layout of [
    '../app/layout.tsx',
    '../../developer/app/layout.tsx',
  ]) {
    assert.match(
      readFileSync(new URL(layout, import.meta.url), 'utf8'),
      /developer-redesign.css/,
    );
  }
});
