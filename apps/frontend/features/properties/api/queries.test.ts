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
import { deletePropertyPhoto, uploadPropertyPhoto } from './queries';

const apiClientMock = vi.mocked(apiClient);
const uploadFileMock = vi.mocked(uploadFile);

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
    pinned_at: null,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

/** Файл-заглушка: хелпер-функции читают только идентичность и размер. */
const file = new File(['bytes'], 'photo.jpg', { type: 'image/jpeg' });

describe('uploadPropertyPhoto — POST /properties/{id}/photo (XHR, ADR 0065)', () => {
  beforeEach(() => {
    uploadFileMock.mockReset();
    uploadFileMock.mockResolvedValue(makeDto({ photo_url: '/api/properties/prop-1/photo' }));
  });

  it('делегирует в uploadFile фото-путём, файлом и прогрессом выгрузки', async () => {
    const onProgress = (): void => undefined;
    await uploadPropertyPhoto({ id: 'prop-1', file, onProgress });

    expect(uploadFileMock).toHaveBeenCalledTimes(1);
    expect(uploadFileMock).toHaveBeenCalledWith('/properties/prop-1/photo', file, onProgress);
  });

  it('ответ маппится в сущность — photo_url становится photoUrl', async () => {
    const property = await uploadPropertyPhoto({ id: 'prop-1', file });

    expect(property.id).toBe('prop-1');
    expect(property.photoUrl).toBe('/api/properties/prop-1/photo');
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
