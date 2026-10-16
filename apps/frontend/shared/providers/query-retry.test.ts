import { describe, expect, it } from 'vitest';
import { ApiError } from '../api/errors';
import { queryRetry } from './query-retry';

describe('queryRetry', () => {
  it('does not retry the suspended-access guard error', () => {
    const suspended = new ApiError(
      'membership_suspended',
      'Доступ приостановлен',
      'req-1',
      403,
    );
    expect(queryRetry(0, suspended)).toBe(false);
  });

  it.each([400, 401, 403, 404, 409, 429])(
    'does not retry a %i ApiError',
    (status) => {
      const error = new ApiError('code', 'detail', 'req-1', status);
      expect(queryRetry(0, error)).toBe(false);
    },
  );

  it('retries a 5xx ApiError up to the default three attempts', () => {
    const error = new ApiError('internal', 'Ошибка сервера (код 500)', 'req-1', 500);
    expect(queryRetry(0, error)).toBe(true);
    expect(queryRetry(1, error)).toBe(true);
    expect(queryRetry(2, error)).toBe(true);
    expect(queryRetry(3, error)).toBe(false);
  });

  it('retries a network failure without a status', () => {
    const network = new ApiError('network_error', 'fetch failed', undefined, undefined);
    expect(queryRetry(0, network)).toBe(true);
    expect(queryRetry(3, network)).toBe(false);
  });

  it('retries an error that is not an ApiError', () => {
    expect(queryRetry(0, new Error('boom'))).toBe(true);
    expect(queryRetry(3, new Error('boom'))).toBe(false);
  });
});
