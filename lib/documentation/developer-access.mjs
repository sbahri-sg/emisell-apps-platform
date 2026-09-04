const privateHeaders = {
  'Cache-Control': 'private, no-store, max-age=0',
  Vary: 'Cookie, Authorization, X-Organization-Id',
  'X-Content-Type-Options': 'nosniff',
  'Referrer-Policy': 'no-referrer',
};
const failure = (status, code, message) => Response.json({ error: { code, message } }, { status, headers: privateHeaders });

// Reuse the gateway's developer authentication. Do not accept Admin/Merchant
// cookies, forwarded role/identity headers, decoded claims, or fallback tokens.
export async function serveDeveloperDocumentation(request, { gatewayUrl, sessionCookieName = 'emisell_session', fetcher = (url, init) => fetch(url, init), render }) {
  const headers = new Headers({ Accept: 'application/json' });
  const cookies = (request.headers.get('Cookie') ?? '').split(';').map((part) => part.trim()).filter((part) => part.startsWith(`${sessionCookieName}=`));
  if (cookies.length > 1) return failure(401, 'unauthorized', 'Sign in with one valid developer session.');
  if (cookies[0]) headers.set('Cookie', cookies[0]);
  else {
    const authorization = request.headers.get('Authorization');
    if (!authorization?.match(/^Bearer [^\s,]+$/)) return failure(401, 'unauthorized', 'Sign in to the Developer Console to read these guides.');
    headers.set('Authorization', authorization);
    // The gateway validates a bearer token's organization binding. This header
    // is context only and is never used locally as proof of authorization.
    const organization = request.headers.get('X-Organization-Id');
    if (organization) headers.set('X-Organization-Id', organization);
  }
  try {
    const origin = new URL(gatewayUrl);
    if (!['http:', 'https:'].includes(origin.protocol) || origin.username || origin.password) throw new Error('Invalid gateway');
    const identity = await fetcher(new URL('/v1/session', origin), { headers, cache: 'no-store', redirect: 'manual', signal: AbortSignal.timeout(8000) });
    if (identity.status === 401 || identity.status === 403) return failure(identity.status, 'access_denied', 'A valid developer identity and active workspace are required.');
    if (!identity.ok) return failure(502, 'identity_unavailable', 'Developer identity verification is temporarily unavailable.');
    const actor = (await identity.json())?.data;
    if (!actor?.userId || !actor.organizationId || actor.activeOrganization?.organizationId !== actor.organizationId || !['owner', 'admin', 'developer', 'analyst'].includes(actor.role) || !['session', 'bearer'].includes(actor.authenticationMethod)) {
      return failure(403, 'workspace_required', 'Accept your invitation and choose an active developer workspace.');
    }
  } catch {
    return failure(502, 'identity_unavailable', 'Developer identity verification is temporarily unavailable.');
  }
  try {
    const result = await render();
    for (const [key, value] of Object.entries(privateHeaders)) result.headers.set(key, value);
    return result;
  } catch {
    return failure(500, 'documentation_unavailable', 'Developer documentation is temporarily unavailable.');
  }
}
