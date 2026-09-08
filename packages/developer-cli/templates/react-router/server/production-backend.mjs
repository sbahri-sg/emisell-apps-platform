// Server-only integration seam, not a mock authorization implementation.
// Keep this fail-closed until the operator supplies a supported production API.
// Never reuse the loopback reviewed-UI endpoints or return token claims as proof.
export async function createBackend({ env, config }) {
  void env; void config;
  // Implement verifySession(token, { signal }), readProducts(token, { signal,
  // query }), readResource(token, { path, signal, query }) for orders/shipping,
  // and checkReady({ signal }). See DEPLOYMENT.md for the contract.
  throw Error('Production Core authorization/resource adapter is not configured.');
}
