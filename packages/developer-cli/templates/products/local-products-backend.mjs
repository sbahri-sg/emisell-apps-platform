import { fileURLToPath } from 'node:url';
import { createLocalCoreVerifier } from './server/local-core.mjs';
import { createLocalProductReader } from './server/local-products.mjs';

const config = {
  coreOrigin: process.env.EMISELL_LOCAL_CORE_ORIGIN,
  appId: process.env.EMISELL_LOCAL_APP_ID,
  clientId: process.env.EMISELL_LOCAL_CLIENT_ID,
};
export const verifySession = createLocalCoreVerifier(config);
export const readProducts = createLocalProductReader({ ...config,
  secretFile: process.env.EMISELL_LOCAL_CLIENT_SECRET_FILE,
  publicDirectory: fileURLToPath(new URL('./public/', import.meta.url)),
});
