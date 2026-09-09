import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import {
  developerLocation,
  developerSearch,
  forApp,
} from './developer-navigation.ts';

void test('app context survives navigation, refresh and back without choosing an identity', () => {
  const overview = developerSearch('', 'apps', 'app_A');
  const credentials = developerSearch(overview, 'app-clients', 'app_A');
  const reviews = developerSearch(credentials, 'reviews', 'app_A');
  assert.deepEqual(developerLocation(overview), {
    view: 'apps',
    appId: 'app_A',
  });
  assert.deepEqual(developerLocation(credentials), {
    view: 'app-clients',
    appId: 'app_A',
  });
  assert.deepEqual(developerLocation(reviews), {
    view: 'reviews',
    appId: 'app_A',
  });
  assert.deepEqual(developerLocation(developerSearch(reviews, 'apps')), {
    view: 'apps',
    appId: '',
  });
  assert.equal(developerLocation('?view=staff&surface=admin').view, 'apps');
});

void test('organization tools clear the app filter instead of implying scoped results', () => {
  for (const view of [
    'testing',
    'ui-releases',
    'integration-releases',
    'catalog',
    'scopes',
  ]) {
    const search = developerSearch('?view=reviews&app=app_A', view, 'app_A');
    assert.equal(new URLSearchParams(search).has('app'), false);
    assert.equal(developerLocation(`?view=${view}&app=app_A`).appId, '');
  }
});

void test('unknown app context is not silently replaced with all organization data', () => {
  assert.equal(
    developerLocation('?view=app-clients&app=unknown').appId,
    'unknown',
  );
  const source = readFileSync(
    new URL('../components/portal.tsx', import.meta.url),
    'utf8',
  );
  assert.match(source, /drafts\.find\(\(app\) => app\.id === developerAppId\)/);
  assert.match(source, /developer && developerAppId && !developerApp/);
  assert.match(source, /Aplikasi tidak tersedia/);
  assert.match(source, /key=\{developerApp\?\.id \?\? 'all'\}/);
});

void test('client, review and release filtering uses app IDs, never matching names', () => {
  const rows = [
    { id: 'one', appId: 'app_A', name: 'Same name' },
    { id: 'two', appId: 'app_B', name: 'Same name' },
    { id: 'legacy', name: 'Same name' },
  ];
  assert.deepEqual(
    forApp(rows, 'app_A', (row) => row.appId).map((row) => row.id),
    ['one'],
  );
  assert.deepEqual(
    forApp(rows, 'missing', (row) => row.appId),
    [],
  );
  assert.equal(
    forApp(rows, undefined, (row) => row.appId),
    rows,
  );
  assert.equal(rows.length, 3);
  const clients = readFileSync(
    new URL('../components/app-clients.tsx', import.meta.url),
    'utf8',
  );
  assert.match(clients, /row\.client\.binding\.appId/);
  assert.match(clients, /row\.manifest\.metadata\.appId/);
  assert.match(clients, /row\.manifest\.appId/);
  assert.match(clients, /5 \* 60 \* 1000/);
  assert.doesNotMatch(clients, /localStorage|sessionStorage/);
});
