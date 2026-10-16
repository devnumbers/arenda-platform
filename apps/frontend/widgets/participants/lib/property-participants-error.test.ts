import { describe, expect, it } from 'vitest';
import { ApiError } from '@/shared/api/errors';
import { resolvePropertyParticipantsError } from './property-participants-error';

// Краевые #719: отзыв или приостановка своего доступа в открытой сессии —
// перечитывание списка участников отвечает 404 (бэк скрывает объект как
// «не найден», policy.CanView), экран обязан показать аккуратный
// not-found-экран, а не generic-ошибку с бесполезным «Повторить».
describe('resolvePropertyParticipantsError', () => {
  it('returns not_found for ApiError with status 404', () => {
    const error = new ApiError('not_found', 'Объект не найден', undefined, 404);
    expect(resolvePropertyParticipantsError(error)).toBe('not_found');
  });

  it('returns generic for 403', () => {
    const error = new ApiError('forbidden', 'Нет доступа', undefined, 403);
    expect(resolvePropertyParticipantsError(error)).toBe('generic');
  });

  it('returns generic for 500', () => {
    const error = new ApiError('internal_error', 'Ошибка сервера', undefined, 500);
    expect(resolvePropertyParticipantsError(error)).toBe('generic');
  });

  it('returns generic for network error without status', () => {
    const error = new ApiError('network_error', 'Нет соединения');
    expect(resolvePropertyParticipantsError(error)).toBe('generic');
  });

  it('returns generic for non-ApiError values', () => {
    expect(resolvePropertyParticipantsError(new Error('boom'))).toBe('generic');
    expect(resolvePropertyParticipantsError(undefined)).toBe('generic');
    expect(resolvePropertyParticipantsError('boom')).toBe('generic');
  });
});
