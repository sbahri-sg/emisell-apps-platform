import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import {
  cliCommands,
  documentationTopics,
  isDocumentationTopic,
  resourceScopes,
  adjacentArticles,
  documentationArticles,
  searchDocumentation,
} from './documentation.ts';

const source = (path: string) =>
  readFileSync(new URL(path, import.meta.url), 'utf8');

void test('public documentation has an explicit topic allowlist, not an internal API selector', () => {
  assert.equal(new Set(documentationTopics.map(({ id }) => id)).size, 11);
  for (const { id } of documentationTopics)
    assert.equal(isDocumentationTopic(id), true);
  for (const value of [
    'admin',
    'core',
    'gateway',
    '../../admin',
    'api-keys',
    '',
  ]) {
    assert.equal(isDocumentationTopic(value), false);
  }
  const component = source('../components/documentation.tsx');
  assert.doesNotMatch(
    component,
    /fetch\(|PortalAPI|api-docs.generated|localStorage|sessionStorage|shopify|emisell\.dev/,
  );
  assert.match(component, /href="\/development"/);
  assert.match(component, /id="docs-main"/);
  const root = source('../app/page.tsx');
  assert.match(root, /<Documentation/);
  assert.doesNotMatch(root, /UnifiedPortal|PortalAPI|session|searchParams/);
  const guide = source('../app/docs/[topic]/page.tsx');
  assert.match(guide, /if \(!isDocumentationTopic\(topic\)\) notFound\(\)/);
});

void test('search reads article bodies, ranks title matches, and never returns private routes', () => {
  assert.deepEqual(searchDocumentation('   '), []);
  assert.deepEqual(searchDocumentation('tidak-ada-artikel-ini'), []);
  assert.equal(searchDocumentation('credential')[0].id, 'credentials');
  assert.ok(
    searchDocumentation('read_inventory').some(({ id }) => id === 'scopes'),
  );
  assert.ok(searchDocumentation('SIGTERM').length === 0);
  const result = searchDocumentation('Cara kerja').find(
    ({ id }) => id === 'authentication',
  );
  assert.equal(result?.href, '/docs/authentication#cara-kerja');
  for (const result of searchDocumentation('backend'))
    assert.match(result.href, /^\/docs\/[a-z-]+(?:#[a-z0-9-]+)?$/);
  assert.ok(searchDocumentation('a').length <= 8);
});

void test('article navigation follows the editorial order and handles both ends and unknown IDs', () => {
  assert.equal(adjacentArticles(documentationTopics[0].id).previous, null);
  assert.equal(adjacentArticles(documentationTopics.at(-1)!.id).next, null);
  assert.equal(adjacentArticles('authentication').next?.id, 'credentials');
  assert.equal(adjacentArticles('credentials').previous?.id, 'authentication');
  assert.deepEqual(adjacentArticles('unknown'), { previous: null, next: null });
  for (const article of documentationArticles) {
    assert.ok(article.headings.length > 0);
    assert.equal(
      new Set(article.headings.map(({ id }) => id)).size,
      article.headings.length,
    );
  }
});

void test('protected routes select a workspace but never elevate the account role', () => {
  assert.match(
    source('../app/admin/page.tsx'),
    /<UnifiedPortal surface="admin"/,
  );
  assert.match(
    source('../app/development/page.tsx'),
    /<UnifiedPortal surface="developer"/,
  );
  for (const path of ['../app/admin/page.tsx', '../app/development/page.tsx']) {
    assert.match(source(path), /index: false, follow: false/);
    assert.doesNotMatch(source(path), /searchParams|localStorage/);
  }
  const entry = source('../components/unified-portal.tsx');
  assert.match(entry, /session\.user\.surface !== surface/);
  assert.match(entry, /credentials: 'same-origin'/);
  assert.match(entry, /knownSignedOut=\{!session\}/);
  assert.match(entry, /request\('session'\)/);
  assert.match(entry, /request\('login', \{ email, password \}\)/);
  const portal = source('../components/portal.tsx');
  assert.match(portal, /if \(knownSignedOut\) return/);
  assert.match(portal, /new PortalAPI\(surface\)/);
  assert.match(portal, /developerMainMenu/);
});

void test('public commands and read scopes are documented in the existing CLI, not invented capabilities', () => {
  const cli = source('../../../packages/developer-cli/README.md');
  for (const [command] of cliCommands)
    assert.ok(cli.includes(command), command);
  for (const [scope] of resourceScopes) {
    assert.ok(cli.includes(scope), scope);
    assert.match(scope, /^read_/);
  }
  assert.equal(resourceScopes.length, 7);
});
