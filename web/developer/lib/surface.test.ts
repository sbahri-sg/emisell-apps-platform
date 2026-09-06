import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
void test("developer entrypoint is separate and cannot choose an admin or merchant surface", () => {
  const page = readFileSync(new URL("../app/page.tsx", import.meta.url), "utf8");
  const config = readFileSync(new URL("../vite.config.ts", import.meta.url), "utf8");
  assert.match(page, /Portal surface="developer"/);
  assert.doesNotMatch(page, /searchParams|workspace|localStorage|sessionStorage/);
  assert.match(config, /4319/);
  assert.match(config, /\/api\/v1\/developer/);
  assert.doesNotMatch(config, /\/api\/v1\/admin/);
});
