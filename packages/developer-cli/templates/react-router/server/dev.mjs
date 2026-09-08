import { createRequire } from 'node:module';
import { readFile, realpath } from 'node:fs/promises';
import { join, resolve, relative, isAbsolute, sep } from 'node:path';
import { pathToFileURL } from 'node:url';
import { startBoundary } from './http.mjs';
import { createBackend } from './backend.mjs';
import { readProject, supportedNode } from './project.mjs';
import { runtimeOptions, deploymentBackend } from './runtime.mjs';

export async function start({ root = process.cwd(), port, config, adapters, built = false, containerPreview = false, env = process.env } = {}) {
  if (!supportedNode()) throw Error('Gunakan Node.js 22.12 atau lebih baru.');
  root = await realpath(root);
  config ??= await readProject(root);
  const runtime = runtimeOptions(config, { env, port, built, containerPreview });
  config = runtime.config;
  const require = createRequire(join(root, 'package.json'));
  async function dependency(name) {
    try { return await import(pathToFileURL(require.resolve(name))); }
    catch { throw Error('Dependency framework belum siap. Jalankan npm install di folder project.'); }
  }
  const [{ default: express }, { createRequestHandler }] = await Promise.all([dependency('express'), dependency('@react-router/express')]);
  const app = express(); app.disable('x-powered-by');
  let vite, server;
  if (runtime.mode !== 'development' && adapters) throw Error('Adapter test/dev tidak boleh disisipkan ke runtime deployment.');
  const backend = runtime.production ? await deploymentBackend(root, env, config) : containerPreview ? {} :
    typeof adapters?.verifySession === 'function' ? adapters : await createBackend(root, env);
  // Prevent symlink escapes before Vite/static middleware reads a public source.
  app.use(async (req, res, next) => {
    try {
      const pathname = decodeURIComponent(req.url.split('?')[0]);
      // Vite serves public/foo at /foo. Reject symlinks there as well as in app/.
      const publicFile = resolve(root, built ? 'build/client' : 'public', '.' + pathname);
      try {
        if (await realpath(publicFile) !== publicFile) { res.sendStatus(404); return; }
      } catch (error) { if (error.code !== 'ENOENT' && error.code !== 'ENOTDIR') { res.sendStatus(404); return; } }
      if (/^\/(?:app|public)\//.test(pathname)) {
        const actual = await realpath(join(root, pathname));
        const inside = relative(root, actual);
        if (actual !== resolve(root, '.' + pathname) || inside.startsWith('..' + sep) || isAbsolute(inside) || !/^(app|public)[/\\]/.test(inside)) { res.sendStatus(404); return; }
      }
      next();
    } catch { res.sendStatus(404); }
  });
  try {
    server = await startBoundary({ config, port: runtime.port, host: runtime.host, production: runtime.production, adapters: backend, handler: app, isWebReady: () => Boolean(server?.frameworkReady) });
    if (!built) {
      const { createServer } = await dependency('vite');
      vite = await createServer({ root, server: { middlewareMode: true, hmr: { server, ...(config.appOrigin ? { protocol: 'wss', host: new URL(config.appOrigin).hostname, clientPort: 443 } : {}) } }, appType: 'custom' });
      app.use(vite.middlewares);
    } else {
      await readFile(join(root, 'build/server/index.js'));
      app.use('/assets', express.static(join(root, 'build/client/assets'), { fallthrough: false, index: false, dotfiles: 'deny' }));
      app.get('/favicon.svg', (_req, res) => res.sendFile('favicon.svg', { root: join(root, 'build/client') }));
    }
    app.use(createRequestHandler({
      build: vite ? () => vite.ssrLoadModule('virtual:react-router/server-build') : await import(pathToFileURL(join(root, 'build/server/index.js'))),
      mode: built ? 'production' : 'development',
      getLoadContext: (_req, res) => ({ nonce: res.locals.nonce, publicConfig: { name: config.name, parentOrigin: config.parentOrigin, runtime: runtime.mode } }),
    }));
    app.use((_error, _req, res, _next) => { res.status(500).json({ error: 'framework_unavailable' }); });
    server.once('close', () => { void vite?.close(); });
    server.backendConfigured = Boolean(backend.verifySession);
    server.frameworkReady = true;
    server.runtime = runtime.mode;
    return server;
  } catch (error) {
    await vite?.close();
    server?.closeAllConnections(); server?.close();
    throw error;
  }
}
