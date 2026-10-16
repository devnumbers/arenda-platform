import { describe, expect, it } from 'vitest';
import { ApiError } from './errors';
import { nullOn404 } from './null-on-404';

describe('nullOn404', () => {
  it('returns the fetched value as is', async () => {
    const value = { status: 'active' };
    await expect(nullOn404(() => Promise.resolve(value))).resolves.toBe(value);
  });

  it('maps a 404 ApiError to null', async () => {
    const notFound = new ApiError('not_found', 'Подписка не найдена', 'req-1', 404);
    await expect(nullOn404(() => Promise.reject(notFound))).resolves.toBeNull();
  });

  it('rethrows a non-404 ApiError', async () => {
    const forbidden = new ApiError('forbidden', 'Нет доступа', 'req-1', 403);
    await expect(nullOn404(() => Promise.reject(forbidden))).rejects.toBe(forbidden);
  });

  it('rethrows an error that is not an ApiError', async () => {
    const network = new Error('fetch failed');
    await expect(nullOn404(() => Promise.reject(network))).rejects.toBe(network);
  });
});
