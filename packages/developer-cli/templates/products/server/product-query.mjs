// Only filters, never tenant identity or credentials, may come from the browser.
export function productQuery(search = new URLSearchParams()) {
  for (const key of search.keys()) {
    if (!['limit', 'cursor', 'q'].includes(key) || search.getAll(key).length !== 1) throw Error('invalid_query');
  }
  const limit = search.get('limit') ?? '5';
  if (!/^[1-9][0-9]?$/.test(limit) || Number(limit) > 20) throw Error('invalid_query');
  const q = search.get('q'), cursor = search.get('cursor');
  if (q !== null && (!q || q !== q.trim() || Buffer.byteLength(q) > 100 || /[\x00-\x1f\x7f]/.test(q))) throw Error('invalid_query');
  if (cursor !== null && (!cursor || cursor.length > 1024 || !/^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/.test(cursor))) throw Error('invalid_query');
  return { limit: Number(limit), ...(q ? { q } : {}), ...(cursor ? { cursor } : {}) };
}

export function productPage(value, limit) {
  if (!value || !Array.isArray(value.data) || value.data.length > limit) throw Error('invalid_products');
  const data = value.data.map(p => {
    if (!p || typeof p.id !== 'string' || !/^[\x21-\x7e]{1,256}$/.test(p.id) || typeof p.name !== 'string' || p.name.length > 2000 ||
        typeof p.price !== 'string' || !/^-?\d{1,24}(?:\.\d{1,8})?$/.test(p.price)) throw Error('invalid_products');
    return { id: p.id, name: p.name, price: p.price };
  });
  const nextCursor = value.meta?.nextCursor ?? null;
  if (nextCursor !== null && (typeof nextCursor !== 'string' || nextCursor.length > 1024 || !/^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/.test(nextCursor) || !data.length)) throw Error('invalid_products');
  return { data, meta: { nextCursor } };
}
