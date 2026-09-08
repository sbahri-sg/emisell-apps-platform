import { createInterface } from 'node:readline/promises';
import { basename, resolve } from 'node:path';
import { origin } from './client.mjs';
import { appName, templates } from './project.mjs';

// No prompts in pipelines or CI. EOF / Ctrl+C cancels before creating files.
export async function terminalPrompt({ message, defaultValue, choices }, { input = process.stdin, output = process.stderr } = {}) {
  const rl = createInterface({ input, output, terminal: true });
  const controller = new AbortController();
  const cancel = () => controller.abort();
  rl.once('SIGINT', cancel);
  rl.once('close', cancel);
  try {
    if (choices) output.write(choices.map((choice, i) => `  ${i + 1}. ${choice}\n`).join(''));
    const answer = (await rl.question(`${message}${defaultValue ? ` [${defaultValue}]` : ''}: `, { signal: controller.signal })).trim();
    return answer || defaultValue || '';
  } catch { throw Error('Pembuatan project dibatalkan. Tidak ada project yang dibuat.'); }
  finally { rl.removeListener('close', cancel); rl.close(); }
}

export function projectPath(flags) {
  if (flags.path !== undefined && flags.dir !== undefined) throw Error('Gunakan salah satu --path atau --dir, bukan keduanya.');
  const path = flags.path ?? flags.dir;
  if (path !== undefined && (typeof path !== 'string' || !path.trim() || /\p{C}/u.test(path))) throw Error('Folder project tidak valid.');
  return path;
}

export async function initOptions(flags, { cwd = process.cwd(), interactive = Boolean(process.stdin.isTTY && process.stderr.isTTY && !process.env.CI), prompt = terminalPrompt } = {}) {
  let path = projectPath(flags), name = flags.name, template = flags.template, parentOrigin = flags['parent-origin'];
  if (!interactive && ((!path && !name) || !parentOrigin)) {
    throw Error('Mode non-interaktif: isi --name atau --path serta --parent-origin. Contoh: emisell app init --name my-app --parent-origin http://localhost:3000');
  }
  if (name === undefined) name = path ? basename(resolve(cwd, path)) : await prompt({ message: 'Nama aplikasi', defaultValue: 'my-emisell-app' });
  name = appName(name);
  if (!path) {
    const folder = name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'my-emisell-app';
    path = interactive ? await prompt({ message: 'Folder project baru', defaultValue: folder }) : folder;
  }
  if (typeof path !== 'string' || !path.trim() || /\p{C}/u.test(path)) throw Error('Folder project tidak valid.');
  template ??= 'react-router';
  if (!templates.includes(template)) throw Error('Template harus react-router. Template embedded/products sudah dihapus.');
  if (!parentOrigin) parentOrigin = await prompt({ message: 'Origin Dashboard seller yang dipercaya', defaultValue: 'http://localhost:3000' });
  parentOrigin = origin(parentOrigin);
  // All input is validated before initApp can write anything.
  return { directory: resolve(cwd, path), name, template, parentOrigin };
}
