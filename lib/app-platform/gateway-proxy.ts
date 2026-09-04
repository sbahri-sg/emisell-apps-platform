import { env } from 'cloudflare:workers';

const requestHeaderAllowlist = [
  'accept',
  'accept-language',
  'authorization',
  'content-type',
  'cookie',
  'idempotency-key',
  'if-match',
  'origin',
  'x-csrf-token',
  'x-organization-id',
  'x-request-id',
] as const;

const responseHeaderAllowlist = [
  'cache-control',
  'content-disposition',
  'content-type',
  'etag',
  'location',
  'retry-after',
  'vary',
  'x-request-id',
] as const;

function gatewayOrigin() {
  const bindings = env as { APP_GATEWAY_INTERNAL_URL?: string };
  return (
    bindings.APP_GATEWAY_INTERNAL_URL ??
    process.env.APP_GATEWAY_INTERNAL_URL ??
    'http://localhost:8081'
  ).replace(/\/$/, '');
}

function forwardedRequestHeaders(request: Request) {
  const headers = new Headers();
  for (const name of requestHeaderAllowlist) {
    const value = request.headers.get(name);
    if (value) headers.set(name, value);
  }
  return headers;
}

function forwardedResponseHeaders(upstream: Response) {
  const headers = new Headers();
  for (const name of responseHeaderAllowlist) {
    const value = upstream.headers.get(name);
    if (value) headers.set(name, value);
  }

  const upstreamHeaders = upstream.headers as Headers & {
    getSetCookie?: () => string[];
  };
  const cookies = upstreamHeaders.getSetCookie?.() ?? [];
  if (cookies.length) {
    for (const cookie of cookies) headers.append('set-cookie', cookie);
  } else {
    const cookie = upstream.headers.get('set-cookie');
    if (cookie) headers.append('set-cookie', cookie);
  }
  if (!headers.has('cache-control')) headers.set('cache-control', 'no-store');
  return headers;
}

export async function proxyGatewayRequest(
  request: Request,
  namespace: 'auth' | 'v1',
  segments: string[],
) {
  if (!segments.length) {
    return Response.json(
      { error: { code: 'not_found', message: 'Gateway route not found.' } },
      { status: 404 },
    );
  }

  const incomingURL = new URL(request.url);
  const encodedPath = segments.map(encodeURIComponent).join('/');
  const upstreamURL = new URL(
    `/${namespace}/${encodedPath}${incomingURL.search}`,
    `${gatewayOrigin()}/`,
  );
  const method = request.method.toUpperCase();
  const body = method === 'GET' || method === 'HEAD'
    ? undefined
    : await request.arrayBuffer();

  try {
    const upstream = await fetch(upstreamURL, {
      method,
      headers: forwardedRequestHeaders(request),
      body,
      redirect: 'manual',
      signal: request.signal,
    });
    return new Response(upstream.body, {
      status: upstream.status,
      statusText: upstream.statusText,
      headers: forwardedResponseHeaders(upstream),
    });
  } catch {
    return Response.json(
      {
        error: {
          code: 'gateway_unavailable',
          message: 'App Gateway is temporarily unavailable.',
        },
      },
      { status: 502, headers: { 'Cache-Control': 'no-store' } },
    );
  }
}
