import { ApiError } from '@/shared/api/errors';

/**
 * Глобальный retry-предикат запросов (#769): детерминированные 4xx
 * (включая 429 — у resend-кулдауна свой UX) не ретраются вовсе — гард
 * deep-link показывает своё состояние сразу, а не после трёх волн с
 * экспоненциальными паузами. 5xx и сетевые сбои (ApiError без статуса)
 * ретраются по умолчанию react-query — до трёх попыток.
 */
export function queryRetry(failureCount: number, error: unknown): boolean {
  if (
    error instanceof ApiError
    && error.status !== undefined
    && error.status >= 400
    && error.status < 500
  ) {
    return false;
  }
  return failureCount < 3;
}
