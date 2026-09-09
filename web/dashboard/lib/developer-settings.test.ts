import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
const source = readFileSync(
  new URL('../components/developer-settings.tsx', import.meta.url),
  'utf8',
);
const credentials = readFileSync(
  new URL('../components/application-credentials.tsx', import.meta.url),
  'utf8',
);
void test('Settings shows app-level credentials ahead of account and app information', () => {
  assert.match(source, /<h1>Settings<\/h1>/);
  assert.ok(
    source.indexOf('<ApplicationCredentials') <
      source.indexOf('<ApplicationContact'),
  );
  assert.match(source, /<ApplicationCredentials\s+key=\{app.id\}/);
  assert.match(credentials, /<dl className="dev-credential-rows">/);
  assert.match(credentials, /<dt>Client ID<\/dt>/);
  assert.match(credentials, /<dt>Secret<\/dt>/);
  assert.match(credentials, /Tampilkan Secret/);
  assert.match(credentials, /Salin Secret/);
  assert.match(credentials, /Rotate/);
});
void test('Credentials require explicit reveal and confirmed rotation; secrets are transient', () => {
  assert.match(credentials, /\/apps\/\$\{appId\}\/credentials/);
  assert.match(credentials, /\/reveal/);
  assert.match(credentials, /\/rotate/);
  assert.match(credentials, /<AlertDialogTitle>Rotasi Secret aplikasi/);
  assert.match(credentials, /version: credential.version/);
  assert.match(credentials, /document.hidden/);
  assert.match(credentials, /mounted.current && token === generation.current/);
  assert.match(credentials, /60000/);
  assert.doesNotMatch(credentials, /localStorage|sessionStorage|console\./);
  assert.doesNotMatch(
    credentials,
    /\/app-clients|secretVersion|initialClientId/,
  );
  assert.match(source, /Kompatibilitas client rilis lama/);
  assert.doesNotMatch(source, /Informasi akun|<dt>Organisasi/);
});
void test('Legacy app-client links and Settings select the same app-scoped surface', () => {
  const portal = readFileSync(
    new URL('../components/portal.tsx', import.meta.url),
    'utf8',
  );
  assert.match(
    portal,
    /developerApp &&\s*\['app-settings', 'app-clients'\]\.includes\(view\)/,
  );
  assert.match(portal, /<DeveloperSettings\s+key=\{developerApp.id\}/);
  assert.match(source, /advanced &&/);
});
