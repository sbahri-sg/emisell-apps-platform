import {test} from 'node:test';
import assert from 'node:assert/strict';
import {resourceKind,resourcePage,resourceQuery} from '../templates/react-router/server/resource-query.mjs';
for(const kind of ['catalogs','collections'])test(`${kind} template validates existing paths, queries and product references`,()=>{
 const path=`/v1/${kind}`;
 assert.equal(resourceKind(path),kind);
 assert.deepEqual(resourceQuery(path,new URLSearchParams('q=Blue&limit=5')),{limit:5,q:'Blue'});
 for(const suffix of ['/first','/FIRST','/bulk-delete','/a/secret'])assert.throws(()=>resourceKind(path+suffix));
 for(const query of ['merchantId=other','limit=21','q=%20Blue','q=','q='+encodeURIComponent('a'.repeat(101))])assert.throws(()=>resourceQuery(path,new URLSearchParams(query)));
 assert.throws(()=>resourceQuery(path+'/a',new URLSearchParams('limit=1')));
 const row={id:'a',...(kind==='catalogs'?{title:'A'}:{name:'A'}),productCount:1,secret:'hidden'};
 const ref={productId:'p',...(kind==='catalogs'?{variantId:null}:{}),price:'hidden'};
 const output=resourcePage(path+'/a',{data:{...row,products:[ref]}});
 assert.equal(JSON.stringify(output).includes('hidden'),false);
 assert.throws(()=>resourcePage(path+'/b',{data:{...row,products:[ref]}}));
 assert.throws(()=>resourcePage(path+'/a',{data:{...row,productCount:2,products:[ref]}}));
 assert.deepEqual(resourcePage(path,{data:[],meta:{nextCursor:null}}),{data:[],meta:{nextCursor:null}});
 assert.throws(()=>resourcePage(path,{data:[row]}));
});
