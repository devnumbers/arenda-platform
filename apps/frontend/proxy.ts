import { NextRequest, NextResponse } from 'next/server';
import { safeInternalPath } from '@/shared/lib/safe-internal-path';

const BACKEND_URL = process.env.BACKEND_URL ?? 'http://localhost:8080';
const SESSION_COOKIE_NAMES = ['__Host-session_id', 'session_id'] as const;
const ME_TIMEOUT_MS = 5000;

function hasSessionCookie(request: NextRequest): boolean {
  return SESSION_COOKIE_NAMES.some((name) => request.cookies.has(name));
}

type MeResult =
  | { kind: 'ok'; setCookies: string[] }
  | { kind: 'unauthorized'; setCookies: string[] }
  | { kind: 'error' };

async function fetchMe(request: NextRequest): Promise<MeResult> {
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

    const setCookies = response.headers.getSetCookie();
    if (response.ok) {
      return { kind: 'ok', setCookies };
    }
    if (response.status === 401) {
      return { kind: 'unauthorized', setCookies };
    }
    return { kind: 'error' };
  } catch {
    return { kind: 'error' };
  } finally {
    clearTimeout(timeoutId);
  }
}

function appendSetCookies(response: NextResponse, setCookies: string[]): NextResponse {
  for (const value of setCookies) {
    response.headers.append('set-cookie', value);
  }
  return response;
}

export async function proxy(request: NextRequest) {
  const redirectToLogin = (setCookies: string[] = []) => {
    const from = request.nextUrl.pathname + request.nextUrl.search;
    const loginUrl = new URL('/login', request.url);
    loginUrl.searchParams.set('from', from);
    return appendSetCookies(NextResponse.redirect(loginUrl), setCookies);
  };

  if (request.nextUrl.pathname === '/login') {
    if (!hasSessionCookie(request)) {
      return NextResponse.next();
    }

    const me = await fetchMe(request);
    if (me.kind === 'ok') {
      const target = safeInternalPath(request.nextUrl.searchParams.get('from')) ?? '/dashboard';
      return appendSetCookies(NextResponse.redirect(new URL(target, request.url)), me.setCookies);
    }
    if (me.kind === 'unauthorized') {
      return appendSetCookies(NextResponse.next(), me.setCookies);
    }
    return NextResponse.next();
  }

  if (!hasSessionCookie(request)) {
    return redirectToLogin();
  }

  const me = await fetchMe(request);
  if (me.kind === 'ok') {
    return appendSetCookies(NextResponse.next(), me.setCookies);
  }
  if (me.kind === 'unauthorized') {
    return redirectToLogin(me.setCookies);
  }
  return NextResponse.next();
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
    '/login',
  ],
};
