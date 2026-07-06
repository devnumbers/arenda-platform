import { ApiError } from './errors';

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

  const requestId = response.headers.get('X-Request-ID') ?? undefined;

  if (!response.ok) {
    let code = 'unknown';
    let detail = `Ошибка сервера (код ${response.status})`;

    const contentType = response.headers.get('Content-Type');
    if (contentType?.includes('application/problem+json')) {
      try {
        const problem = (await response.json()) as Record<string, unknown>;
        code = String(problem.code ?? problem.type ?? code);
        detail = String(problem.detail ?? problem.title ?? detail);
      } catch {
        detail = `Ошибка сервера (код ${response.status})`;
      }
    }

    const retryAfterRaw = response.headers.get('Retry-After');
    const retryAfter = retryAfterRaw ? Number(retryAfterRaw) : NaN;
    const validRetryAfter =
      Number.isFinite(retryAfter) && retryAfter > 0 ? retryAfter : undefined;

    throw new ApiError(
      code,
      detail,
      requestId,
      response.status,
      undefined,
      validRetryAfter,
    );
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json() as Promise<T>;
}
