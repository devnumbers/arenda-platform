import { ApiError } from './errors';
import { apiErrorFromResponse } from './problem';

/**
 * Транспорт API-запроса — шов двойного префетча #887: дефолт (браузер)
 * ходит через same-origin прокси `/api`, серверный слой префетча подставляет
 * serverApiClient (прямой ход в BACKEND_URL с cookie). Одинаковая сигнатура
 * и семантика ошибок — queryOptions-фабрики пишутся один раз.
 */
export type ApiTransport = typeof apiClient;

export async function apiClient<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const headers = new Headers(options.headers);
  if (!headers.has('Accept')) {
    headers.set('Accept', 'application/json');
  }
  if (typeof options.body === 'string' && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }

  let response: Response;
  try {
    response = await fetch(`/api${path}`, {
      ...options,
      headers,
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'Не удалось выполнить запрос. Проверьте подключение к интернету.';
    throw new ApiError('network_error', message, undefined, undefined, error);
  }

  if (!response.ok) {
    throw await apiErrorFromResponse(response);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json() as Promise<T>;
}
