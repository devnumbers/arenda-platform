import { ApiError, type FieldError } from './errors';

/**
 * Разбор problem+json ответа бэка в ApiError — общее горло браузерного
 * клиента (/api-прокси, client.ts) и серверного префетча (server-client.ts):
 * семантика ошибок у обоих транспортов одинаковая по канону #887. Поля
 * problem+json — строки по контракту (Problem schema); нестроковое —
 * нарушение протокола, фолбэк не пускает его в UI как '[object Object]'.
 */
function problemText(value: unknown, fallback: string): string {
    return typeof value === 'string' ? value : fallback;
}

export async function apiErrorFromResponse(response: Response): Promise<ApiError> {
    const requestId = response.headers.get('X-Request-ID') ?? undefined;

    let code = 'unknown';
    let detail = `Ошибка сервера (код ${response.status})`;
    let fieldErrors: readonly FieldError[] | undefined;

    const contentType = response.headers.get('Content-Type');
    if (contentType?.includes('application/problem+json')) {
        try {
            const problem = (await response.json()) as Record<string, unknown>;
            code = problemText(problem.code ?? problem.type, code);
            detail = problemText(problem.detail ?? problem.title, detail);
            if (Array.isArray(problem.errors)) {
                fieldErrors = problem.errors
                    .map((e) => (e && typeof e === 'object' ? (e as unknown) : null))
                    .filter((e): e is Record<string, unknown> => e !== null)
                    .map((e) => ({ field: problemText(e.field, ''), detail: problemText(e.detail, '') }))
                    .filter((e) => e.field !== '');
            }
        } catch {
            detail = `Ошибка сервера (код ${response.status})`;
        }
    }

    const retryAfterRaw = response.headers.get('Retry-After');
    const retryAfter = retryAfterRaw ? Number(retryAfterRaw) : NaN;
    const validRetryAfter =
        Number.isFinite(retryAfter) && retryAfter > 0 ? retryAfter : undefined;

    return new ApiError(
        code,
        detail,
        requestId,
        response.status,
        undefined,
        validRetryAfter,
        fieldErrors,
    );
}
