import { NextRequest, NextResponse } from 'next/server';

const BACKEND_URL = process.env.BACKEND_URL ?? 'http://localhost:8080';
const SESSION_COOKIE_NAME = 'session_id';
const ME_TIMEOUT_MS = 5000;

export async function middleware(request: NextRequest) {
  const sessionCookie = request.cookies.get(SESSION_COOKIE_NAME);

  const redirectToLogin = () => {
    const from = request.nextUrl.pathname + request.nextUrl.search;
    const loginUrl = new URL('/login', request.url);
    loginUrl.searchParams.set('from', from);
    return NextResponse.redirect(loginUrl);
  };

  if (!sessionCookie) {
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
  matcher: ['/((?!api/|_next/|static/|login|favicon\\.ico|.*\\..*).*)'],
};
