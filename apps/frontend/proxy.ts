import { NextResponse } from 'next/server';
import type { NextRequest } from 'next/server';

const protectedPaths = ['/dashboard', '/properties', '/tenants', '/finance', '/profile', '/support'];
const sessionCookieName = 'session_id';

export function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;

  const isProtected = protectedPaths.some((path) => pathname === path || pathname.startsWith(`${path}/`));
  if (!isProtected) {
    return NextResponse.next();
  }

  const sessionCookie = request.cookies.get(sessionCookieName);
  if (!sessionCookie?.value) {
    return NextResponse.redirect(new URL('/login', request.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: ['/((?!api|_next/static|_next/image|favicon.ico|.*\\.svg|.*\\.png|.*\\.jpg|.*\\.jpeg$).*)'],
};
