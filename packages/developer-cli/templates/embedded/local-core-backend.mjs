import { createLocalCoreVerifier } from './server/local-core.mjs';

// Explicit server environment, not query parameters or unverified token claims.
export const verifySession = createLocalCoreVerifier({
  coreOrigin: process.env.EMISELL_LOCAL_CORE_ORIGIN,
  appId: process.env.EMISELL_LOCAL_APP_ID,
  clientId: process.env.EMISELL_LOCAL_CLIENT_ID,
});
