import { describe, expect, it } from 'vitest';
import { ApiError } from '@/shared/api/errors';
import { isPropertyLimitReachedError } from './property-limit-gate';

describe('isPropertyLimitReachedError', () => {
  it('закодированный 402 property_limit_reached — гейт лимита', () => {
    expect(
      isPropertyLimitReachedError(
        new ApiError(
          'property_limit_reached',
          'Превышен лимит активных объектов',
          'req-1',
          402,
        ),
      ),
    ).toBe(true);
  });

  it('402 без кода маскируется в генерик-ошибку (канон #1452)', () => {
    expect(
      isPropertyLimitReachedError(
        new ApiError('unknown', 'Ошибка сервера (код 402)', undefined, 402),
      ),
    ).toBe(false);
  });

  it('другой машинный код 402 (tariff_required) — не лимит объектов', () => {
    expect(
      isPropertyLimitReachedError(new ApiError('tariff_required', 'Нужен тариф', undefined, 402)),
    ).toBe(false);
  });

  it('тот же код на другом статусе — не гейт лимита', () => {
    expect(
      isPropertyLimitReachedError(
        new ApiError('property_limit_reached', 'Превышен лимит', undefined, 409),
      ),
    ).toBe(false);
  });

  it('не ApiError — не гейт лимита', () => {
    expect(isPropertyLimitReachedError(new Error('boom'))).toBe(false);
    expect(isPropertyLimitReachedError(undefined)).toBe(false);
  });
});
