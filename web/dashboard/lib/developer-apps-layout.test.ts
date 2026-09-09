import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

void test('Apps landing matches the compact reference without changing app creation and selection', () => {
  const workspace = readFileSync(
    new URL('../components/developer-workspace.tsx', import.meta.url),
    'utf8',
  );
  const landing = workspace.slice(
    workspace.indexOf('<section className="dev-applications">'),
    workspace.indexOf('function DeveloperOverview'),
  );
  assert.match(landing, /Create app/);
  assert.match(landing, /setCreating\(true\)/);
  assert.match(landing, /Search by name or app ID/);
  assert.match(landing, /compact/);
  assert.match(landing, /select\(drafts.find/);
  assert.doesNotMatch(
    landing,
    /<Tabs|Dipublikasikan|Status rilis terbaru|dev-footnote/,
  );
  const apps = readFileSync(
    new URL('../components/developer-apps.tsx', import.meta.url),
    'utf8',
  );
  const compact = apps.slice(
    apps.indexOf('if (compact)\n'),
    apps.indexOf('className="mb-3'),
  );
  assert.ok(compact.length > 100, 'Compact card source must be present');
  assert.match(apps, /if \(compact\) return;/);
  // Cards use real app identity, not mock install counts or credential-like values.
  assert.match(apps, /dev-apps-compact-card/);
  assert.match(apps, /openDraft\(draft.id\)/);
  assert.doesNotMatch(compact, /1 install|client_secret|appLifecycleStatus/);
});
