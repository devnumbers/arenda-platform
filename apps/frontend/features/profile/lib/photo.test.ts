import { describe, expect, it } from 'vitest';
import { type MePhotoStage, mePhotoDisplay } from './photo';

describe('mePhotoDisplay — правило показа с летящей мутацией (мгновенное применение, решение владельца #1230)', () => {
  const serverUrl = '/api/me/photo';
  const uploading: MePhotoStage = {
    kind: 'file',
    file: new File([], 'new.png', { type: 'image/png' }),
    previewUrl: 'data:image/png;base64,preview-1',
  };

  it('превью летящей загрузки перекрывает серверное фото — круг меняется сразу', () => {
    expect(mePhotoDisplay(serverUrl, 3, uploading)).toBe(
      'data:image/png;base64,preview-1',
    );
  });

  it('превью первой загрузки (серверного фото ещё нет)', () => {
    expect(mePhotoDisplay(null, 0, uploading)).toBe(
      'data:image/png;base64,preview-1',
    );
  });

  it('без мутации — серверное фото с бастером', () => {
    expect(mePhotoDisplay(serverUrl, 3, null)).toBe(`${serverUrl}?v=3`);
  });

  it('без мутации и без серверного фото — глиф-плейсхолдер', () => {
    expect(mePhotoDisplay(null, 0, null)).toBeNull();
  });
});
