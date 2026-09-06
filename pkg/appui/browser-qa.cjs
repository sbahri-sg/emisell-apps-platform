// Run with PLAYWRIGHT_MODULE pointing to an installed Playwright module.
// Requires a fresh synthetic demo on fixed loopback origins. Revokes demo only.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
(async () => {
  const output = process.env.UI_QA_OUTPUT;
  assert(output, 'UI_QA_OUTPUT must name a local evidence directory');
  await fs.mkdir(output, { recursive: true });
  const browser = await chromium.launch({ headless: true, executablePath: process.env.PLAYWRIGHT_EXECUTABLE || undefined });
  try {
    const page = await browser.newPage();
    await page.goto('http://127.0.0.1:4320/');
    await page.getByRole('button', {name:'Open demo', exact:true}).click();
    const frame = page.frameLocator('iframe');
    await frame.locator('#status').filter({hasText:'Terhubung'}).waitFor();
    for (const [name, width, zoom] of [['desktop',1280,1],['mobile',360,1],['text-200',720,2]]) {
      await page.setViewportSize({width, height:1000});
      await frame.locator('html').evaluate((el, zoom) => { el.style.fontSize = `${16 * zoom}px`; }, zoom);
      const overflow = await frame.locator('html').evaluate(el => el.scrollWidth > el.clientWidth + 1);
      assert.equal(overflow, false, `${name}: horizontal overflow`);
      await frame.locator('#refresh').focus();
      assert.equal(await frame.locator('#refresh').evaluate(el => getComputedStyle(el).outlineStyle), 'solid');
      if (width === 360) assert((await frame.locator('#refresh').boundingBox()).height >= 44);
      await page.screenshot({path:path.join(output, `${name}.png`), fullPage:true});
    }
    const count = Number(await frame.locator('#renewals').textContent());
    await frame.locator('#refresh').press('Enter');
    await frame.locator('#renewals').filter({hasText:String(count+1)}).waitFor();
    await page.getByRole('button', {name:'Cabut akses uji', exact:true}).click();
    // Issuance fails through an intentionally opaque bridge error: the UI must
    // clear identity without claiming it knows the underlying revocation reason.
    await frame.locator('#status').filter({hasText:'Koneksi belum tersedia'}).waitFor();
    assert.equal(await frame.locator('#merchant').textContent(), '—');
    assert.equal(await frame.locator('#feedback').getAttribute('data-tone'), 'warning');
    await page.screenshot({path:path.join(output,'denied.png'), fullPage:true});
    console.log('PASS desktop, mobile, 200% text, overflow, keyboard renewal, revocation and redacted identity');
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
