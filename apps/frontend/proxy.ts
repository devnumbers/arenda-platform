import { NextRequest, NextResponse } from 'next/server';

const BACKEND_URL = process.env.BACKEND_URL ?? 'http://localhost:8080';
const SESSION_COOKIE_NAMES = ['__Host-session_id', 'session_id'] as const;
const ME_TIMEOUT_MS = 5000;

function hasSessionCookie(request: NextRequest): boolean {
  return SESSION_COOKIE_NAMES.some((name) => request.cookies.has(name));
}

export async function proxy(request: NextRequest) {
  const redirectToLogin = () => {
    const from = request.nextUrl.pathname + request.nextUrl.search;
    const loginUrl = new URL('/login', request.url);
    loginUrl.searchParams.set('from', from);
    return NextResponse.redirect(loginUrl);
  };

  if (!hasSessionCookie(request)) {
    return redirectToLogin();
  }

  const cookieHeader = request.headers.get('cookie') ?? '';
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), ME_TIMEOUT_MS);

  try {
    const response = await fetch(`${BACKEND_URL}/me`, {
      headers: {
        cookie: cookieHeader,
      },
      signal: controller.signal,
    });

    if (response.ok) {
      return NextResponse.next();
    }

    return redirectToLogin();
  } catch {
    return redirectToLogin();
  } finally {
    clearTimeout(timeoutId);
  }
}

export const config = {
  matcher: [
    '/dashboard/:path*',
    '/properties/:path*',
    '/leases/:path*',
    '/tenants/:path*',
    '/finance/:path*',
    '/profile/:path*',
    '/ui-kit/:path*',
  ],
};
