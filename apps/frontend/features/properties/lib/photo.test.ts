import { describe, expect, it } from 'vitest';
import { type PropertyPhotoStage, propertyPhotoDisplay } from './photo';

describe('propertyPhotoDisplay — правило показа фото с незавершённым стейджем (решение владельца 08.10)', () => {
  // URL выдачи версионирован бэком (?v= по ключу) — фронт не строит его.
  const serverUrl = '/api/properties/p1/photo?v=8fe0a9930afe3eabb4560c5db3f4da44';
  const stagedFile: PropertyPhotoStage = {
    kind: 'file',
    file: new File([], 'new.png', { type: 'image/png' }),
    previewUrl: 'blob:preview-1',
  };

  it('staged-файл перекрывает серверное фото — в круге локальное превью', () => {
    expect(propertyPhotoDisplay(serverUrl, stagedFile)).toBe('blob:preview-1');
  });

  it('staged-файл на объекте без фото — превью и без серверного пути', () => {
    expect(propertyPhotoDisplay(null, stagedFile)).toBe('blob:preview-1');
  });

  it('staged-удаление прячет фото, даже пока серверное живо', () => {
    expect(propertyPhotoDisplay(serverUrl, { kind: 'remove' })).toBeNull();
  });

  it('без стейджа — серверное фото как есть (версия уже в URL)', () => {
    expect(propertyPhotoDisplay(serverUrl, null)).toBe(serverUrl);
  });

  it('без стейджа и без серверного фото — глиф-плейсхолдер', () => {
    expect(propertyPhotoDisplay(null, null)).toBeNull();
  });
});
