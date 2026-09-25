// Стриминговый прокси SSE realtime-кадров (карта #714, #716; ADR 0062):
// GET /api/realtime/stream — по образцу notifications stream, поверх общего
// SSE-прокси (см. shared/api/sse-proxy.ts).
import type { NextRequest } from 'next/server';
import { proxyEventStream } from '@/shared/api/sse-proxy';

export const dynamic = 'force-dynamic';

export async function GET(request: NextRequest): Promise<Response> {
  return proxyEventStream(request, '/realtime/stream');
}
