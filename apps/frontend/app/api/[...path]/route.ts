// apps/frontend/app/api/[...path]/route.ts
import type { NextRequest } from 'next/server';
import { NextResponse } from 'next/server';
import { cookies } from 'next/headers';
import { BACKEND_URL } from '@/shared/config/backend-url';

const BACKEND_TIMEOUT_MS = 30000;

async function handler(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> },
) {
  const { path } = await params;

  if (path.length === 0 || path.some((segment) => segment === '' || segment === '..')) {
    return NextResponse.json({ error: 'Некорректный запрос' }, { status: 400 });
  }

  const targetPath = `/${path.join('/')}`;
  const search = request.nextUrl.searchParams.toString();
  const targetUrl = `${BACKEND_URL}${targetPath}${search ? `?${search}` : ''}`;

  const cookieStore = await cookies();
  const cookieHeader = cookieStore.toString();

  const headers = new Headers(request.headers);
  headers.delete('host');
  if (cookieHeader) {
    headers.set('cookie', cookieHeader);
  }

  const body =
    request.method !== 'GET' && request.method !== 'HEAD'
      ? await request.arrayBuffer()
      : undefined;

  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), BACKEND_TIMEOUT_MS);

  try {
    const response = await fetch(targetUrl, {
      method: request.method,
      headers,
      body,
      signal: controller.signal,
    });

    const responseHeaders = new Headers(response.headers);
    responseHeaders.delete('transfer-encoding');
    responseHeaders.delete('set-cookie');

    const setCookies = response.headers.getSetCookie();
    for (const cookie of setCookies) {
      responseHeaders.append('Set-Cookie', cookie);
    }

    return new NextResponse(response.body, {
      status: response.status,
      statusText: response.statusText,
      headers: responseHeaders,
    });
  } catch {
    return NextResponse.json({ error: 'Ошибка сервера' }, { status: 502 });
  } finally {
    clearTimeout(timeoutId);
  }
}

export const GET = handler;
export const POST = handler;
export const PUT = handler;
export const PATCH = handler;
export const DELETE = handler;
export const OPTIONS = handler;
export const HEAD = handler;
