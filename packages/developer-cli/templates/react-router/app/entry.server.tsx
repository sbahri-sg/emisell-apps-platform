import { PassThrough } from 'node:stream';
import { createReadableStreamFromReadable } from '@react-router/node';
import { renderToPipeableStream } from 'react-dom/server';
import { ServerRouter } from 'react-router';
import type { AppLoadContext, EntryContext } from 'react-router';

export const streamTimeout = 5_000;

// Stream React Router's hydration data with the same per-response CSP nonce.
export default function handleRequest(request: Request, status: number, headers: Headers, routerContext: EntryContext, context: AppLoadContext) {
  headers.set('Content-Type', 'text/html; charset=utf-8');
  if (request.method === 'HEAD') return new Response(null, { status, headers });
  return new Promise<Response>((resolve, reject) => {
    let timer: ReturnType<typeof setTimeout>;
    const nonce = context.nonce as string;
    const { pipe, abort } = renderToPipeableStream(
      <ServerRouter context={routerContext} url={request.url} nonce={nonce}/>, {
        nonce,
        onAllReady() {
          const body = new PassThrough();
          const cleanup = () => { clearTimeout(timer); request.signal.removeEventListener('abort', abort); };
          body.once('close', cleanup);
          resolve(new Response(createReadableStreamFromReadable(body), { status, headers }));
          pipe(body);
        },
        onShellError() { clearTimeout(timer); request.signal.removeEventListener('abort', abort); reject(Error('Page rendering unavailable')); },
        onError() { status = 500; },
      },
    );
    timer = setTimeout(abort, streamTimeout + 1000);
    request.signal.addEventListener('abort', abort, { once: true });
    if (request.signal.aborted) abort();
  });
}
