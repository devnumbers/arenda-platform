/**
 * Клиентские хелперы фото профиля (ADR 0065, тикет #1230): слоеное
 * правило показа с незавершённым изменением черновика экрана — зеркало
 * фото-механики объекта и контакта (#1228/#1229, решения владельца
 * 08.10). Кап файла и бастер кэша выдачи — общие хелперы shared/lib/photo.
 */

import { photoDisplayUrl } from '@/shared/lib/photo';

/**
 * Незавершённое изменение фото в черновике экрана аккаунта: применяется
 * коммитом формы, а не сразу — stage держит выбранную замену (`file` с
 * локальным data-URL превью; CSP `img-src 'self' data:` из ADR 0065
 * покрывает его без расширений) либо запланированное удаление
 * (`remove`). Уход без коммита stage просто умирает вместе с экраном,
 * сервер не тронут.
 */
export type MePhotoStage =
  | { readonly kind: 'file'; readonly file: File; readonly previewUrl: string }
  | { readonly kind: 'remove' };

/**
 * Что показывать в круге: staged-превью (замена/загрузка ещё не
 * применены), ничто (staged-удаление поверх живого серверного фото)
 * либо серверное фото с бастером. null — глиф-плейсхолдер.
 */
export function mePhotoDisplay(
  serverPhotoUrl: string | null,
  buster: number,
  stage: MePhotoStage | null,
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
 * серверном фото — запланированное удаление; без серверного фото (удалён
 * лишь staged-файл) сбрасывать нечего — stage обнуляется, DELETE бэк не
 * дергает (он отвечает 404 на удаление несуществующего —
 * ErrPhotoNotFound).
 */
export function mePhotoStageAfterRemove(
  serverPhotoUrl: string | null,
): MePhotoStage | null {
  return serverPhotoUrl === null ? null : { kind: 'remove' };
}
