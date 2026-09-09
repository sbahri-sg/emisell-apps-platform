import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import { developerActivity } from './developer-activity.ts';
import {
  developerAppMenu,
  developerMenuView,
  developerLocation,
  developerSearch,
} from './developer-navigation.ts';
import { portalView } from './surfaces.ts';
import type { Draft, Submission } from './portal.ts';

const app = {
  id: 'app-a',
  revision: 2,
  updatedAt: '2026-09-09T03:00:00Z',
} as Draft;
void test('app navigation has five contextual pages and restores each after reload', () => {
  assert.deepEqual(
    developerAppMenu.map((item) => item.label),
    ['Overview', 'Monitoring', 'Logs', 'Versions', 'App settings'],
  );
  for (const item of developerAppMenu) {
    assert.deepEqual(developerLocation(developerSearch('', item.id, app.id)), {
      view: item.id,
      appId: app.id,
    });
    if (item.id !== 'apps')
      assert.equal(portalView('admin', `?view=${item.id}`), 'overview');
  }
  assert.equal(developerMenuView('reviews'), 'versions');
  assert.equal(developerMenuView('app-clients'), 'app-settings');
});
void test('Logs derives lifecycle events from the selected app, never invented runtime metrics', () => {
  const rows = [
    {
      id: 'one',
      appId: app.id,
      version: '1.0.0',
      status: 'approved',
      createdAt: '2026-09-09T01:00:00Z',
      decidedAt: '2026-09-09T02:00:00Z',
      feedback: 'Accepted',
    },
    {
      id: 'two',
      appId: 'app-b',
      version: '9.0.0',
      status: 'approved',
      createdAt: '2026-09-09T04:00:00Z',
      decidedAt: '2026-09-09T05:00:00Z',
    },
  ] as Submission[];
  const events = developerActivity(app, rows);
  assert.deepEqual(
    events.map((row) => row.type),
    ['draft', 'decision', 'submission'],
  );
  assert.equal(events[1].feedback, 'Accepted');
  assert.equal(events[2].status, 'submitted');
  assert.equal(
    events.some((row) => row.submissionId === 'two'),
    false,
  );
  assert.equal(rows[0].id, 'one');
  assert.deepEqual(developerActivity({ ...app, updatedAt: 'invalid' }, []), []);
});
void test('creating an app opens Overview, while version editing preserves existing backend review', () => {
  const source = readFileSync(
    new URL('../components/portal.tsx', import.meta.url),
    'utf8',
  );
  const create = source.slice(
    source.indexOf('create={(name) =>'),
    source.indexOf(") : view === 'staff'"),
  );
  assert.match(create, /navigate\('apps', value.app.id\)/);
  assert.doesNotMatch(create, /setEditor\(value.app\)/);
  assert.match(source, /new PortalAPI\(surface\)/);
  assert.match(source, /<DraftEditor/);
  const sections = readFileSync(
    new URL('../components/developer-app-sections.tsx', import.meta.url),
    'utf8',
  );
  assert.match(sections, /Belum tersedia bukan berarti tidak ada error/);
  assert.match(sections, /bukan seluruh audit historis/);
  assert.doesNotMatch(sections, /fetch\(|\.request\(|Shopify|0 ms|100%/);
});
