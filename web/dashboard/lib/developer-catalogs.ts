// Demonstration only: no network, credentials, installation or merchant data.
const sampleCatalogs = [
  {
    id: 'catalog_example_retail',
    title: 'Katalog Retail',
    status: 'ACTIVE',
    currencyCode: 'IDR',
    autoIncludeNewProducts: true,
    products: [],
    markets: [],
  },
  {
    id: 'catalog_example_wholesale',
    title: 'Katalog Grosir',
    status: 'DRAFT',
    currencyCode: 'IDR',
    autoIncludeNewProducts: false,
    products: [],
    markets: [],
  },
];

export function catalogSample(search = '', page = 1, limit = 10) {
  const query = search.trim().slice(0, 100);
  const size =
    Number.isInteger(limit) && limit >= 1 && limit <= 100 ? limit : 10;
  const rows = sampleCatalogs.filter((row) =>
    row.title.toLocaleLowerCase().includes(query.toLocaleLowerCase()),
  );
  const totalPages = Math.ceil(rows.length / size);
  const current = Math.max(
    1,
    Math.min(Number.isSafeInteger(page) ? page : 1, totalPages || 1),
  );
  const params = new URLSearchParams({
    page: String(current),
    limit: String(size),
    sort: 'title',
    order: 'asc',
  });
  if (query) params.set('search', query);
  const response = {
    data: rows
      .toSorted((a, b) => a.title.localeCompare(b.title))
      .slice((current - 1) * size, current * size)
      .map((row) => ({ ...row, products: [], markets: [] })),
    meta: { total: rows.length, limit: size, page: current, totalPages },
    message: 'response successfully',
  };
  return {
    request: {
      method: 'GET',
      path: `/v1/catalogs?${params}`,
      headers: { Accept: 'application/json' },
    },
    response,
  };
}
