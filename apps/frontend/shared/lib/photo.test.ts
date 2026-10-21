import { describe, expect, it } from 'vitest';
import { PHOTO_FILE_MAX_BYTES, photoFileTooLarge } from './photo';

describe('photoFileTooLarge — клиентский пречек капа 5 МиБ (ADR 0065)', () => {
  it('ровно кап проходит — бэкенд отвергает только превышение', () => {
    expect(photoFileTooLarge(PHOTO_FILE_MAX_BYTES)).toBe(false);
  });

  it('байт сверх капа отвергается до похода в сеть', () => {
    expect(photoFileTooLarge(PHOTO_FILE_MAX_BYTES + 1)).toBe(true);
  });
});
