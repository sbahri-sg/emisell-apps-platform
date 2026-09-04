const privateHeaders = {
  'Cache-Control': 'private, no-store, max-age=0',
  'Vary': 'Cookie, Authorization',
  'X-Content-Type-Options': 'nosniff',
  'Referrer-Policy': 'no-referrer',
};

function failure(status, code, message) {
  return Response.json({ error: { code, message } }, { status, headers: privateHeaders });
}

// The gateway, not a client-side role or decoded JWT, verifies the operator.
// No fallback token is supplied here, including in local development.
export async function serveDocumentation(request, { gatewayUrl, sessionCookieName = 'emisell_admin_session', fetcher = (input, init) => fetch(input, init), render }) {
  const incoming = request.headers;
  const headers = new Headers({ Accept: 'application/json' });
  const session = (incoming.get('Cookie') ?? '').split(';').map((part) => part.trim())
    .find((part) => part.startsWith(`${sessionCookieName}=`));
  if (session) headers.set('Cookie', session);
  if (!session) return failure(401, 'unauthorized', 'Sign in to access internal documentation.');
  try {
    const origin = new URL(gatewayUrl);
    if (!['http:', 'https:'].includes(origin.protocol) || origin.username || origin.password) throw new Error('Invalid gateway configuration');
    // workerd supports manual/follow, not redirect:error. Never follow a
    // redirect with forwarded credentials; every 3xx fails the !ok check.
    const identity = await fetcher(new URL('/auth/admin/session', origin), { headers, cache: 'no-store', redirect: 'manual', signal: AbortSignal.timeout(8000) });
    if (identity.status === 401 || identity.status === 403) return failure(identity.status, 'access_denied', 'A valid platform-operator session is required.');
    if (!identity.ok) return failure(502, 'identity_unavailable', 'Identity verification is temporarily unavailable.');
    const payload = await identity.json();
    if (payload?.data?.platformOperator !== true) return failure(403, 'forbidden', 'Internal documentation is restricted to Emisell platform operators.');
  } catch {
    return failure(502, 'identity_unavailable', 'Identity verification is temporarily unavailable.');
  }
  try {
    const response = await render();
    for (const [key, value] of Object.entries(privateHeaders)) response.headers.set(key, value);
    return response;
  } catch {
    return failure(500, 'documentation_unavailable', 'The documentation contract is temporarily unavailable.');
  }
}
