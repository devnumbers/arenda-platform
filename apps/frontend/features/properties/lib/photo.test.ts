import { describe, expect, it } from 'vitest';
import {
  PHOTO_FILE_MAX_BYTES,
  type PropertyPhotoStage,
  photoDisplayUrl,
  photoFileTooLarge,
  propertyPhotoDisplay,
} from './photo';

describe('photoFileTooLarge — клиентский пречек капа 5 МиБ (ADR 0065)', () => {
  it('ровно кап проходит — бэкенд отвергает только превышение', () => {
    expect(photoFileTooLarge(PHOTO_FILE_MAX_BYTES)).toBe(false);
  });

  it('байт сверх капа отвергается до похода в сеть', () => {
    expect(photoFileTooLarge(PHOTO_FILE_MAX_BYTES + 1)).toBe(true);
  });
});

describe('photoDisplayUrl — бастер браузерного кэша выдачи (private max-age=300, ADR 0065)', () => {
  it('без мутаций URL канонический, без параметра', () => {
    expect(photoDisplayUrl('/api/v1/properties/p1/photo', 0)).toBe(
      '/api/v1/properties/p1/photo',
    );
  });

  it('после мутации версия едет параметром — <img> перезапрашивает байты', () => {
    expect(photoDisplayUrl('/api/v1/properties/p1/photo', 2)).toBe(
      '/api/v1/properties/p1/photo?v=2',
    );
  });
});

describe('propertyPhotoDisplay — правило показа фото с незавершённым стейджем (решение владельца 08.10)', () => {
  const serverUrl = '/api/v1/properties/p1/photo';
  const stagedFile: PropertyPhotoStage = {
    kind: 'file',
    file: new File([], 'new.png', { type: 'image/png' }),
    previewUrl: 'blob:preview-1',
  };

  it('staged-файл перекрывает серверное фото — в круге локальное превью', () => {
    expect(propertyPhotoDisplay(serverUrl, 3, stagedFile)).toBe('blob:preview-1');
  });

  it('staged-файл на объекте без фото — превью и без серверного пути', () => {
    expect(propertyPhotoDisplay(null, 0, stagedFile)).toBe('blob:preview-1');
  });

  it('staged-удаление прячет фото, даже пока серверное живо', () => {
    expect(propertyPhotoDisplay(serverUrl, 3, { kind: 'remove' })).toBeNull();
  });

  it('без стейджа — серверное фото с бастером', () => {
    expect(propertyPhotoDisplay(serverUrl, 3, null)).toBe(`${serverUrl}?v=3`);
  });

  it('без стейджа и без серверного фото — глиф-плейсхолдер', () => {
    expect(propertyPhotoDisplay(null, 0, null)).toBeNull();
  });
});
