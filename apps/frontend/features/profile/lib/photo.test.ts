import { describe, expect, it } from 'vitest';
import { type MePhotoStage, mePhotoDisplay } from './photo';

describe('mePhotoDisplay — правило показа с летящей мутацией (мгновенное применение, решение владельца #1230)', () => {
  // URL выдачи версионирован бэком (?v= по ключу) — фронт не строит его.
  const serverUrl = '/api/me/photo?v=8fe0a9930afe3eabb4560c5db3f4da44';
  const uploading: MePhotoStage = {
    kind: 'file',
    file: new File([], 'new.png', { type: 'image/png' }),
    previewUrl: 'data:image/png;base64,preview-1',
  };

  it('превью летящей загрузки перекрывает серверное фото — круг меняется сразу', () => {
    expect(mePhotoDisplay(serverUrl, uploading)).toBe(
      'data:image/png;base64,preview-1',
    );
  });

  it('превью первой загрузки (серверного фото ещё нет)', () => {
    expect(mePhotoDisplay(null, uploading)).toBe(
      'data:image/png;base64,preview-1',
    );
  });

  it('без мутации — серверное фото как есть (версия уже в URL)', () => {
    expect(mePhotoDisplay(serverUrl, null)).toBe(serverUrl);
  });

  it('без мутации и без серверного фото — глиф-плейсхолдер', () => {
    expect(mePhotoDisplay(null, null)).toBeNull();
  });

  it('провал прошлой попытки (failed) дисплей не меняет — превью держится (план 037: файл переживает провал)', () => {
    const failed: MePhotoStage = { ...uploading, failed: true };
    expect(mePhotoDisplay(serverUrl, failed)).toBe('data:image/png;base64,preview-1');
    expect(mePhotoDisplay(null, failed)).toBe('data:image/png;base64,preview-1');
  });
});
