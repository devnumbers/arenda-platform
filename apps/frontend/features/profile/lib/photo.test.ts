import { describe, expect, it } from 'vitest';
import {
  type MePhotoStage,
  mePhotoDisplay,
  mePhotoStageAfterRemove,
} from './photo';

describe('mePhotoDisplay — правило показа фото с незавершённым стейджем (канон #1228)', () => {
  const serverUrl = '/api/v1/me/photo';
  const stagedFile: MePhotoStage = {
    kind: 'file',
    file: new File([], 'new.png', { type: 'image/png' }),
    previewUrl: 'data:image/png;base64,preview-1',
  };

  it('staged-файл перекрывает серверное фото — в круге локальное превью', () => {
    expect(mePhotoDisplay(serverUrl, 3, stagedFile)).toBe(
      'data:image/png;base64,preview-1',
    );
  });

  it('staged-файл на профиле без фото — превью и без серверного пути', () => {
    expect(mePhotoDisplay(null, 0, stagedFile)).toBe(
      'data:image/png;base64,preview-1',
    );
  });

  it('staged-удаление прячет фото, даже пока серверное живо', () => {
    expect(mePhotoDisplay(serverUrl, 3, { kind: 'remove' })).toBeNull();
  });

  it('без стейджа — серверное фото с бастером', () => {
    expect(mePhotoDisplay(serverUrl, 3, null)).toBe(`${serverUrl}?v=3`);
  });

  it('без стейджа и без серверного фото — глиф-плейсхолдер', () => {
    expect(mePhotoDisplay(null, 0, null)).toBeNull();
  });
});

describe('mePhotoStageAfterRemove — stage после подтверждения удаления (бэк отвечает 404 на удаление несуществующего)', () => {
  it('серверное фото живо — запланировано удаление, применится коммитом формы', () => {
    expect(mePhotoStageAfterRemove('/api/v1/me/photo')).toEqual({
      kind: 'remove',
    });
  });

  it('серверного фото нет (только staged-файл) — stage сбрасывается, сервер не трогается', () => {
    expect(mePhotoStageAfterRemove(null)).toBeNull();
  });
});
