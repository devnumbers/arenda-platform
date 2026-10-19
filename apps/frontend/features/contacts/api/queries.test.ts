import { beforeEach, describe, expect, it, vi } from 'vitest';

// Транспорт подменяется стабами, ловящими вызовы: тест пинит wire-контракт
// фото-пары (ADR 0065) и маппинг ответа. POST фото идёт через XHR-транспорт
// uploadFile (план 036 — fetch не отдаёт upload-progress), DELETE — через
// apiClient; wire-контракт XHR пинит upload.test.ts, здесь — делегация и
// маппинг. Рендер хука в node-окружении недоступен — функции зовутся напрямую.
vi.mock('@/shared/api/client', () => ({ apiClient: vi.fn() }));
vi.mock('@/shared/api/upload', () => ({ uploadFile: vi.fn() }));

import { apiClient } from '@/shared/api/client';
import { uploadFile } from '@/shared/api/upload';
import type { components } from '@/shared/api/dto';
import { deleteContactPhoto, uploadContactPhoto } from './queries';

const apiClientMock = vi.mocked(apiClient);
const uploadFileMock = vi.mocked(uploadFile);

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

describe('uploadContactPhoto — POST /contacts/{id}/photo (XHR, ADR 0065)', () => {
  beforeEach(() => {
    uploadFileMock.mockReset();
    uploadFileMock.mockResolvedValue(
      makeDto({ photoUrl: '/api/contacts/contact-1/photo' }),
    );
  });

  it('делегирует в uploadFile фото-путём, файлом и прогрессом выгрузки', async () => {
    const onProgress = (): void => undefined;
    await uploadContactPhoto({ id: 'contact-1', file, onProgress });

    expect(uploadFileMock).toHaveBeenCalledTimes(1);
    expect(uploadFileMock).toHaveBeenCalledWith('/contacts/contact-1/photo', file, onProgress);
  });

  it('ответ маппится в сущность — photoUrl остаётся photoUrl', async () => {
    const contact = await uploadContactPhoto({ id: 'contact-1', file });

    expect(contact.id).toBe('contact-1');
    expect(contact.photoUrl).toBe('/api/contacts/contact-1/photo');
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
