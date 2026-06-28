import { NextRequest, NextResponse } from 'next/server';

const BACKEND_URL = process.env.BACKEND_URL ?? 'http://localhost:8080';
const SESSION_COOKIE_NAME = 'session_id';

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

  try {
    const response = await fetch(`${BACKEND_URL}/me`, {
      headers: {
        cookie: cookieHeader,
      },
    });

    if (response.ok) {
      return NextResponse.next();
    }

    return redirectToLogin();
  } catch {
    return redirectToLogin();
  }
}

export const config = {
  matcher: ['/((?!login|api|_next|static|favicon.ico|.*\\..*).*)'],
};
