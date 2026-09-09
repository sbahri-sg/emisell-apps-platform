import { catalogSample } from './developer-catalogs.ts';

export const initialCatalogConfiguration = {
  source: 'own',
  store: 'store_example',
  prefix: '',
  limit: 10,
  buyerCountry: '',
  shipsTo: '',
  shipsFrom: '',
  categories: '',
  color: '',
  gender: '',
  size: '',
  condition: '',
  inStock: true,
  minPrice: '0',
  maxPrice: '',
  priceTier: '',
  minRating: '',
  minReviews: '',
};
export type CatalogConfiguration = typeof initialCatalogConfiguration;

// Unsupported filters stay in the mock envelope, never in the backend request.
export function catalogPreview(
  query: string,
  title: string,
  configuration: CatalogConfiguration,
) {
  const sample = catalogSample(
    [configuration.prefix, query].filter(Boolean).join(' '),
    1,
    configuration.limit,
  );
  return {
    request: {
      mock: true,
      request: sample.request,
      preview: { catalog: title, ...configuration },
    },
    response: { mock: true, ...sample.response },
    path: sample.request.path,
  };
}

export function catalogPreviewCode(
  value: ReturnType<typeof catalogPreview>,
  tab: string,
  format: string,
) {
  if (tab === 'response') return JSON.stringify(value.response, null, 2);
  if (format === 'curl')
    return [
      '# Example only. This preview does not execute requests.',
      '# Use merchant authentication in your backend.',
      `curl --request GET 'https://api.emisell.example${value.path}'`,
      '',
      '# Region, attribute and listing controls are mock-only.',
    ].join('\n');
  if (format === 'js')
    return `// Example only; not executed by this preview.\n// Use the existing authenticated merchant API client.\nconst path = ${JSON.stringify(value.path)};\nconst catalogs = await merchantApi.get(path);\n\n// Region, attribute and listing controls are mock-only.\nconsole.log(catalogs);`;
  return JSON.stringify(value.request, null, 2);
}
