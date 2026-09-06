import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import { searchCatalog, readApp } from "./catalog.ts";
void test("public search and detail never send portal cookies or mutate data", async () => {
  const original = globalThis.fetch;
  const calls: { url: string; options: RequestInit }[] = [];
  globalThis.fetch = async (input, options) => {
    calls.push({ url: String(input), options: options ?? {} });
    return Response.json({ apps: [], total: 0, page: 1, pageSize: 20 });
  };
  try {
    await searchCatalog("a & b", "shipping/v1", 2);
    await readApp("cat_test");
    assert.equal(calls[0].url, "/api/v1/store/apps?search=a+%26+b&capability=shipping%2Fv1&page=2");
    assert.equal(calls[1].url, "/api/v1/store/apps/cat_test");
    for (const c of calls) {
      assert.equal(c.options.credentials, "omit");
      assert.equal(c.options.cache, "no-store");
      assert.equal(c.options.method, undefined);
      assert.equal(c.options.body, undefined);
    }
    assert.throws(() => readApp("../admin"));
    assert.throws(() => searchCatalog("", "bad", 1));
    assert.throws(() => searchCatalog("", "", 0));
  } finally {
    globalThis.fetch = original;
  }
});
void test("unavailable and withdrawn listings fail explicitly", async () => {
  const original = globalThis.fetch;
  try {
    globalThis.fetch = async () => Response.json({ error: "not_found" }, { status: 404 });
    await assert.rejects(readApp("cat_test"), /tidak tersedia/);
    globalThis.fetch = async () => Response.json({ error: "unavailable" }, { status: 503 });
    await assert.rejects(searchCatalog("", "", 1), /belum dapat/);
  } finally {
    globalThis.fetch = original;
  }
});
void test("store is a fixed separate public surface", () => {
  const config = readFileSync(new URL("../vite.config.ts", import.meta.url), "utf8");
  assert.match(config, /4318/);
  assert.match(config, /\/api\/v1\/store/);
  assert.doesNotMatch(config, /\/api\/v1\/admin|\/api\/v1\/developer/);
  const page = readFileSync(new URL("../app/page.tsx", import.meta.url), "utf8");
  assert.doesNotMatch(page, /localStorage|sessionStorage|workspace|dangerouslySetInnerHTML/);
  assert.match(page, /Instalasi belum tersedia/);
});
