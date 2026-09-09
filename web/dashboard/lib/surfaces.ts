// Product boundaries, not a registry of deployed or implemented services.
export const SURFACES = [
  {
    id: 'admin',
    name: 'Dashboard Admin',
    port: 4317,
    audience: 'Tim Emisell',
    description: 'Review, publikasi, developer, dan operasional platform.',
  },
  {
    id: 'store',
    name: 'App Store',
    port: 4318,
    audience: 'Pengunjung & merchant',
    description: 'Temukan aplikasi yang sudah dipublikasikan.',
  },
  {
    id: 'developer',
    name: 'Portal Developer',
    port: 4319,
    audience: 'Pembuat aplikasi',
    description:
      'Kelola aplikasi, versi, dan pengajuan review milik developer.',
  },
] as const;

// URL state can select a permitted view only, never identity or merchant mode.
export function portalDocsGroup(search: string): string {
  const group = new URLSearchParams(search).get('api_group') ?? '';
  return [
    'admin',
    'developer',
    'store',
    'core',
    'app-access',
    'gateway',
  ].includes(group)
    ? group
    : 'admin';
}

export function portalView(surface: string, search: string): string {
  const view = new URLSearchParams(search).get('view');
  const allowed =
    surface === 'admin'
      ? [
          'overview',
          'activity',
          'staff',
          'developers',
          'apps',
          'reviews',
          'catalog',
          'integration-releases',
          'ui-releases',
          'scopes',
          'api-docs',
          'api-keys',
          'testing',
          'app-clients',
        ]
      : [
          'apps',
          'stores',
          'catalogs',
          'monitoring',
          'logs',
          'versions',
          'app-settings',
          'reviews',
          'catalog',
          'integration-releases',
          'ui-releases',
          'scopes',
          'tooling',
          'testing',
          'app-clients',
        ];
  return view && allowed.includes(view) ? view : allowed[0];
}
