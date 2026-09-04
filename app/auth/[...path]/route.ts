import { proxyGatewayRequest } from '../../../lib/app-platform/gateway-proxy';

export const dynamic = 'force-dynamic';

type RouteContext = { params: Promise<{ path: string[] }> };

async function handler(request: Request, context: RouteContext) {
  return proxyGatewayRequest(request, 'auth', (await context.params).path);
}

export const GET = handler;
export const POST = handler;
export const PUT = handler;
export const PATCH = handler;
export const DELETE = handler;
