import { mkdir, readFile, writeFile, readdir, lstat } from 'node:fs/promises';
import { basename, resolve, join } from 'node:path';
import { pathToFileURL } from 'node:url';
import { origin } from './client.mjs';
import { appName, readProject, supportedNode } from './project.mjs';

export async function initApp(directory, parentOrigin, template = 'react-router', name = basename(resolve(directory))) {
  if (template !== 'react-router') throw Error('Template harus react-router. Template embedded/products sudah dihapus; gunakan project baru.');
  parentOrigin = origin(parentOrigin);
  name = appName(name);
  const target = resolve(directory);
  await mkdir(target, { mode: 0o700 }); // Never overwrite even an empty directory.
  async function copy(source, destination) {
    for (const entry of await readdir(source, { withFileTypes: true })) {
      if (['node_modules', 'package-lock.json', 'build', '.react-router'].includes(entry.name) ||
          (entry.name.startsWith('.') && !['.env.example', '.env.production.example', '.dockerignore'].includes(entry.name))) continue;
      if (entry.isSymbolicLink()) throw Error('Template tidak boleh mengandung symlink.');
      const src = new URL(entry.name + (entry.isDirectory() ? '/' : ''), source);
      const dest = join(destination, entry.name);
      if (entry.isDirectory()) { await mkdir(dest); await copy(src, dest); }
      else if (entry.isFile()) await writeFile(dest, await readFile(src), { flag: 'wx', mode: 0o600 });
    }
  }
  await copy(new URL('../templates/react-router/', import.meta.url), target);
  const packageFile = join(target, 'package.json');
  const pkg = JSON.parse(await readFile(packageFile, 'utf8'));
  pkg.name = name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'emisell-app';
  await writeFile(packageFile, JSON.stringify(pkg, null, 2) + '\n');
  await writeFile(join(target, '.gitignore'), 'node_modules/\nbuild/\n.react-router/\n.env\n.env.*\n!.env.example\n!.env.production.example\n.local/\nprivate/\nsecrets/\n*.secret\n*.pem\n*.key\n*.log\n', { flag: 'wx' });
  await writeFile(join(target, 'emisell.app.json'), JSON.stringify({
    schema: 'emisell.react-router-app/v1', name, template, parentOrigin, uiKitVersion: '0.1.0',
  }, null, 2) + '\n', { flag: 'wx' });
  return target;
}

export async function startDev(directory, port = 4330, adapters = {}) {
  const root = resolve(directory);
  const config = await readProject(root);
  if (!supportedNode()) throw Error('Template memerlukan Node.js 22.12 atau lebih baru.');
  if (!Number.isInteger(port) || port < 0 || port > 65535) throw Error('Port tidak valid.');
  const entry = join(root, 'server/dev.mjs');
  const stat = await lstat(entry);
  if (!stat.isFile() || stat.isSymbolicLink()) throw Error('server/dev.mjs harus file biasa dari project tepercaya.');
  // Framework development executes project code, like npm run dev; doctor never does.
  const { start } = await import(pathToFileURL(entry));
  return start({ root, port, config, adapters });
}
