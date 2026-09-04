import { serveDocumentation } from '../../../../lib/documentation/access.mjs';
import { buildPostman, buildReference, contracts, selectSpec } from '../../../../lib/documentation/contracts.mjs';
import { env } from 'cloudflare:workers';

export const dynamic = 'force-dynamic';

export async function GET(request: Request) {
  const bindings = env as { APP_GATEWAY_INTERNAL_URL?: string; AUTH_SESSION_COOKIE_NAME?: string };
  return serveDocumentation(request, {
    gatewayUrl: bindings.APP_GATEWAY_INTERNAL_URL ?? process.env.APP_GATEWAY_INTERNAL_URL ?? 'http://localhost:8081',
    sessionCookieName: 'emisell_admin_session',
    render: async () => {
      const params = new URL(request.url).searchParams;
      const contract = params.get('contract') ?? 'emisell';
      const format = params.get('format') ?? 'reference';
      if (!contracts.some((entry) => entry.id === contract) || !['reference', 'openapi', 'postman'].includes(format)) {
        return Response.json({ error: { code: 'validation_error', message: 'Unknown documentation contract or format.' } }, { status: 400 });
      }
      // Import only on the server, after authorization. No source contract or
      // generated private artifact is emitted into public/ or a client bundle.
      const sources = await import('virtual:emisell-documentation');
      const result = format === 'openapi' ? selectSpec(sources, contract) : format === 'postman' ? buildPostman(sources, contract) : buildReference(sources, contract);
      return Response.json(result, { headers: format === 'reference' ? {} : {
        'Content-Disposition': `attachment; filename="emisell-${contract}.${format === 'postman' ? 'postman_collection' : 'openapi'}.json"`,
      } });
    },
  });
}
