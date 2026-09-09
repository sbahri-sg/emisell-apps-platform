import {test} from 'node:test';
import assert from 'node:assert/strict';
import {run} from '../src/cli.mjs';
import {Client} from '../src/client.mjs';
import {browserLogin} from '../src/browser-auth.mjs';
const app='app_'+'A'.repeat(26);
const cookie='emisell_portal_session='+'C'.repeat(52);
const json=(v,h={})=>new Response(JSON.stringify(v),{headers:{'content-type':'application/json',...h}});
function fixture(){
 let value={origin:'https://apps.example.com',cookie,expiresAt:Date.now()+10000,accountId:'u1'};
 const calls=[],opened=[],output=[];
 const store={load:async()=>structuredClone(value),save:async v=>{value=structuredClone(v);},clear:async()=>{value=null;}};
 const fetcher=async(url,options)=>{
  calls.push([url,options]);
  if(url.endsWith('/session'))return json({user:{id:'u1',surface:'developer',role:'developer'}});
  if(url.endsWith('/activity'))return json({expiresAt:new Date(Date.now()+3600000).toISOString()});
  if(url.endsWith('/account'))return json({sellerOrigin:'https://seller.example.com',profile:{stores:[{id:'m1',commonId:'my-store',name:'My Store'}]}});
  if(url.endsWith('/apps'))return json({apps:[{id:app,document:{name:'Product app'},activeVersion:{document:{version:'1.0.0'}}}]});
  if(url.endsWith('/install-url'))return json({url:`https://seller.example.com/auth/stores?app=${app}&version=1.0.0`});
  throw Error('unexpected route');
 };
 return {store,calls,opened,output,fetcher,options:{store,fetcher,output:v=>output.push(v),open:async v=>opened.push(v),cwd:'/tmp/project',interactive:false}};
}
test('install selects owned app/store and opens consent only; remembered choice never adds mutation',async()=>{
 const f=fixture();
 await run(['app','install','--app',app,'--store','my-store'],f.options);
 assert.equal(f.opened[0],`https://seller.example.com/auth/stores?app=${app}&version=1.0.0&store=m1`);
 assert.ok(f.calls.every(([url,init])=>init.method==='GET'||url.endsWith('/activity')));
 assert.equal((await f.store.load()).selections['/tmp/project'].storeId,'m1');
 await run(['app','install','--no-open'],f.options);
 assert.equal(f.opened.length,1);
 assert.ok(f.output.join('').includes('Belum dianggap terpasang'));
});
test('foreign store and foreign app cannot be selected or opened',async()=>{
 for(const flags of [['--app',app,'--store','foreign'],['--app','app_'+'B'.repeat(26),'--store','m1']]){
  const f=fixture();await assert.rejects(run(['app','install',...flags],f.options));assert.equal(f.opened.length,0);
 }
});
test('untrusted install origin is rejected',async()=>{
 const f=fixture(),fetcher=async(url,init)=>url.endsWith('/install-url')?json({url:`https://evil.example/auth/stores?app=${app}&version=1.0.0`}):f.fetcher(url,init);
 await assert.rejects(run(['app','install','--app',app,'--store','m1'],{...f.options,fetcher}),/Alamat/);
 assert.equal(f.opened.length,0);
});
test('config link saves navigation without opening browser; reset requires explicit selection',async()=>{
 const f=fixture();await run(['app','config','link','--app',app,'--store','m1'],f.options);
 assert.equal(f.opened.length,0);
 await assert.rejects(run(['app','install','--reset'],f.options),/non-interaktif/);
});
test('revoked session clears local secret and never reaches account/apps',async()=>{
 const f=fixture();await assert.rejects(run(['stores','list'],{...f.options,fetcher:async()=>new Response('',{status:401})}),/401/);
 assert.equal(await f.store.load(),null);
});
test('browser login rejects wrong identity and never exposes verifier/session',async()=>{
 const request='A'.repeat(52),verifier='B'.repeat(52),output=[];
 const client=new Client('https://apps.example.com','',async url=>{
  if(url.endsWith('/start'))return json({request,verifier,authorizeUrl:`https://seller.example.com/auth/developer?request=${request}`,expiresIn:300,interval:3});
  if(url.endsWith('/poll'))return json({status:'authorized'},{'set-cookie':`${cookie}; Max-Age=3600`});
  return json({user:{surface:'admin',role:'administrator'}});
 });
 await assert.rejects(browserLogin(client,{output:v=>output.push(v),noOpen:true,wait:async()=>{}}),/bukan akun/);
 assert.ok(!output.join('').includes(verifier));assert.ok(!output.join('').includes(cookie));
});
