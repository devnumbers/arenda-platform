import { describe, expect, it } from 'vitest';
import { ApiError } from '@/shared/api/errors';
import { resolvePropertyDetailError } from './resolve-property-detail-error';

describe('resolvePropertyDetailError', () => {
  it('returns not_found for ApiError with status 404', () => {
    const error = new ApiError('not_found', 'Объект не найден', undefined, 404);
    expect(resolvePropertyDetailError(error)).toBe('not_found');
  });

  it('returns suspended for 403 with code membership_suspended', () => {
    const error = new ApiError('membership_suspended', 'Доступ приостановлен', undefined, 403);
    expect(resolvePropertyDetailError(error)).toBe('suspended');
  });

  it('returns generic for 403 with another code', () => {
    const error = new ApiError('forbidden', 'Нет доступа', undefined, 403);
    expect(resolvePropertyDetailError(error)).toBe('generic');
  });

  it('returns generic for 500', () => {
    const error = new ApiError('internal_error', 'Ошибка сервера', undefined, 500);
    expect(resolvePropertyDetailError(error)).toBe('generic');
  });

  it('returns generic for network error without status', () => {
    const error = new ApiError('network_error', 'Нет соединения');
    expect(resolvePropertyDetailError(error)).toBe('generic');
  });

  it('returns generic for non-ApiError values', () => {
    expect(resolvePropertyDetailError(new Error('boom'))).toBe('generic');
    expect(resolvePropertyDetailError(undefined)).toBe('generic');
    expect(resolvePropertyDetailError('boom')).toBe('generic');
  });
});
