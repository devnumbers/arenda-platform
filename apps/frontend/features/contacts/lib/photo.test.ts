import { describe, expect, it } from 'vitest';
import {
  type ContactPhotoStage,
  contactPhotoDisplay,
  contactPhotoStageAfterRemove,
} from './photo';

describe('contactPhotoDisplay — правило показа фото с незавершённым стейджем (канон #1228)', () => {
  const serverUrl = '/api/v1/contacts/c1/photo';
  const stagedFile: ContactPhotoStage = {
    kind: 'file',
    file: new File([], 'new.png', { type: 'image/png' }),
    previewUrl: 'data:image/png;base64,preview-1',
  };

  it('staged-файл перекрывает серверное фото — в круге локальное превью', () => {
    expect(contactPhotoDisplay(serverUrl, 3, stagedFile)).toBe(
      'data:image/png;base64,preview-1',
    );
  });

  it('staged-файл на карточке без фото — превью и без серверного пути', () => {
    expect(contactPhotoDisplay(null, 0, stagedFile)).toBe(
      'data:image/png;base64,preview-1',
    );
  });

  it('staged-удаление прячет фото, даже пока серверное живо', () => {
    expect(contactPhotoDisplay(serverUrl, 3, { kind: 'remove' })).toBeNull();
  });

  it('без стейджа — серверное фото с бастером', () => {
    expect(contactPhotoDisplay(serverUrl, 3, null)).toBe(`${serverUrl}?v=3`);
  });

  it('без стейджа и без серверного фото — глиф-плейсхолдер', () => {
    expect(contactPhotoDisplay(null, 0, null)).toBeNull();
  });
});

describe('contactPhotoStageAfterRemove — stage после подтверждения удаления (бэк отвечает 404 на удаление несуществующего)', () => {
  it('серверное фото живо — запланировано удаление, применится сабмитом', () => {
    expect(contactPhotoStageAfterRemove('/api/v1/contacts/c1/photo')).toEqual({
      kind: 'remove',
    });
  });

  it('серверного фото нет (создание или staged-файл) — stage сбрасывается, сервер не трогается', () => {
    expect(contactPhotoStageAfterRemove(null)).toBeNull();
  });
});
