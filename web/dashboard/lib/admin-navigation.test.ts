import test from 'node:test';
import assert from 'node:assert/strict';
import { adminNavigationGroups } from './admin-navigation.ts';

test('admin navigation retains every existing route exactly once', () => {
  const ids = adminNavigationGroups.flatMap(group => group.ids);
  assert.equal(new Set(ids).size, ids.length);
  assert.deepEqual([...ids].sort(), [
    'overview', 'apps', 'developers', 'activity', 'staff', 'reviews',
    'testing', 'app-clients', 'integration-releases', 'ui-releases',
    'catalog', 'api-docs', 'api-keys', 'scopes',
  ].sort());
  assert.deepEqual(adminNavigationGroups.map(group => group.label), [
    'Utama', 'Review & distribusi', 'Integrasi & akses', 'Platform',
  ]);
  assert.deepEqual(adminNavigationGroups.filter(group => group.collapsible).map(group => group.label), ['Integrasi & akses']);
});
