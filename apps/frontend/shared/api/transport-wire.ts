import { apiErrorFromResponse } from './problem';

/**
 * Транспортная обвязка шва двойного префетча #887 — общее горло двух
 * транспортов с одинаковой сигнатурой и семантикой ошибок: браузерный
 * client.ts (same-origin прокси `/api`) и серверный server-client.ts
 * (прямой ход в BACKEND_URL с cookie). Нормализация заголовков запроса
 * и разбор ответа — общий кусок обоих; специфика транспорта (URL, cookie,
 * таймаут, текст network_error) остаётся у владельца. Без 'server-only' —
 * как problem.ts: модуль нужен обоим транспортам и юнит-тестам.
 */

/**
 * Дефолт заголовков запроса: Accept: application/json всегда, Content-Type:
 * application/json для строкового body. Вызывается до добавления транспортной
 * специфики (cookie у сервера), чтобы не перекрывать явные заголовки
 * вызывающего.
 */
export function normalizeRequestHeaders(init: RequestInit): Headers {
  const headers = new Headers(init.headers);
  if (!headers.has('Accept')) {
    headers.set('Accept', 'application/json');
  }
  if (typeof init.body === 'string' && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }
  return headers;
}

/**
 * Хвост ответа, общий для обоих транспортов: неуспех → ApiError из
 * problem+json, пустой 204 → undefined, остальное — JSON тела.
 */
export async function parseTransportResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    throw await apiErrorFromResponse(response);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json() as Promise<T>;
}
