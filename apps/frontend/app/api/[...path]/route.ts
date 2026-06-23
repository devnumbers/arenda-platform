// apps/frontend/app/api/[...path]/route.ts
import { NextRequest, NextResponse } from 'next/server';
import { cookies } from 'next/headers';

const BACKEND_URL = process.env.BACKEND_URL ?? 'http://localhost:8080';

async function handler(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> },
) {
  const { path } = await params;
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

  const response = await fetch(targetUrl, {
    method: request.method,
    headers,
    body: request.body,
    // @ts-expect-error Next.js streaming requirement
    duplex: 'half',
  });

  const responseHeaders = new Headers(response.headers);
  responseHeaders.delete('transfer-encoding');

  return new NextResponse(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers: responseHeaders,
  });
}

export const GET = handler;
export const POST = handler;
export const PATCH = handler;
export const DELETE = handler;
