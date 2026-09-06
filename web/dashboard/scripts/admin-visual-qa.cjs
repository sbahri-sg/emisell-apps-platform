// Read-only visual QA: all API calls intercepted; no user cookies or mutations.
const {chromium}=require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assert=require('node:assert/strict');
const fs=require('node:fs/promises');
const path=require('node:path');
(async()=>{
 const output=process.env.UI_QA_OUTPUT; assert(output);
 await fs.mkdir(output,{recursive:true});
 const browser=await chromium.launch({headless:true,executablePath:process.env.PLAYWRIGHT_EXECUTABLE||undefined});
 try {
 const page=await browser.newPage();
 const errors=[]; page.on('pageerror',e=>errors.push(e.message));
 const submissions=Array.from({length:8},(_,i)=>({id:`qa-${i}`,appId:`app-${i}`,organizationId:'qa',submitterId:'qa',draftRevision:1,version:'1.0.0',snapshot:{name:i===0?'Application avec un nom très long pour vérifier la mise en page':'Application QA '+i},status:i%2?'approved':'submitted',createdAt:`2026-09-0${8-i}T10:00:00Z`,decidedAt:null,reviewerId:'',feedback:''}));
 await page.route('**/api/**',route=>{
  const p=new URL(route.request().url()).pathname;
  if(p.endsWith('/ui-releases')) return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({nextAfterId:'',releases:Array.from({length:8},(_,i)=>({id:`ui-qa-${i}`,revision:1,status:i%2?'submitted':'signed',manifest:{appId:`qa-${i}`,name:`UI QA ${i}`,summary:'Synthetic release for layout testing',version:'1.0.0',url:'https://example.invalid/',mode:i%2?'external':'embedded'}}))})});
  const scopes=Array.from({length:8},(_,i)=>({handle:`read_qa_${i}`,resource:i%2?'Orders':'Products',action:'read',status:'planned',grantable:false,implies:[],requiresAny:[],review:'standard',notes:'Synthetic scope for isolated layout testing.'}));
  if(p.endsWith('/access-scopes') || p.endsWith('/access-scopes/verification')) return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(p.endsWith('/verification')?{profile:'qa',checkedAt:'2026-09-07T00:00:00Z',verification:'platform_build_inventory',coreChecked:false,environment:'local',contractRevision:'a'.repeat(64),operations:[],scopes:scopes.map(s=>({handle:s.handle,status:'planned',grantable:false,contractStatus:'missing',operations:[],blockers:['Test fixture: not implemented']}))}:{profile:'qa',checkedAt:'2026-09-07',source:'qa',grantable:false,scopes})});
  let body=p.endsWith('/session')?{user:{id:'qa',email:'qa@example.invalid',role:'administrator',surface:'admin'}}:p.endsWith('/submissions')?{submissions}:p.endsWith('/catalog')?{releases:[]}:p.endsWith('/platform-keys') && route.request().method()==='GET'?{limit:200,keys:[{id:'qa-local',name:'Emisell Backend — Lokal',access:'platform_full',status:'active',createdAt:'2026-09-06T10:00:00Z'},{id:'qa-old',name:'Kunci lama',access:'platform_full',status:'revoked',createdAt:'2026-08-20T10:00:00Z'}]}:null;
  return route.fulfill({status:body?200:404,contentType:'application/json',body:JSON.stringify(body||{code:'not_found'})});
 });
 for(const [name,width] of [['desktop',1536],['mobile',390]]){
  await page.setViewportSize({width,height:1024});
  await page.goto('http://localhost:4317/?view=reviews');
  await page.getByRole('heading',{name:'Pengajuan review',exact:true}).waitFor();
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false,`${name} overflow`);
  await page.screenshot({path:path.join(output,`${name}.png`),fullPage:true});
  await page.getByRole('tab',{name:'Antrean',exact:true}).click();
  await page.waitForFunction(()=>[...document.querySelectorAll('button')].filter(b=>b.textContent==='Lihat detail' && b.checkVisibility()).length===4);
  await page.getByRole('textbox',{name:'Cari aplikasi atau versi'}).fill('no-match');
  await page.getByText('Tidak ada pengajuan yang cocok dengan filter.').waitFor();
  if(width===390){await page.getByRole('button',{name:'Buka navigasi'}).click();await page.getByRole('button',{name:'Ringkasan',exact:true}).click();await page.getByRole('heading',{name:'Ringkasan platform'}).waitFor();}
  await page.goto('http://localhost:4317/?view=api-keys');
  await page.getByText('Emisell Backend — Lokal',{exact:true}).waitFor();
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false,`${name} API keys overflow`);
  await page.screenshot({path:path.join(output,`api-keys-${name}.png`),fullPage:true});
  await page.getByRole('tab',{name:'Dicabut',exact:true}).click();
  await page.getByText('Kunci lama',{exact:true}).waitFor();
  assert.equal(await page.getByText('Emisell Backend — Lokal',{exact:true}).count(),0);
  await page.getByRole('textbox',{name:'Cari nama kunci'}).fill('no-match');
  await page.getByText('Tidak ada kunci yang sesuai dengan pencarian atau status ini.').waitFor();
  await page.getByRole('button',{name:'Buat API key',exact:true}).click();
  await page.getByRole('textbox',{name:'Nama koneksi'}).waitFor();
  await page.getByRole('button',{name:'Batal',exact:true}).click();
  await page.getByRole('link',{name:'Buka dokumentasi API'}).click();
  await page.getByRole('heading',{name:'Dokumentasi API',exact:true}).waitFor();
  await page.getByRole('article',{name:'Detail endpoint'}).waitFor();
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false,`${name} docs overflow`);
  await page.screenshot({path:path.join(output,`docs-${name}.png`),fullPage:true});
  await page.getByRole('tab',{name:'Response & error',exact:true}).click();
  await page.getByLabel('Status response',{exact:true}).waitFor();
  await page.getByRole('textbox',{name:'Cari endpoint API'}).fill('no-such-endpoint-qa');
  await page.getByText('Ubah kata pencarian atau pilih kelompok API lain.').waitFor();
  await page.getByRole('button',{name:'Hapus pencarian'}).click();
  await page.goto('http://localhost:4317/?view=scopes');
  await page.getByText('read_qa_0',{exact:true}).waitFor();
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false,`${name} scopes overflow`);
  await page.screenshot({path:path.join(output,`scopes-${name}.png`),fullPage:true});
  await page.getByRole('button',{name:'Halaman scope berikutnya'}).click();
  await page.getByText('read_qa_6',{exact:true}).waitFor();
  await page.getByRole('tab',{name:'Aktif',exact:true}).click();
  await page.getByText('Tidak ada scope yang cocok. Ubah pencarian atau filter.').waitFor();
  await page.getByRole('tab',{name:'Semua scope',exact:true}).click();
  await page.getByLabel('Resource',{exact:true}).selectOption('Products');
  assert.equal(await page.getByText('read_qa_1',{exact:true}).count(),0);
  await page.goto('http://localhost:4317/?view=ui-releases');
  await page.getByText('UI QA 0',{exact:true}).waitFor();
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false,`${name} releases overflow`);
  await page.screenshot({path:path.join(output,`releases-${name}.png`),fullPage:true});
  await page.getByRole('button',{name:'Halaman rilis berikutnya'}).click();
  await page.getByText('UI QA 6',{exact:true}).waitFor();
  await page.getByRole('tab',{name:'Ditolak',exact:true}).click();
  await page.getByText('Tidak ada rilis yang cocok dengan filter.').waitFor();
  await page.goto('http://localhost:4317/?view=apps');
  await page.getByRole('heading',{name:'Aplikasi',exact:true}).waitFor();
  await page.getByRole('button',{name:'Lihat aplikasi'}).first().waitFor();
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false,`${name} apps overflow`);
  await page.screenshot({path:path.join(output,`apps-${name}.png`),fullPage:true});
  await page.getByRole('button',{name:'Lihat aplikasi'}).first().click();
  await page.getByRole('button',{name:'Kembali ke aplikasi'}).click();
  await page.getByRole('textbox',{name:'Cari nama aplikasi',exact:true}).fill('qa-no-match');
  await page.getByText('Tidak ada aplikasi yang cocok dengan daftar atau filter ini.').waitFor();
 }
 assert.deepEqual(errors,[]);console.log('PASS desktop/mobile overflow, filters, empty state, mobile navigation; synthetic API only');
 } finally {await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
