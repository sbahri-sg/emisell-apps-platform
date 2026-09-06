import test from 'node:test';
import assert from 'node:assert/strict';
import {connectFrame, mountEmbeddedApp, protocol} from './bridge.mjs';

test('bridge pins window/origin, rejects replay and stops after disposal', async () => {
  let listener, load, calls=0;
  const sent=[];
  const child={postMessage:(...args)=>sent.push(args)};
  const frame={contentWindow:child,addEventListener:(_,fn)=>{load=fn;},removeEventListener(){}};
  const host={location:{origin:'https://core.example'},addEventListener:(_,fn)=>{listener=fn;},removeEventListener(){}};
  const dispose=connectFrame({host,frame,appOrigin:'https://app.example',parentOrigin:host.location.origin,launchURL:'https://app.example/app',getSession:async()=>{calls++;return {launch:{url:'https://app.example/app',parentOrigin:host.location.origin},identityToken:'signed-token',expiresIn:60};}});
  const data={protocol,type:'identity.request',nonce:'a'.repeat(24)};
  await listener({source:{},origin:'https://app.example',data});
  await listener({source:child,origin:'https://evil.example',data});
  assert.equal(calls,0);
  await listener({source:child,origin:'https://app.example',data});
  await listener({source:child,origin:'https://app.example',data});
  assert.equal(calls,1); assert.equal(sent[0][1],'https://app.example');
  load(); dispose();
  await listener({source:child,origin:'https://app.example',data});assert.equal(calls,1);
});

test('external launch cannot mount an iframe or send an identity token', async () => {
  const session = {launch:{mode:'external',url:'https://app.example/',parentOrigin:'https://core.example'},identityToken:'must-not-send',expiresIn:60};
  await assert.rejects(mountEmbeddedApp({container:{},getSession:async()=>session,host:{}}), /Embedded launch required/);
  let listener;
  const sent=[];
  const child={postMessage:message=>sent.push(message)};
  const frame={contentWindow:child,addEventListener(){},removeEventListener(){}};
  const host={location:{origin:'https://core.example'},addEventListener:(_,fn)=>{listener=fn;},removeEventListener(){}};
  connectFrame({host,frame,appOrigin:'https://app.example',parentOrigin:host.location.origin,launchURL:session.launch.url,getSession:async()=>session});
  await listener({source:child,origin:'https://app.example',data:{protocol,type:'identity.request',nonce:'c'.repeat(24)}});
  assert.equal(sent[0].type,'identity.error');
  assert.equal(sent[0].token,undefined);
});

test('navigation while issuing drops token; outage sends no token', async()=>{
  let listener, load, resolve;
  const sent=[];const child={postMessage:d=>sent.push(d)};
  const frame={contentWindow:child,addEventListener:(_,f)=>{load=f},removeEventListener(){}};
  const host={location:{origin:'https://core.example'},addEventListener:(_,f)=>{listener=f},removeEventListener(){}};
  let offline=false;
  connectFrame({host,frame,appOrigin:'https://app.example',parentOrigin:host.location.origin,launchURL:'https://app.example/',getSession:()=>offline?Promise.reject(Error('outage')):new Promise(r=>{resolve=r})});
  const event={source:child,origin:'https://app.example',data:{protocol,type:'identity.request',nonce:'b'.repeat(24)}};
  const pending=listener(event);load();resolve({launch:{url:'https://app.example/',parentOrigin:host.location.origin},identityToken:'secret',expiresIn:60});await pending;
  assert.equal(sent.length,0);offline=true;await listener(event);assert.equal(sent[0].type,'identity.error');assert.equal(sent[0].token,undefined);
});
