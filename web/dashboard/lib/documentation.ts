import generated from './documentation.generated.json' with { type: 'json' };

// Safe build-time Markdown AST, never internal contracts or runtime account data.
export type DocNode = {
  type: string;
  text?: string;
  children?: DocNode[];
  href?: string;
  id?: string;
  depth?: number;
  language?: string;
  ordered?: boolean;
  start?: number;
  items?: DocNode[][];
  header?: DocNode[][];
  rows?: DocNode[][][];
};
export type DocArticle = {
  id: string;
  title: string;
  summary: string;
  category: string;
  minutes: number;
  searchText: string;
  nodes: DocNode[];
  headings: { id: string; title: string; depth: number }[];
};
export const documentationArticles = generated as DocArticle[];
export const documentationTopics = documentationArticles.map(
  ({ id, title, summary, category, minutes }) => ({
    id,
    title,
    summary,
    category,
    minutes,
  }),
);
export const documentationCategories = [
  ...new Set(documentationTopics.map(({ category }) => category)),
];
export type DocumentationTopic = string;
export function isDocumentationTopic(
  value: string,
): value is DocumentationTopic {
  return documentationTopics.some((topic) => topic.id === value);
}

export function adjacentArticles(id: string) {
  const index = documentationTopics.findIndex((article) => article.id === id);
  return {
    previous: index > 0 ? documentationTopics[index - 1] : null,
    next: index >= 0 ? (documentationTopics[index + 1] ?? null) : null,
  };
}

export function searchDocumentation(query: string) {
  const terms = query
    .trim()
    .toLocaleLowerCase('id-ID')
    .slice(0, 200)
    .split(/\s+/)
    .filter(Boolean);
  if (!terms.length) return [];
  return documentationArticles
    .flatMap((article) => {
      const title = article.title.toLocaleLowerCase('id-ID');
      const body = `${article.summary} ${article.searchText}`.toLocaleLowerCase(
        'id-ID',
      );
      if (!terms.every((term) => `${title} ${body}`.includes(term))) return [];
      const heading = article.headings.find(({ title }) =>
        terms.every((term) => title.toLocaleLowerCase('id-ID').includes(term)),
      );
      return [
        {
          id: article.id,
          title: article.title,
          summary: article.summary,
          category: article.category,
          href: `/docs/${article.id}${heading ? `#${heading.id}` : ''}`,
          score:
            terms.filter((term) => title.includes(term)).length * 10 +
            (heading ? 2 : 0),
        },
      ];
    })
    .sort((a, b) => b.score - a.score)
    .slice(0, 8);
}

export const cliCommands = [
  ['emisell app init', 'Buat project baru melalui wizard.'],
  ['emisell app dev', 'Jalankan aplikasi dan backend lokal.'],
  ['emisell app build', 'Build project aplikasi.'],
  ['emisell app info', 'Lihat metadata project tanpa membaca credential.'],
  ['emisell app doctor', 'Periksa Node.js, metadata, file, dan dependency.'],
] as const;

export const resourceScopes = [
  ['read_products', 'Produk', 'Daftar, pencarian, dan detail produk.'],
  ['read_orders', 'Pesanan', 'Daftar, filter status, dan detail item pesanan.'],
  [
    'read_shipping',
    'Pengiriman',
    'Konfigurasi, profil, dan nama zona; bukan tarif atau pembuatan shipment.',
  ],
  [
    'read_catalogs',
    'Katalog',
    'Daftar dan referensi produk; bukan harga khusus katalog.',
  ],
  [
    'read_collections',
    'Koleksi',
    'Daftar, pencarian, dan referensi produk dalam koleksi.',
  ],
  ['read_inventory', 'Stok', 'Baca stok tanpa mengubah jumlah persediaan.'],
  [
    'read_locations',
    'Lokasi',
    'Nama dan status lokasi; tanpa alamat atau telepon.',
  ],
] as const;
