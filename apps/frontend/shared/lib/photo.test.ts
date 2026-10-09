import { describe, expect, it } from 'vitest';
import {
  PHOTO_FILE_MAX_BYTES,
  photoDisplayUrl,
  photoFileTooLarge,
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
    expect(photoDisplayUrl('/api/contacts/c1/photo', 0)).toBe(
      '/api/contacts/c1/photo',
    );
  });

  it('после мутации версия едет параметром — <img> перезапрашивает байты', () => {
    expect(photoDisplayUrl('/api/contacts/c1/photo', 2)).toBe(
      '/api/contacts/c1/photo?v=2',
    );
  });
});
