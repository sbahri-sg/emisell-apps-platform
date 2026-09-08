import { mkdir, readFile, writeFile, lstat } from 'node:fs/promises';
import { basename, resolve, join } from 'node:path';
import { createServer } from 'node:http';
import { origin } from './client.mjs';
import { productQuery, productPage } from '../templates/products/server/product-query.mjs';
import { appName, publicAssets, readProject, starterAssets } from './project.mjs';

const files = starterAssets;
const types = { '.html': 'text/html', '.mjs': 'text/javascript', '.css': 'text/css' };
export async function initApp(directory, parentOrigin, template = 'embedded', name = basename(resolve(directory))) {
  if (!['embedded', 'products'].includes(template)) throw Error('Template harus embedded atau products.');
  parentOrigin = origin(parentOrigin);
  name = appName(name);
  const target = resolve(directory);
  // Never merge into or overwrite an existing project, even an empty directory.
  await mkdir(target, { mode: 0o700 });
  await mkdir(join(target, 'public'));
  await mkdir(join(target, 'server'));
  for (const name of ['backend.mjs', 'local-core-backend.mjs', 'server/identity.mjs', 'server/local-core.mjs', 'server/README.md']) {
    await writeFile(join(target, name), await readFile(new URL(`../templates/embedded/${name}`, import.meta.url)), { flag: 'wx' });
  }
  for (const name of files) {
    const source = template === 'products' && ['index.html', 'app.mjs'].includes(name) ? 'products' : 'embedded';
    const content = await readFile(new URL(`../templates/${source}/public/${name}`, import.meta.url));
    await writeFile(join(target, 'public', name), content, { flag: 'wx' });
  }
  if (template === 'products') {
    for (const name of ['local-products-backend.mjs', 'server/local-products.mjs', 'server/product-query.mjs', 'public/products.css']) {
      await writeFile(join(target, name), await readFile(new URL(`../templates/products/${name}`, import.meta.url)), { flag: 'wx' });
    }
  }
  await writeFile(join(target, '.gitignore'), '.env\n.env.*\n.local/\n*.secret\n', { flag: 'wx' });
  await writeFile(join(target, 'emisell.app.json'), JSON.stringify({ schema: 'emisell.local-app/v1', name, template, parentOrigin, uiKitVersion: '0.1.0' }, null, 2) + '\n', { flag: 'wx' });
  await writeFile(join(target, 'README.md'), `# Embedded Starter\n\nRun: emisell app dev --dir .\n\nEdit public/index.html and public/app.mjs, then refresh the browser.\nUI Kit 0.1.0 and Bridge are bundled snapshots; no npm dependencies.\n\nThis is a loopback-only preview, not a production server. /api/session defaults to 503. Use --backend ./my-app/backend.mjs only after configuring trusted adapters; see server/README.md.\nImplement server-side identity verification and current authorization before using seller data.\nDo not put secrets in public/. Only four starter assets are served by the preview.\n\nFor seller Testing, host on HTTPS, obtain a valid reviewed UI release and request an assignment\nwith emisell testing request. This does not install the app or grant merchant access.\n`, { flag: 'wx' });
  if (template === 'products') await writeFile(join(target, 'README.md'), await readFile(new URL('../templates/products/README.md', import.meta.url)));
  const guide = await readFile(join(target, 'README.md'), 'utf8');
  await writeFile(join(target, 'README.md'), `## Mulai dari folder project (CLI 0.3+)\n\n\`\`\`sh\nemisell app info\nemisell app doctor\nemisell app dev\n\`\`\`\n\nCLI menemukan emisell.app.json dari folder ini atau subfoldernya. --path memilih project secara eksplisit; --dir tetap didukung. Doctor hanya memeriksa konfigurasi/aset lokal, bukan izin toko. Akses data tetap membutuhkan --backend dan konfigurasi privat yang dijelaskan di bawah.\n\n${guide}`);
  return target;
}

