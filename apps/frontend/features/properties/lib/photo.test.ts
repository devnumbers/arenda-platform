import { describe, expect, it } from 'vitest';
import { type PropertyPhotoStage, propertyPhotoDisplay } from './photo';

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
