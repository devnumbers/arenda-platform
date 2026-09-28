// Стриминговый прокси SSE (карта #734, #742/#747): GET /api/notifications/stream
// — поверх общего SSE-прокси (см. shared/api/sse-proxy.ts; тот же транспорт
// обслуживает и realtime-стрим #714, #716).
import type { NextRequest } from 'next/server';
import { proxyEventStream } from '@/shared/api/sse-proxy';

export const dynamic = 'force-dynamic';

export async function GET(request: NextRequest): Promise<Response> {
  return proxyEventStream(request, '/notifications/stream');
}