export async function startDev(directory, port = 4330, { verifySession, readProducts } = {}) {
  if (!Number.isInteger(port) || port < 0 || port > 65535) throw Error('Port tidak valid.');
  const root = resolve(directory);
  const config = await readProject(root);
  const parent = config.parentOrigin;
  // Explicit local operator configuration; never trust forwarded request headers.
  const appOrigin = config.appOrigin;
  const proof = config.endpointProof;
  const publicDir = join(root, 'public');
  const servedFiles = publicAssets(config);
  if (!(await lstat(publicDir)).isDirectory() || (await lstat(publicDir)).isSymbolicLink()) throw Error('Direktori public tidak valid.');
  const server = createServer(async (req, res) => {
    res.setHeader('Cache-Control', 'no-store');
    res.setHeader('X-Content-Type-Options', 'nosniff');
    res.setHeader('Referrer-Policy', 'no-referrer');
    res.setHeader('Content-Security-Policy', `default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; frame-ancestors ${parent}; base-uri 'none'; form-action 'none'`);
    const expected = `127.0.0.1:${server.address().port}`;
    if (req.headers.host !== expected) { res.writeHead(403); res.end(); return; }
    const pathname = req.url?.split('?')[0];
    if ((req.url === '/api/session' || pathname === '/api/products') && req.method === 'POST') {
      const reply = (status, data) => { res.writeHead(status, { 'Content-Type': 'application/json' }); res.end(JSON.stringify(data)); };
      if (!verifySession) { reply(503, { error: 'backend_identity_verifier_not_configured' }); return; }
      if (pathname === '/api/products' && !readProducts) { reply(503, { error: 'product_reader_not_configured' }); return; }
      let query;
      try { query = productQuery(new URL(req.url, 'http://local.invalid').searchParams); }
      catch { reply(400, { error: 'invalid_query' }); return; }
      if (req.headers.origin !== (appOrigin || `http://${expected}`) || req.headers['transfer-encoding'] || (req.headers['content-length'] && req.headers['content-length'] !== '0')) { reply(403, { error: 'access_denied' }); return; }
      const bearer = req.headers.authorization;
      if (!bearer?.startsWith('Bearer ') || bearer.length > 4103) { reply(401, { error: 'invalid_identity' }); return; }
      const controller = new AbortController(); let timer;
      try {
        const identity = await Promise.race([
          Promise.resolve().then(() => verifySession(bearer.slice(7), { signal: controller.signal })),
          new Promise((_, reject) => { timer = setTimeout(() => { controller.abort(); reject(Error('timeout')); }, 5000); }),
        ]);
        const fields = ['merchantId', 'actorId', 'appId', 'installationId'];
        if (!identity || !fields.every(k => typeof identity[k] === 'string' && /^[\x21-\x7e]{1,256}$/.test(identity[k])) || !Number.isSafeInteger(identity.expiresAt) || identity.expiresAt <= Date.now() / 1000) throw Error('invalid result');
        if (pathname === '/api/products') {
          const data = await Promise.race([
            Promise.resolve().then(() => readProducts(bearer.slice(7), { signal: controller.signal, query })),
            new Promise((_, reject) => { clearTimeout(timer); timer = setTimeout(() => { controller.abort(); reject(Error('timeout')); }, 8000); }),
          ]);
          reply(200, productPage(data, query.limit));
        } else reply(200, { status: 'connected', ...Object.fromEntries(fields.map(k => [k, identity[k]])), expiresAt: identity.expiresAt });
      } catch (e) {
        const code = e?.code === 'invalid_identity' ? 401 : e?.code === 'access_denied' ? 403 : 503;
        reply(code, { error: code === 401 ? 'invalid_identity' : code === 403 ? 'access_denied' : 'backend_identity_verifier_unavailable' });
      } finally { clearTimeout(timer); }
      return;
    }
    if (!['GET', 'HEAD'].includes(req.method)) { res.writeHead(405); res.end(); return; }
    if (proof && req.url === `/.well-known/emisell-app-verification/${proof.id}`) {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(req.method === 'HEAD' ? undefined : JSON.stringify(proof.document)); return;
    }
    if (req.url === '/dev-config.json') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(req.method === 'HEAD' ? undefined : JSON.stringify({ parentOrigin: parent })); return;
    }
    const name = req.url === '/' ? 'index.html' : req.url?.slice(1);
    if (!servedFiles.includes(name)) { res.writeHead(404); res.end(); return; }
    try {
      const path = join(publicDir, name);
      if (!(await lstat(path)).isFile() || (await lstat(path)).isSymbolicLink()) throw Error('invalid file');
      const data = await readFile(path);
      res.writeHead(200, { 'Content-Type': `${types[name.slice(name.lastIndexOf('.'))]}; charset=utf-8` });
      res.end(req.method === 'HEAD' ? undefined : data);
    } catch { res.writeHead(404); res.end(); }
  });
  server.requestTimeout = 10000; server.headersTimeout = 10000;
  await new Promise((resolve, reject) => {
    const failed = error => reject(Error(error.code === 'EADDRINUSE'
      ? `Port ${port} sedang digunakan. Pilih --port lain; CLI tidak menghentikan proses yang sedang berjalan.`
      : 'Preview gagal dijalankan. Periksa port dan izin jaringan lokal.'));
    server.once('error', failed);
    server.listen(port, '127.0.0.1', () => { server.removeListener('error', failed); resolve(); });
  });
  return server;
}
