import type { NextRequest } from 'next/server';
import { NextResponse } from 'next/server';
import { buildCspReportOnlyPolicy, generateCspNonce } from '@/shared/lib/csp';
import { safeInternalPath } from '@/shared/lib/safe-internal-path';

const BACKEND_URL = process.env.BACKEND_URL ?? 'http://localhost:8080';
// CSP шаг 2 — разведка (тикет #406, решение #331): nonce + strict-dynamic
// в Content-Security-Policy-Report-Only. Флаг ставит только stage-compose
// (deploy/docker-compose.stage.yml); без переменной поведение прежнее —
// прод конфигурацию шаг не трогает.
const CSP_REPORT_ONLY = process.env.CSP_REPORT_ONLY === 'true';
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
  // Разведка строгой CSP: nonce на каждый запрос. Заголовок запроса Next
  // разбирает при SSR и вешает nonce на фреймворк-скрипты и инлайн-стили
  // (Report-Only он понимает так же, как блокирующий — app-render Next 16);
  // заголовок ответа браузер не блокирует, только репортит в /api/csp-report.
  const reportOnlyPolicy = CSP_REPORT_ONLY
    ? buildCspReportOnlyPolicy(generateCspNonce(), { isDev: process.env.NODE_ENV === 'development' })
    : null;
  const forwardHeaders = reportOnlyPolicy
    ? (() => {
        const headers = new Headers(request.headers);
        headers.set('Content-Security-Policy-Report-Only', reportOnlyPolicy);
        return headers;
      })()
    : undefined;
  const withReportOnlyCsp = (response: NextResponse): NextResponse => {
    if (reportOnlyPolicy !== null) {
      response.headers.set('Content-Security-Policy-Report-Only', reportOnlyPolicy);
    }
    return response;
  };
  const next = () =>
    withReportOnlyCsp(NextResponse.next(forwardHeaders ? { request: { headers: forwardHeaders } } : undefined));

  const redirectToLogin = (setCookies: string[] = []) => {
    const from = request.nextUrl.pathname + request.nextUrl.search;
    const loginUrl = new URL('/login', request.url);
    loginUrl.searchParams.set('from', from);
    return appendSetCookies(withReportOnlyCsp(NextResponse.redirect(loginUrl)), setCookies);
  };

  if (request.nextUrl.pathname === '/login') {
    if (!hasSessionCookie(request)) {
      return next();
    }

    const me = await fetchMe(request);
    if (me.kind === 'ok') {
      const target = safeInternalPath(request.nextUrl.searchParams.get('from')) ?? '/dashboard';
      return appendSetCookies(withReportOnlyCsp(NextResponse.redirect(new URL(target, request.url))), me.setCookies);
    }
    if (me.kind === 'unauthorized') {
      return appendSetCookies(next(), me.setCookies);
    }
    return next();
  }

  if (!hasSessionCookie(request)) {
    return redirectToLogin();
  }

  const me = await fetchMe(request);
  if (me.kind === 'ok') {
    return appendSetCookies(next(), me.setCookies);
  }
  if (me.kind === 'unauthorized') {
    return redirectToLogin(me.setCookies);
  }
  return next();
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
