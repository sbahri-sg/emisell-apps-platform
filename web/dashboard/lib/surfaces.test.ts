import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import test from 'node:test';
import { SURFACES } from './surfaces.ts';

void test('three product surfaces have distinct, fixed local ports', () => {
  assert.deepEqual(
    SURFACES.map(({ id, port }) => [id, port]),
    [
      ['admin', 4317],
      ['store', 4318],
      ['developer', 4319],
    ],
  );
  assert.equal(new Set(SURFACES.map((surface) => surface.port)).size, 3);
});
void test('merchant UI, state adapters, and their styles are removed, not hidden', () => {
  for (const path of [
    'components/dashboard.tsx',
    'components/local-dashboard.tsx',
    'components/operations-panel.tsx',
    'components/payments-panel.tsx',
    'lib/dashboard-model.ts',
    'lib/api-platform.ts',
    'lib/operations.ts',
    'lib/payments.ts',
    'app/dashboard.css',
    'app/operations.css',
    'app/payments.css',
  ]) {
    assert.equal(
      existsSync(new URL('../' + path, import.meta.url)),
      false,
      path,
    );
  }
});
void test('entrypoint cannot select merchant mode, use a workspace session, or fetch tenant data', () => {
  const page = readFileSync(
    new URL('../app/page.tsx', import.meta.url),
    'utf8',
  );
  assert.doesNotMatch(
    page,
    /APIPlatform|LocalDashboard|dashboard-model|searchParams|URLSearchParams|VITE_EMISELL_LOCAL|fetch\(|localStorage|sessionStorage/,
  );
  assert.match(page, /<Documentation/);
  assert.doesNotMatch(page, /<form|onClick|href="http:\/\/localhost:431[89]/);
});
