import { ApiError } from './errors';
import { normalizeRequestHeaders, parseTransportResponse } from './transport-wire';

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
  const headers = normalizeRequestHeaders(options);

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

  return parseTransportResponse<T>(response);
}
