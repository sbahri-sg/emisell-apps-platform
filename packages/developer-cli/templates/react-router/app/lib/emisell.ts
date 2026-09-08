import { getIdentity } from './bridge.mjs';

export type Product = { id: string; name: string; price: string };
export type ProductPage = { data: Product[]; meta: { nextCursor: string | null } };
export type Session = { status: 'connected'; merchantId: string; appId: string; actorId: string; installationId: string; expiresAt: number };

// A fresh identity on every request. Never persist a bearer or trust it as a grant.
async function request<T>(parentOrigin: string, endpoint: string, query: URLSearchParams, signal?: AbortSignal): Promise<T> {
  signal?.throwIfAborted();
  const identity = await getIdentity({ parentOrigin }) as { token: string };
  signal?.throwIfAborted();
  const timeout = AbortSignal.timeout(15000);
  const response = await fetch(endpoint + (query.size ? '?' + query : ''), {
    method: 'POST', headers: { Authorization: `Bearer ${identity.token}` },
    credentials: 'omit', cache: 'no-store', redirect: 'error', signal: signal ? AbortSignal.any([signal, timeout]) : timeout,
  });
  if (response.status === 401 || response.status === 403) throw Error('Akses ditolak atau sudah dicabut. Periksa izin dan instalasi di Dashboard seller.');
  if (!response.ok) throw Error('Backend belum siap. Periksa konfigurasi .env dan koneksi Core lokal.');
  return response.json();
}

export const emisell = {
  resource: (parentOrigin: string, path: string, query = new URLSearchParams(), signal?: AbortSignal) => {
    if (!/^\/v1\/(?:(?:products|orders|catalogs|collections)(?:\/[A-Za-z0-9_-]{1,128})?|settings\/location(?:\/[A-Za-z0-9_-]{1,128})?|settings\/shipping(?:\/profile\/[A-Za-z0-9_-]{1,128})?)$/.test(path)) throw Error('Resource tidak didukung.');
    return request<ResourceResult>(parentOrigin,path,query,signal);
  },
  session: (parentOrigin: string, signal?: AbortSignal) => request<Session>(parentOrigin, '/api/session', new URLSearchParams(), signal),
  async products(parentOrigin: string, { q = '', cursor, signal }: { q?: string; cursor?: string | null; signal?: AbortSignal } = {}): Promise<ProductPage> {
    if (new TextEncoder().encode(q).length > 100 || /[\x00-\x1f\x7f]/.test(q)) throw Error('Pencarian maksimum 100 byte, tanpa karakter kontrol.');
    const query = new URLSearchParams({ limit: '5' });
    if (q) query.set('q', q);
    if (cursor) query.set('cursor', cursor);
    const result = await request<ProductPage>(parentOrigin, '/api/products', query, signal);
    if (!Array.isArray(result.data) || result.data.length > 5 || !result.meta ||
        !(result.meta.nextCursor === null || typeof result.meta.nextCursor === 'string')) throw Error('Respons produk tidak valid.');
    return result;
  },
};
export type ResourceRow = {id:string;name?:string;title?:string;productCount?:number;stock?:number;isActive?:boolean;numberFormat?:string;status?:string;subtotal?:string;currency?:string|null};
export type ResourceResult = {data: ResourceRow[] | {id:string;profiles?:ResourceRow[];[key:string]:unknown} | null;meta?:{nextCursor:string|null}};
