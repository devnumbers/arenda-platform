import { describe, expect, it } from 'vitest';
import {
  type ContactPhotoStage,
  contactPhotoDisplay,
  contactPhotoStageAfterRemove,
} from './photo';

describe('contactPhotoDisplay — правило показа фото с незавершённым стейджем (канон #1228)', () => {
  // URL выдачи версионирован бэком (?v= по ключу) — фронт не строит его.
  const serverUrl = '/api/contacts/c1/photo?v=8fe0a9930afe3eabb4560c5db3f4da44';
  const stagedFile: ContactPhotoStage = {
    kind: 'file',
    file: new File([], 'new.png', { type: 'image/png' }),
    previewUrl: 'data:image/png;base64,preview-1',
  };

  it('staged-файл перекрывает серверное фото — в круге локальное превью', () => {
    expect(contactPhotoDisplay(serverUrl, stagedFile)).toBe(
      'data:image/png;base64,preview-1',
    );
  });

  it('staged-файл на карточке без фото — превью и без серверного пути', () => {
    expect(contactPhotoDisplay(null, stagedFile)).toBe(
      'data:image/png;base64,preview-1',
    );
  });

  it('staged-удаление прячет фото, даже пока серверное живо', () => {
    expect(contactPhotoDisplay(serverUrl, { kind: 'remove' })).toBeNull();
  });

  it('без стейджа — серверное фото как есть (версия уже в URL)', () => {
    expect(contactPhotoDisplay(serverUrl, null)).toBe(serverUrl);
  });

  it('без стейджа и без серверного фото — глиф-плейсхолдер', () => {
    expect(contactPhotoDisplay(null, null)).toBeNull();
  });
});

describe('contactPhotoStageAfterRemove — stage после подтверждения удаления (бэк отвечает 404 на удаление несуществующего)', () => {
  it('серверное фото живо — запланировано удаление, применится сабмитом', () => {
    expect(contactPhotoStageAfterRemove('/api/contacts/c1/photo')).toEqual({
      kind: 'remove',
    });
  });

  it('серверного фото нет (создание или staged-файл) — stage сбрасывается, сервер не трогается', () => {
    expect(contactPhotoStageAfterRemove(null)).toBeNull();
  });
});
