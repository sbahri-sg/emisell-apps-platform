import { serveDeveloperDocumentation } from '../../../lib/documentation/developer-access.mjs';
import { renderDeveloperDocumentation } from '../../../lib/documentation/developer-content.mjs';

export const dynamic = 'force-dynamic';

export async function GET(request: Request) {
  return serveDeveloperDocumentation(request, {
    gatewayUrl: process.env.APP_GATEWAY_INTERNAL_URL ?? 'http://localhost:8081',
    sessionCookieName: process.env.AUTH_SESSION_COOKIE_NAME ?? 'emisell_session',
    render: () => renderDeveloperDocumentation(new URL(request.url).searchParams, {
      loadGuide: async () => (await import('virtual:emisell-developer-guide')).developerGuide,
      loadSources: () => import('virtual:emisell-documentation'),
    }),
  });
}
