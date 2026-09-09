import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
const account = readFileSync(
  new URL('../components/developer-account.tsx', import.meta.url),
  'utf8',
);
const contact = readFileSync(
  new URL('../components/application-contact.tsx', import.meta.url),
  'utf8',
);
void test('account menu loads verified profile and auto-starts merchant SSO without manual linking', () => {
  assert.match(account, /\.request<Account>\('\/account'\)/);
  assert.match(account, /profile\?\.stores.map/);
  assert.match(account, /\/api\/v1\/developer-login\/start/);
  assert.match(account, /body: '\{\}'/);
  assert.doesNotMatch(
    account,
    /Hubungkan akun Emisell|startMerchantLogin\(true\)/,
  );
  assert.doesNotMatch(
    account,
    /session\.organization|localStorage|sessionStorage|document.cookie/,
  );
});
void test('contact edits are app-scoped, persisted on the backend, and protected against stale revisions', () => {
  assert.match(contact, /\/apps\/\$\{appId\}\/contact/);
  assert.match(contact, /'PUT'/);
  assert.match(contact, /revision: contact.revision/);
  assert.match(contact, /error.status === 409/);
  assert.match(contact, /Contact information/);
  assert.doesNotMatch(
    contact,
    /localStorage|sessionStorage|credentials\/rotate/,
  );
});

void test('compact account menu keeps real stores, active dashboard and working logout', () => {
  assert.match(account, /aria-current="page"/);
  assert.match(account, /initials\(store.name\)/);
  assert.match(account, /title=\{store.name\}/);
  assert.match(account, /encodeURIComponent\(store.commonId\)/);
  assert.match(account, /disabled=\{busy\} onClick=\{logout\}/);
  assert.doesNotMatch(account, /Create store|Recent stores|\(paid\)/);
  const css = readFileSync(
    new URL('../app/developer-redesign.css', import.meta.url),
    'utf8',
  );
  assert.match(
    css,
    /\.dev-account-menu \{[^}]*max-width: calc\(100vw - 1rem\)/,
  );
  assert.match(css, /\.dev-account-store-name \{[^}]*text-overflow: ellipsis/);
});
