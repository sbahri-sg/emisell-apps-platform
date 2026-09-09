export type DeveloperStore = { id: string; name: string; commonId: string };
export type DeveloperAccountData = {
  profile: null | { email: string; name: string; stores: DeveloperStore[] };
  sellerOrigin: string;
};

// Only navigate to fixed seller routes for a store supplied by the account API.
export function developerStoreURL(
  account: DeveloperAccountData,
  id: string,
  destination: 'home' | 'apps' = 'home',
) {
  const store = account.profile?.stores.find((value) => value.id === id);
  if (!store || !/^[A-Za-z0-9_-]{1,128}$/.test(store.commonId)) return null;
  try {
    const url = new URL(account.sellerOrigin);
    if (
      !['http:', 'https:'].includes(url.protocol) ||
      url.username ||
      url.password ||
      url.search ||
      url.hash ||
      url.pathname !== '/'
    )
      return null;
    url.pathname = `/store/${encodeURIComponent(store.commonId)}${destination === 'apps' ? '/settings/apps' : ''}`;
    return url.href;
  } catch {
    return null;
  }
}

export function searchDeveloperStores(stores: DeveloperStore[], query: string) {
  const term = query.trim().toLocaleLowerCase();
  return stores.filter((store) =>
    `${store.name} ${store.commonId}`.toLocaleLowerCase().includes(term),
  );
}

// Navigation only: Core rechecks its current seller session and permission.
export function developerInstallURL(
  account: DeveloperAccountData,
  merchantId: string,
  appId: string,
  version: string,
) {
  if (
    !/^app_[A-Z2-7]{26}$/.test(appId) ||
    !/^\d{1,6}\.\d{1,6}\.\d{1,6}$/.test(version)
  )
    return null;
  const destination = developerStoreURL(account, merchantId, 'apps');
  if (!destination) return null;
  const url = new URL(destination);
  url.pathname = url.pathname.replace(/\/settings\/apps$/, '/app/grant');
  url.searchParams.set('app', appId);
  url.searchParams.set('version', version);
  return url.href;
}
