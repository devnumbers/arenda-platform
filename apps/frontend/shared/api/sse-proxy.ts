// Общий стриминговый прокси SSE (карта #734, #742/#747; #714, #716):
// долгоживущие SSE-маршруты уходят в бэк ПОМИМО общего catch-all
// app/api/[...path]/route.ts — тот оборачивает upstream в AbortController
// с 30-секундным таймаутом, который рвёт каждое долгоживущее SSE-соединение
// (TTL соединения на бэке — час). Маршрут стримит тело upstream как есть:
// без таймаута, без буферизации; закрытие страницы отменяет upstream через
// request.signal. Last-Event-ID пробрасывается — браузер шлёт его на
// переподключении (в v1 бэк курсор только логирует, ADR 0060).
import type { NextRequest } from 'next/server';
import { BACKEND_URL } from '@/shared/config/backend-url';

export async function proxyEventStream(request: NextRequest, backendPath: string): Promise<Response> {
  const headers = new Headers({
    cookie: request.headers.get('cookie') ?? '',
    accept: 'text/event-stream',
  });
  const lastEventID = request.headers.get('last-event-id');
  if (lastEventID !== null) {
    headers.set('last-event-id', lastEventID);
  }

  try {
    const upstream = await fetch(`${BACKEND_URL}${backendPath}`, {
      method: 'GET',
      headers,
      signal: request.signal,
    });

    if (!upstream.ok || upstream.body === null) {
      // 401/problem и прочие не-стримы — passthrough: не-200 у EventSource —
      // fail-соединение, переподключением займётся клиентский провайдер.
      return new Response(upstream.body, {
        status: upstream.status,
        statusText: upstream.statusText,
        headers: { 'content-type': upstream.headers.get('content-type') ?? 'application/json' },
      });
    }

    return new Response(upstream.body, {
      status: 200,
      headers: {
        'content-type': 'text/event-stream; charset=utf-8',
        'cache-control': 'no-cache, no-transform',
        'x-accel-buffering': 'no',
      },
    });
  } catch {
    // Отмена со стороны страницы (request.signal) или сеть — соединение
    // закончено; reconnect делает клиент.
    return new Response(null, { status: 499 });
  }
}
