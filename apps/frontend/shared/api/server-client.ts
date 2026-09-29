import 'server-only';
import { cookies } from 'next/headers';
import { BACKEND_URL } from '@/shared/config/backend-url';
import { ApiError } from './errors';
import { normalizeRequestHeaders, parseTransportResponse } from './transport-wire';

const BACKEND_TIMEOUT_MS = 30000;

/**
 * Серверный транспорт API — половина канона серверного префетча #887:
 * RSC-слой префетча ходит в бэк напрямую (BACKEND_URL с cookie запроса),
 * повторяя семантику прокси `/api` (app/api/[...path]/route.ts): таймаут
 * 30с, problem+json → ApiError, сетевой сбой → ApiError('network_error').
 * Относительный `/api` на сервере не резолвится, а тащить фетч через
 * собственный роут-хендлер значило бы петлю через себя.
 *
 * 401/403 здесь не обрабатываются особо: канон «нет гидратации» исполняет
 * слой префетча — неуспешные запросы не попадают в HydrationBoundary,
 * клиент перечитывает сам и ведёт себя как сегодня (см. server-prefetch).
 *
 * Set-Cookie не пробрасывается: префетч read-only, ротацию сессии несёт
 * гейт proxy.ts и ответы мутаций браузерного клиента.
 */
export async function serverApiClient<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const headers = normalizeRequestHeaders(options);

  const cookieStore = await cookies();
  const cookieHeader = cookieStore.toString();
  if (cookieHeader) {
    headers.set('cookie', cookieHeader);
  }

  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), BACKEND_TIMEOUT_MS);

  let response: Response;
  try {
    response = await fetch(`${BACKEND_URL}${path}`, {
      ...options,
      headers,
      signal: controller.signal,
    });
  } catch (error) {
    throw new ApiError(
      'network_error',
      'Не удалось выполнить запрос на сервере',
      undefined,
      undefined,
      error,
    );
  } finally {
    clearTimeout(timeoutId);
  }

  return parseTransportResponse<T>(response);
}
