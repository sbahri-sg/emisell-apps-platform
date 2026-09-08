import { createServer } from 'node:http';
import { randomBytes } from 'node:crypto';
import { productQuery, productPage } from './product-query.mjs';
import { resourceQuery, resourcePage } from './resource-query.mjs';

// Shared by the real framework server and isolated security tests.
export async function startBoundary({ config, port = 4330, host = '127.0.0.1', production = false, isWebReady = () => true, adapters = {}, handler = (_req, res) => { res.writeHead(404); res.end(); } }) {
  if (!Number.isInteger(port) || port < 0 || port > 65535) throw Error('Port tidak valid.');
  const { verifySession, readProducts, readResource } = adapters;
  const { parentOrigin: parent, appOrigin, endpointProof: proof } = config;
  let inflight = 0;
  const server = createServer((req, res) => {
    // Bound concurrent work per process; ingress still needs its own rate limits.
    if (production && inflight >= 64) {
      res.writeHead(503, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store', 'Connection': 'close' }); res.end('{"error":"busy"}'); return;
    }
    inflight++; let released = false;
    const release = () => { if (!released) { released = true; inflight--; } };
    res.once('finish', release); res.once('close', release);
    Promise.resolve(handle(req, res)).catch(() => {
      if (!res.headersSent) res.writeHead(500, { 'Content-Type': 'application/json' });
      res.end('{"error":"request_failed"}');
    });
  });
  async function handle(req, res) {
    const loopbackHost = `127.0.0.1:${server.address().port}`;
    const expected = production ? new URL(appOrigin).host : loopbackHost;
    const trustedOrigin = appOrigin || `http://${expected}`;
    const nonce = randomBytes(24).toString('base64');
    res.locals = { nonce };
    res.setHeader('Cache-Control', 'no-store');
    res.setHeader('X-Content-Type-Options', 'nosniff');
    res.setHeader('Referrer-Policy', 'no-referrer');
    // Vite injects styles during local development. Scripts still require a nonce.
    res.setHeader('Content-Security-Policy', `default-src 'self'; script-src 'self' 'nonce-${nonce}'; style-src 'self' 'unsafe-inline'; connect-src 'self'${production ? '' : ' ' + trustedOrigin.replace(/^http/, 'ws')}; frame-ancestors ${parent}; object-src 'none'; base-uri 'none'; form-action 'self'`);
    // Forwarded headers never select authority, origin or URLs for this app.
    for (const key of Object.keys(req.headers)) if (key === 'forwarded' || key.startsWith('x-forwarded-')) delete req.headers[key];
    const health = ['/health/live', '/health/ready'].includes(req.url);
    const localHealth = health && ['127.0.0.1', '::1', '::ffff:127.0.0.1'].includes(req.socket.remoteAddress) && req.headers.host === loopbackHost;
    if (req.rawHeaders.filter((v, i) => i % 2 === 0 && v.toLowerCase() === 'host').length !== 1) { res.writeHead(403); res.end(); return; }
    if (health && (localHealth || req.headers.host === expected)) {
      if (!['GET', 'HEAD'].includes(req.method)) { res.writeHead(405); res.end(); return; }
      let ready = Boolean(isWebReady());
      if (req.url === '/health/ready' && production) {
        ready &&= typeof verifySession === 'function' && typeof readProducts === 'function' && typeof adapters.checkReady === 'function';
        if (ready) {
          const controller = new AbortController(); let timer;
          try { ready = await Promise.race([Promise.resolve().then(() => adapters.checkReady({ signal: controller.signal })), new Promise(resolve => { timer = setTimeout(() => { controller.abort(); resolve(false); }, 2000); })]) === true; }
          catch { ready = false; } finally { clearTimeout(timer); }
        }
      }
      const status = req.url === '/health/live' ? 200 : ready ? 200 : 503;
      res.writeHead(status, { 'Content-Type': 'application/json' });
      res.end(req.method === 'HEAD' ? undefined : JSON.stringify({ status: status === 200 ? 'ok' : 'not_ready' })); return;
    }
    if (req.headers.host !== expected) { res.writeHead(403); res.end(); return; }
    if (!req.url || req.url.length > 8192) { res.writeHead(400); res.end(); return; }
    const pathname = req.url.split('?')[0];
    const existingResource = /^\/v1\/(?:(?:products|orders|catalogs|collections)(?:\/[^/]+)?|settings\/location(?:\/[^/]+)?|settings\/shipping(?:\/profile\/[^/]+)?)$/.test(pathname);
    if ((req.url === '/api/session' || pathname === '/api/products' || existingResource) && req.method === 'POST') {
      const reply = (status, data) => { res.writeHead(status, { 'Content-Type': 'application/json' }); res.end(JSON.stringify(data)); };
      if (!verifySession) { reply(503, { error: 'backend_identity_verifier_not_configured' }); return; }
      if (pathname === '/api/products' && !readProducts) { reply(503, { error: 'product_reader_not_configured' }); return; }
      if (existingResource && !readResource) { reply(503, {error:'resource_reader_not_configured'}); return; }
      let query;
      try { const search = new URL(req.url, 'http://local.invalid').searchParams;
        query = existingResource ? resourceQuery(pathname,search) : productQuery(search); }
      catch { reply(400, { error: 'invalid_query' }); return; }
      if (req.headers.origin !== (appOrigin || `http://${expected}`) || req.headers['transfer-encoding'] || (req.headers['content-length'] && req.headers['content-length'] !== '0')) { reply(403, { error: 'access_denied' }); return; }
      const bearer = req.headers.authorization;
      if (!bearer?.startsWith('Bearer ') || bearer.length > 4103) { reply(401, { error: 'invalid_identity' }); return; }
      if (req.headers.cookie || ['authorization', 'origin'].some(key => req.rawHeaders.filter((v, i) => i % 2 === 0 && v.toLowerCase() === key).length !== 1)) { reply(403, { error: 'access_denied' }); return; }
      const controller = new AbortController(); let timer;
      try {
        const identity = await Promise.race([
          Promise.resolve().then(() => verifySession(bearer.slice(7), { signal: controller.signal })),
          new Promise((_, reject) => { timer = setTimeout(() => { controller.abort(); reject(Error('timeout')); }, 5000); }),
        ]);
        const fields = ['merchantId', 'actorId', 'appId', 'installationId'];
        if (!identity || !fields.every(k => typeof identity[k] === 'string' && /^[\x21-\x7e]{1,256}$/.test(identity[k])) || !Number.isSafeInteger(identity.expiresAt) || identity.expiresAt <= Date.now() / 1000) throw Error('invalid result');
        if (pathname === '/api/products' || existingResource) {
          const data = await Promise.race([
            Promise.resolve().then(() => existingResource ? readResource(bearer.slice(7),{path:pathname,signal:controller.signal,query}) : readProducts(bearer.slice(7), { signal: controller.signal, query })),
            new Promise((_, reject) => { clearTimeout(timer); timer = setTimeout(() => { controller.abort(); reject(Error('timeout')); }, 8000); }),
          ]);
          reply(200, existingResource ? resourcePage(pathname,data,query.limit,query) : productPage(data, query.limit));
        } else reply(200, { status: 'connected', ...Object.fromEntries(fields.map(k => [k, identity[k]])), expiresAt: identity.expiresAt });
      } catch (e) {
        const code = e?.code === 'invalid_identity' ? 401 : e?.code === 'access_denied' ? 403 : 503;
        reply(code, { error: code === 401 ? 'invalid_identity' : code === 403 ? 'access_denied' : 'backend_identity_verifier_unavailable' });
      } finally { clearTimeout(timer); }
      return;
    }

    if (pathname.startsWith('/api/') || pathname.startsWith('/v1/')) { res.writeHead(404); res.end(); return; }
    if (!['GET', 'HEAD'].includes(req.method)) { res.writeHead(405); res.end(); return; }
    if (proof && req.url === `/.well-known/emisell-app-verification/${proof.id}`) {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(req.method === 'HEAD' ? undefined : JSON.stringify(proof.document)); return;
    }
    if (req.url === '/dev-config.json') {
      if (production) { res.writeHead(404); res.end(); return; }
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(req.method === 'HEAD' ? undefined : JSON.stringify({ parentOrigin: parent })); return;
    }
    // Never expose backend, dotfiles, secrets, config or arbitrary filesystem paths.
    let decoded;
    try { decoded = decodeURIComponent(pathname); } catch { res.writeHead(400); res.end(); return; }
    // Vite's optimized frontend dependencies are the only served hidden directory.
    const publicPath = decoded.replace(/^\/node_modules\/\.vite\/deps\//, '/node_modules/vite-deps/');
    if (production && /^\/(?:app|public|node_modules|@[^/]*)(?:\/|$)/.test(decoded)) { res.writeHead(404); res.end(); return; }
    if (decoded.startsWith('/@id/') && !/^\/@id\/__x00__virtual:react-router\/(inject-hmr-runtime|hmr-runtime|browser-manifest)$/.test(decoded)) {
      res.writeHead(404); res.end(); return;
    }
    if (decoded.includes('..') || decoded.includes('\\') || decoded.includes('%') || decoded.includes('\0') ||
        /(?:^|\/)(?:\.[^/]+|server|private)(?:\/|$)/.test(publicPath) ||
        /\.(?:server\.[^/]+|secret|pem|key)(?:$|\/)/.test(decoded) ||
        /^\/(?:emisell\.app\.json|package(?:-lock)?\.json|README\.md|TESTING\.md|[^/]*backend\.mjs|@fs)(?:$|\/)/.test(decoded)) {
      res.writeHead(404); res.end(); return;
    }
    return handler(req, res);
  }
  server.requestTimeout = 10000; server.headersTimeout = 10000;
  const upgrades = new Set();
  server.prependListener('upgrade', (req, socket) => {
    upgrades.add(socket);
    socket.once('close', () => upgrades.delete(socket));
    const expected = `127.0.0.1:${server.address()?.port}`;
    if (production || req.headers.host !== expected || req.headers.origin !== (appOrigin || `http://${expected}`)) socket.destroy();
  });
  // http.closeAllConnections excludes upgraded sockets. Close HMR clients too
  // so Ctrl+C does not wait forever for an open browser tab.
  const close = server.close.bind(server);
  server.close = callback => { for (const socket of upgrades) socket.destroy(); return close(callback); };
  await new Promise((resolve, reject) => {
    server.once('error', error => reject(Error(error.code === 'EADDRINUSE'
      ? `Port ${port} sedang digunakan. Pilih --port lain; proses existing tidak dihentikan.`
      : 'Preview gagal dijalankan. Periksa port dan izin jaringan lokal.')));
    server.listen(port, host, resolve);
  });
  return server;
}
