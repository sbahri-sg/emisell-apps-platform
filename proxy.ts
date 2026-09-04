import { NextRequest, NextResponse } from 'next/server';
import {
  developerLoginPath,
  isDeveloperConsolePath,
  isDeveloperContentEndpoint,
} from './lib/app-platform/access-routing.mjs';

const developerSessionCookie =
  process.env.AUTH_SESSION_COOKIE_NAME ?? 'emisell_session';
const adminSessionCookie = 'emisell_admin_session';

function hasCookie(request: NextRequest, name: string) {
  return Boolean(request.cookies.get(name)?.value.trim());
}

export function proxy(request: NextRequest) {
  const { pathname, search } = request.nextUrl;

  if (
    isDeveloperConsolePath(pathname) &&
    !isDeveloperContentEndpoint(pathname) &&
    !hasCookie(request, developerSessionCookie)
  ) {
    return NextResponse.redirect(
      new URL(developerLoginPath(`${pathname}${search}`), request.url),
    );
  }

  const adminPage =
    (pathname === '/admin' || pathname.startsWith('/admin/')) &&
    pathname !== '/admin/login' &&
    pathname !== '/admin/docs/content';
  if (adminPage && !hasCookie(request, adminSessionCookie)) {
    const login = new URL('/admin/login', request.url);
    login.searchParams.set('reason', 'required');
    login.searchParams.set('next', `${pathname}${search}`);
    return NextResponse.redirect(login);
  }

  return NextResponse.next();
}

export const config = {
  matcher: [
    '/',
    '/overview',
    '/apps/:path*',
    '/stores/:path*',
    '/docs/:path*',
    '/accept-invitation',
    '/api-access',
    '/team',
    '/settings',
    '/admin/:path*',
  ],
};
