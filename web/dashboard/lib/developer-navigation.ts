import { portalView } from './surfaces.ts';

export const developerMainMenu = [
  { id: 'apps', label: 'Apps' },
  { id: 'stores', label: 'Stores' },
  { id: 'catalogs', label: 'Catalogs' },
];

// Presentation state only. The API still authenticates and authorizes every request.
export const developerAppMenu = [
  { id: 'apps', label: 'Overview' },
  { id: 'monitoring', label: 'Monitoring' },
  { id: 'logs', label: 'Logs' },
  { id: 'versions', label: 'Versions' },
  { id: 'app-settings', label: 'App settings' },
];
export const appContextViews = [
  ...developerAppMenu.map((item) => item.id),
  'app-clients',
  'reviews',
];

export function developerMenuView(view: string) {
  if (view === 'app-clients') return 'app-settings';
  if (view === 'reviews') return 'versions';
  return view;
}

export function developerLocation(search: string) {
  const params = new URLSearchParams(search);
  const view = portalView('developer', search);
  const appId = appContextViews.includes(view) ? (params.get('app') ?? '') : '';
  return { view, appId };
}

export function developerSearch(search: string, next: string, appId = '') {
  const params = new URLSearchParams(search);
  params.set('view', next);
  const view = portalView('developer', params.toString());
  params.set('view', view);
  params.delete('app');
  if (appId && appContextViews.includes(view)) params.set('app', appId);
  return `?${params.toString()}`;
}

export function forApp<T>(
  rows: T[],
  appId: string | undefined,
  identify: (row: T) => string | undefined,
): T[] {
  return appId ? rows.filter((row) => identify(row) === appId) : rows;
}
