import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { initApp, startDev } from './development.mjs';
import { initOptions, projectPath } from './init.mjs';
import { findProject, inspectProject, readProject } from './project.mjs';

const optionsByCommand = {
  init: ['dir', 'path', 'name', 'parent-origin', 'template'],
  dev: ['dir', 'path', 'port', 'backend'],
  info: ['dir', 'path', 'json'],
  doctor: ['dir', 'path', 'json'],
};

export async function runLocal(command, flags, { output = console.log, cwd = process.cwd(), interactive, prompt, startPreview = startDev } = {}) {
  if (Object.keys(flags).some(key => !optionsByCommand[command]?.includes(key))) throw Error('Opsi tidak dikenal. Lihat emisell app ' + command + ' --help.');
  if (command === 'init') {
    const config = await initOptions(flags, { cwd, interactive, prompt });
    let target;
    try { target = await initApp(config.directory, config.parentOrigin, config.template, config.name); }
    catch (error) {
      if (error.code === 'EEXIST') throw Error('EEXIST: folder tujuan sudah ada. Pilih --path baru; file existing tidak ditimpa.');
      throw error;
    }
    output(`Project ${config.name} dibuat: ${target}\nTemplate: ${config.template}\n\nBerikutnya:\n  Masuk ke folder project tersebut.\n  emisell app doctor\n  emisell app dev\n\nBelum terhubung ke toko. Gunakan app dev --help untuk konfigurasi backend.`);
    return 0;
  }
  const path = projectPath(flags);
  let directory;
  try { directory = await findProject(resolve(cwd, path ?? '.'), path !== undefined); }
  catch (error) {
    if (command !== 'doctor') throw error;
    const report = { readyForLocalPreview: false, storeAccess: 'not-checked', checks: [{ id: 'project', status: 'error', message: error.message }] };
    output(flags.json ? JSON.stringify(report, null, 2) : `ERROR: ${error.message}`); return 1;
  }
  if (command === 'doctor') {
    const report = await inspectProject(directory);
    output(flags.json ? JSON.stringify(report, null, 2) : [
      'Pemeriksaan project lokal', ...report.checks.map(check => `${check.status.toUpperCase()}: ${check.message}`),
      report.readyForLocalPreview ? 'Preview lokal siap. Akses toko belum diverifikasi.' : 'Perbaiki kesalahan di atas sebelum menjalankan preview.',
    ].join('\n'));
    return report.readyForLocalPreview ? 0 : 1;
  }
  const config = await readProject(directory);
  if (command === 'info') {
    const info = { name: config.name, directory, template: config.template, parentOrigin: config.parentOrigin,
      appOrigin: config.appOrigin, uiKitVersion: config.uiKitVersion, endpointProofConfigured: Boolean(config.endpointProof),
      templateScopes: config.template === 'products' ? ['read_products'] : [], storeAccess: 'not-checked' };
    output(flags.json ? JSON.stringify(info, null, 2) : [
      `Aplikasi: ${info.name}`, `Project: ${directory}`, `Template: ${info.template}`, `Dashboard seller: ${info.parentOrigin}`,
      `Origin aplikasi: ${info.appOrigin ?? 'Preview loopback (HTTPS belum dikonfigurasi)'}`,
      `Kebutuhan template: ${info.templateScopes.join(', ') || 'Tidak ada akses data bawaan'}`,
      'Kebutuhan template bukan izin seller. Koneksi server dan akses toko belum diverifikasi.',
    ].join('\n'));
    return 0;
  }
  const port = flags.port || '4330';
  if (!/^[0-9]+$/.test(port) || Number(port) < 1024 || Number(port) > 65535) throw Error('Port harus 1024–65535.');
  let verifySession, readProducts;
  if (flags.backend) {
    // Explicit opt-in only. Preserve 0.2.0 paths relative to the invoking terminal.
    const module = await import(pathToFileURL(resolve(cwd, flags.backend)));
    if (typeof module.verifySession !== 'function') throw Error('Backend harus mengekspor verifySession.');
    verifySession = module.verifySession;
    if (module.readProducts !== undefined && typeof module.readProducts !== 'function') throw Error('readProducts harus berupa fungsi.');
    readProducts = module.readProducts;
  }
  const server = await startPreview(directory, Number(port), { verifySession, readProducts });
  output(`Preview: http://127.0.0.1:${server.address().port}\nProject: ${directory}\nBackend: ${verifySession ? 'Modul dimuat; akses toko tetap diperiksa per permintaan.' : 'Belum dikonfigurasi. UI preview saja; akses data ditolak.'}\nRefresh setelah edit. Ctrl+C untuk berhenti. Ini bukan server produksi atau instalasi seller.`);
  await new Promise(resolve => {
    const stop = () => { server.closeAllConnections(); server.close(resolve); };
    process.once('SIGINT', stop); process.once('SIGTERM', stop);
    server.once('close', () => { process.removeListener('SIGINT', stop); process.removeListener('SIGTERM', stop); resolve(); });
  });
  return 0;
}
