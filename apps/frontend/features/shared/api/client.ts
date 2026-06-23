import { ApiError } from './errors';

export async function apiClient<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const headers = new Headers(options.headers);
  if (options.body && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }

  const response = await fetch(`/api${path}`, {
    ...options,
    headers,
  });

  const requestId = response.headers.get('X-Request-ID') ?? undefined;

  if (!response.ok) {
    let code = 'unknown';
    let detail = `Request failed with status ${response.status}`;

    const contentType = response.headers.get('Content-Type');
    if (contentType?.includes('application/problem+json')) {
      const problem = (await response.json()) as Record<string, unknown>;
      code = String(problem.code ?? problem.type ?? code);
      detail = String(problem.detail ?? problem.title ?? detail);
    }

    throw new ApiError(code, detail, requestId, response.status);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json() as Promise<T>;
}
