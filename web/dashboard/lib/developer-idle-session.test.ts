import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const read = (path: string) => readFileSync(new URL(path, import.meta.url), 'utf8');
void test('only trusted foreground input renews; passive checks use GET and never revoke merchant sessions', () => {
  const source = read('./use-developer-idle-session.ts');
  assert.match(source, /event.isTrusted/);
  assert.match(source, /document.visibilityState === 'visible'/);
  assert.match(source, /method: renew \? 'POST' : 'GET'/);
  assert.match(source, /setTimeout\(\(\) => void check\(\)/);
  assert.match(source, /response.status === 401/);
  assert.doesNotMatch(source, /setInterval|\/logout|document.cookie|localStorage/);
});
void test('landing login opens development and developer password form is retired', () => {
  assert.match(read('../components/documentation.tsx'), /Login <ArrowUpRight/);
  const portal = read('../components/portal.tsx');
  assert.match(portal, /useDeveloperIdleSession\(developer && !!session\)/);
  assert.doesNotMatch(portal, /Hubungkan akun Emisell|Sudah memiliki akun developer lama/);
  assert.match(read('../components/developer-account.tsx'), /started.current \|\|/);
});
