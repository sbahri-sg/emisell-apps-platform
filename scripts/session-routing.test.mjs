import assert from 'node:assert/strict';
import test from 'node:test';
import {
  developerDestination,
  developerLoginPath,
  isDeveloperConsolePath,
  isDeveloperContentEndpoint,
} from '../lib/app-platform/access-routing.mjs';

test('developer console routes are protected while the session API stays machine-readable', () => {
  for (const path of ['/', '/overview', '/apps', '/apps/example/webhooks', '/stores', '/docs', '/accept-invitation']) {
    assert.equal(isDeveloperConsolePath(path), true, path);
  }
  assert.equal(isDeveloperContentEndpoint('/docs/content'), true);
  assert.equal(isDeveloperConsolePath('/login'), false);
  assert.equal(isDeveloperConsolePath('/merchant/apps'), false);
  assert.equal(isDeveloperConsolePath('/admin'), false);
});

test('developer return destinations remain local and console-scoped', () => {
  assert.equal(developerDestination('/apps/example?tab=home'), '/apps/example?tab=home');
  assert.equal(developerDestination('/'), '/overview');
  for (const unsafe of ['https://attacker.example', '//attacker.example', '/admin', '/docs/content', '/apps\\evil', '/apps%5cevil', '/apps#secret']) {
    assert.equal(developerDestination(unsafe), '/overview', unsafe);
  }
  assert.equal(
    developerLoginPath('/apps/example?tab=home', 'expired'),
    '/login?reason=expired&next=%2Fapps%2Fexample%3Ftab%3Dhome',
  );
});
