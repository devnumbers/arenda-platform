/**
 * Клиентские хелперы фото контакта (ADR 0065, тикет #1229): слоеное
 * правило показа с незавершённым изменением черновика формы — зеркало
 * фото-механики объекта (#1228, решение владельца 08.10). Кап файла и
 * бастер кэша выдачи — общие хелперы shared/lib/photo.
 */

import { photoDisplayUrl } from '@/shared/lib/photo';

/**
 * Незавершённое изменение фото в черновике формы: применяется кнопками
 * сохранения, а не сразу — stage держит выбранную замену (`file` с
 * локальным data-URL превью; CSP `img-src 'self' data:` из ADR 0065
 * покрывает его без расширений) либо запланированное удаление
 * (`remove`). Уход без сохранения stage просто умирает вместе с формой,
 * сервер не тронут. При создании карточки фото уходит загрузкой сразу
 * после POST /contacts — прозрачно внутри того же сабмита.
 */
export type ContactPhotoStage =
  | { readonly kind: 'file'; readonly file: File; readonly previewUrl: string }
  | { readonly kind: 'remove' };

/**
 * Что показывать в круге: staged-превью (замена/загрузка ещё не
 * применены), ничто (staged-удаление поверх живого серверного фото)
 * либо серверное фото с бастером. null — BoldUser-плейсхолдер.
 */
export function contactPhotoDisplay(
  serverPhotoUrl: string | null,
  buster: number,
  stage: ContactPhotoStage | null,
): string | null {
  if (stage?.kind === 'file') {
    return stage.previewUrl;
  }
  if (stage?.kind === 'remove') {
    return null;
  }
  return serverPhotoUrl !== null ? photoDisplayUrl(serverPhotoUrl, buster) : null;
}

/**
 * Stage, который подтверждение удаления оставляет в черновике: при живом
 * серверном фото — запланированное удаление; без серверного фото (карточка
 * создаётся либо удалён лишь staged-файл) сбрасывать нечего — stage
 * обнуляется, DELETE бэк не дергает (он отвечает 404 на удаление
 * несуществующего — ErrPhotoNotFound).
 */
export function contactPhotoStageAfterRemove(
  serverPhotoUrl: string | null,
): ContactPhotoStage | null {
  return serverPhotoUrl === null ? null : { kind: 'remove' };
}
