import { fileURLToPath } from 'node:url';
import { start } from './dev.mjs';

const args = process.argv.slice(2);
if (args.some(arg => !['--built', '--container-preview'].includes(arg))) throw Error('Gunakan npm run dev, npm start atau npm run docker:preview. PORT memilih port server.');
try {
  const server = await start({ root: fileURLToPath(new URL('../', import.meta.url)), built: args.includes('--built'), containerPreview: args.includes('--container-preview') });
  console.log(`Emisell ${server.runtime}: port ${server.address().port}. Backend ${server.backendConfigured ? 'configured; access checked per request' : 'disabled; no store access'}.`);
  let stopping = false;
  for (const signal of ['SIGINT', 'SIGTERM']) process.once(signal, () => {
    if (stopping) return; stopping = true; server.frameworkReady = false;
    // Stop accepting requests, allow bounded in-flight operations to finish.
    const deadline = setTimeout(() => { server.closeAllConnections(); process.exit(1); }, 10000);
    deadline.unref();
    server.close(() => clearTimeout(deadline));
  });
} catch (error) { console.error(error.message); process.exitCode = 1; }
