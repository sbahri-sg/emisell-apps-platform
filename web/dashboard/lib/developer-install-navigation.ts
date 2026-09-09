// The origin comes from the authenticated API's operator configuration, never a query override.
export function installSelectionURL(
  value: unknown,
  appId: string,
): string | null {
  if (typeof value !== 'string') return null;
  try {
    const url = new URL(value);
    const params = url.searchParams;
    if (
      !['http:', 'https:'].includes(url.protocol) ||
      url.username ||
      url.password ||
      url.hash ||
      url.pathname !== '/auth/stores' ||
      !/^app_[A-Z2-7]{26}$/.test(appId) ||
      params.get('app') !== appId ||
      params.getAll('app').length !== 1 ||
      params.getAll('version').length !== 1 ||
      !/^\d{1,6}\.\d{1,6}\.\d{1,6}$/.test(params.get('version') ?? '') ||
      [...params.keys()].some((key) => key !== 'app' && key !== 'version')
    )
      return null;
    return url.href;
  } catch {
    return null;
  }
}
