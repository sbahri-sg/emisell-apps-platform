const developerRoots = [
  '/overview',
  '/apps',
  '/stores',
  '/docs',
  '/accept-invitation',
  '/api-access',
  '/team',
  '/settings',
];

export function isDeveloperConsolePath(pathname) {
  return pathname === '/' || developerRoots.some(
    (root) => pathname === root || pathname.startsWith(`${root}/`),
  );
}

export function isDeveloperContentEndpoint(pathname) {
  return pathname === '/docs/content';
}

export function developerDestination(value) {
  if (
    typeof value !== 'string' ||
    !value.startsWith('/') ||
    value.startsWith('//') ||
    /[\\\r\n#]/.test(value) ||
    /%(?:0a|0d|23|5c)/i.test(value)
  ) {
    return '/overview';
  }

  try {
    const parsed = new URL(value, 'https://app-platform.invalid');
    if (parsed.origin !== 'https://app-platform.invalid') return '/overview';
    if (parsed.pathname === '/') return '/overview';
    if (
      !isDeveloperConsolePath(parsed.pathname) ||
      isDeveloperContentEndpoint(parsed.pathname)
    ) {
      return '/overview';
    }
    return `${parsed.pathname}${parsed.search}`;
  } catch {
    return '/overview';
  }
}

export function developerLoginPath(value, reason = 'required') {
  const params = new URLSearchParams({
    reason,
    next: developerDestination(value),
  });
  return `/login?${params.toString()}`;
}

