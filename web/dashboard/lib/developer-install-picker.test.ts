import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { developerInstallURL } from './developer-stores.ts';
import { installSelectionURL } from './developer-install-navigation.ts';

const app = 'app_WTFRTL3YUTU7PRKZLLWBJTLENR';
const account = {
  sellerOrigin: 'http://localhost:3000',
  profile: {
    name: 'Developer',
    email: 'dev@example.test',
    stores: [{ id: 'merchant1', commonId: 'store-1', name: 'My store' }],
  },
};
void test('merchant picker hands the selected app and active version to the Core consent route', () => {
  assert.equal(
    developerInstallURL(account, 'merchant1', app, '1.0.0'),
    `http://localhost:3000/store/store-1/app/grant?app=${app}&version=1.0.0`,
  );
  assert.equal(developerInstallURL(account, 'foreign', app, '1.0.0'), null);
  assert.equal(
    developerInstallURL(account, 'merchant1', '../app', '1.0.0'),
    null,
  );
  assert.equal(
    developerInstallURL(account, 'merchant1', app, '1.0.0&consent=true'),
    null,
  );
  assert.equal(
    developerInstallURL(
      { ...account, sellerOrigin: 'javascript:alert(1)' },
      'merchant1',
      app,
      '1.0.0',
    ),
    null,
  );
  const source = readFileSync(
    new URL('../components/developer-install-picker.tsx', import.meta.url),
    'utf8',
  );
  assert.match(source, /install-url/);
  assert.match(source, /window.location.assign\(target\)/);
  assert.doesNotMatch(source, /\/account|Search your stores/);
  assert.doesNotMatch(
    source,
    /client\.install|client\.prepare|document.cookie|localStorage|consented|installed=true/,
  );
  const workspace = readFileSync(
    new URL('../components/developer-workspace.tsx', import.meta.url),
    'utf8',
  );
  assert.match(workspace, /setSelectingMerchant\(true\)/);
});
void test('handoff accepts only the requested app and merchant selection route', () => {
  const target = `http://localhost:3000/auth/stores?app=${app}&version=1.0.0`;
  assert.equal(installSelectionURL(target, app), target);
  assert.equal(
    installSelectionURL(
      target.replace('http://localhost:3000', 'https://seller.emisell.com'),
      app,
    ),
    target.replace('http://localhost:3000', 'https://seller.emisell.com'),
  );
  for (const value of [
    null,
    '/auth/stores',
    'javascript:alert(1)',
    target + '&next=https://evil.test',
    target + '&app=' + app,
    target + '#extra',
    target.replace('/auth/stores', '/development'),
    target.replace('localhost', 'user:password@localhost'),
    target.replace('1.0.0', 'invalid'),
  ]) {
    assert.equal(installSelectionURL(value, app), null);
  }
  assert.equal(installSelectionURL(target, 'app_' + 'A'.repeat(26)), null);
});
