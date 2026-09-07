import { createIdentityVerifier } from './server/identity.mjs';

// Fail closed until trusted server configuration and real authorization adapters exist.
// Do not copy these values from a browser token or emisell.app.json.
// Never place trust configuration, provider keys or server code under public/.
export const verifySession = createIdentityVerifier({
  keys: {}, // { trustedKeyId: publicEd25519SPKIPEM }, provisioned by your operator.
  issuer: '', // Exact configured Platform issuer.
  audience: '', // Your reviewed app-client ID, not a user-selected ID.
  resolveExpected: undefined, // Independent backend binding lookup. See README.
  authorizeCurrent: undefined, // Live checks; unavailable or revoked must deny.
});
