// Navigation organization only; authorization remains in the existing routes.
export const adminNavigationGroups = [
  { label: 'Utama', collapsible: false, ids: ['overview', 'apps', 'developers'] },
  { label: 'Review & distribusi', collapsible: false, ids: ['reviews', 'testing', 'catalog'] },
  { label: 'Integrasi & akses', collapsible: true, ids: ['integration-releases', 'ui-releases', 'app-clients', 'api-keys', 'scopes'] },
  { label: 'Platform', collapsible: false, ids: ['activity', 'staff', 'api-docs'] },
];
