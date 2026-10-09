import { beforeEach, describe, expect, it, vi } from 'vitest';

// Транспорт подменяется стабом, ловящим вызов: тест пинит wire-контракт
// фото-пары (multipart POST + DELETE, ADR 0065) и маппинг ответа. Рендер
// хука в node-окружении недоступен — функции зовутся напрямую.
vi.mock('@/shared/api/client', () => ({ apiClient: vi.fn() }));

import { apiClient } from '@/shared/api/client';
import type { components } from '@/shared/api/dto';
import { deleteContactPhoto, uploadContactPhoto } from './queries';

const apiClientMock = vi.mocked(apiClient);

function makeDto(
  overrides: Partial<components['schemas']['ContactResponse']> = {},
): components['schemas']['ContactResponse'] {
  return {
    id: 'contact-1',
    propertyId: null,
    photoUrl: null,
    firstName: 'Александр',
    lastName: 'Иванов',
    patronymic: '',
    role: 'Сантехник',
    phone: '',
    email: '',
    messengerName: '',
    messengerUsername: '',
    note: '',
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

/** Файл-заглушка: хелпер-функции читают только идентичность и размер. */
const file = new File(['bytes'], 'photo.jpg', { type: 'image/jpeg' });

describe('uploadContactPhoto — POST /contacts/{id}/photo (multipart, ADR 0065)', () => {
  beforeEach(() => {
    apiClientMock.mockReset();
    apiClientMock.mockResolvedValue(
      makeDto({ photoUrl: '/api/v1/contacts/contact-1/photo' }),
    );
  });

  it('уходит multipart-формой с полем file на фото-путь карточки', async () => {
    await uploadContactPhoto({ id: 'contact-1', file });

    expect(apiClientMock).toHaveBeenCalledTimes(1);
    const [path, init] = apiClientMock.mock.calls[0] ?? [];
    expect(path).toBe('/contacts/contact-1/photo');
    expect(init?.method).toBe('POST');
    const body = init?.body;
    expect(body).toBeInstanceOf(FormData);
    if (body instanceof FormData) {
      expect(body.get('file')).toBe(file);
    }
  });

  it('ответ маппится в сущность — photo_url становится photoUrl', async () => {
    const contact = await uploadContactPhoto({ id: 'contact-1', file });

    expect(contact.id).toBe('contact-1');
    expect(contact.photoUrl).toBe('/api/v1/contacts/contact-1/photo');
  });
});

describe('deleteContactPhoto — DELETE /contacts/{id}/photo (ADR 0065)', () => {
  beforeEach(() => {
    apiClientMock.mockReset();
    apiClientMock.mockResolvedValue(undefined);
  });

  it('DELETE на тот же путь, тело пустое', async () => {
    await deleteContactPhoto({ id: 'contact-1' });

    expect(apiClientMock).toHaveBeenCalledTimes(1);
    const [path, init] = apiClientMock.mock.calls[0] ?? [];
    expect(path).toBe('/contacts/contact-1/photo');
    expect(init?.method).toBe('DELETE');
    expect(init?.body).toBeUndefined();
  });
});
