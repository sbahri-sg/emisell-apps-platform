import { env } from 'cloudflare:workers';
import { serveDeveloperDocumentation } from '../../../lib/documentation/developer-access.mjs';
import { renderDeveloperDocumentation } from '../../../lib/documentation/developer-content.mjs';

export const dynamic = 'force-dynamic';

export async function GET(request: Request) {
  const bindings = env as { APP_GATEWAY_INTERNAL_URL?: string; AUTH_SESSION_COOKIE_NAME?: string };
  return serveDeveloperDocumentation(request, {
    gatewayUrl: bindings.APP_GATEWAY_INTERNAL_URL ?? process.env.APP_GATEWAY_INTERNAL_URL ?? 'http://localhost:8081',
    sessionCookieName: bindings.AUTH_SESSION_COOKIE_NAME ?? 'emisell_session',
    render: () => renderDeveloperDocumentation(new URL(request.url).searchParams, {
      loadGuide: async () => (await import('virtual:emisell-developer-guide')).developerGuide,
      loadSources: () => import('virtual:emisell-documentation'),
    }),
  });
}
