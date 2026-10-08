import { beforeEach, describe, expect, it, vi } from 'vitest';

// Транспорт подменяется стабом, ловящим вызов: тест пинит wire-контракт
// фото-пары (multipart POST + DELETE, ADR 0065) и маппинг ответа. Рендер
// хука в node-окружении недоступен — функции зовутся напрямую.
vi.mock('@/shared/api/client', () => ({ apiClient: vi.fn() }));

import { apiClient } from '@/shared/api/client';
import type { components } from '@/shared/api/dto';
import { deletePropertyPhoto, uploadPropertyPhoto } from './queries';

const apiClientMock = vi.mocked(apiClient);

function makeDto(
  overrides: Partial<components['schemas']['PropertyResponse']> = {},
): components['schemas']['PropertyResponse'] {
  return {
    id: 'prop-1',
    name: 'Гараж',
    type: 'garage',
    address: 'Екатеринбург, ул. Мамина, 4',
    attributes: {},
    status: 'active',
    members_count: 0,
    pinned_at: null,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

/** Файл-заглушка: хелпер-функции читают только идентичность и размер. */
const file = new File(['bytes'], 'photo.jpg', { type: 'image/jpeg' });

describe('uploadPropertyPhoto — POST /properties/{id}/photo (multipart, ADR 0065)', () => {
  beforeEach(() => {
    apiClientMock.mockReset();
    apiClientMock.mockResolvedValue(makeDto({ photo_url: '/api/v1/properties/prop-1/photo' }));
  });

  it('уходит multipart-формой с полем file на фото-путь объекта', async () => {
    await uploadPropertyPhoto({ id: 'prop-1', file });

    expect(apiClientMock).toHaveBeenCalledTimes(1);
    const [path, init] = apiClientMock.mock.calls[0] ?? [];
    expect(path).toBe('/properties/prop-1/photo');
    expect(init?.method).toBe('POST');
    const body = init?.body;
    expect(body).toBeInstanceOf(FormData);
    if (body instanceof FormData) {
      expect(body.get('file')).toBe(file);
    }
  });

  it('ответ маппится в сущность — photo_url становится photoUrl', async () => {
    const property = await uploadPropertyPhoto({ id: 'prop-1', file });

    expect(property.id).toBe('prop-1');
    expect(property.photoUrl).toBe('/api/v1/properties/prop-1/photo');
  });
});

describe('deletePropertyPhoto — DELETE /properties/{id}/photo (ADR 0065)', () => {
  beforeEach(() => {
    apiClientMock.mockReset();
    apiClientMock.mockResolvedValue(undefined);
  });

  it('DELETE на тот же путь, тело пустое', async () => {
    await deletePropertyPhoto({ id: 'prop-1' });

    expect(apiClientMock).toHaveBeenCalledTimes(1);
    const [path, init] = apiClientMock.mock.calls[0] ?? [];
    expect(path).toBe('/properties/prop-1/photo');
    expect(init?.method).toBe('DELETE');
    expect(init?.body).toBeUndefined();
  });
});
