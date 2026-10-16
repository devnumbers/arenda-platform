/**
 * Клиентские хелперы фото объекта (ADR 0065, тикет #1228): слоеное
 * правило показа с незавершённым изменением черновика формы. Кап файла
 * и бастер кэша выдачи — общие для сущностей, живут в shared/lib/photo.
 */

import { photoDisplayUrl } from '@/shared/lib/photo';

/**
 * Незавершённое изменение фото в черновике формы (решение владельца
 * 08.10): фото применяется кнопками сохранения, а не сразу — stage
 * держит выбранную замену (`file` с локальным data-URL превью; CSP
 * `img-src 'self' data:` из ADR 0065 покрывает его без расширений,
 * blob:-превью она бы заблокировала) либо запланированное удаление
 * (`remove`). Уход без сохранения stage просто умирает вместе с формой,
 * сервер не тронут.
 */
export type PropertyPhotoStage =
  | { readonly kind: 'file'; readonly file: File; readonly previewUrl: string }
  | { readonly kind: 'remove' };

/**
 * Что показывать в круге: staged-превью (замена/загрузка ещё не
 * применены), ничто (staged-удаление поверх живого серверного фото)
 * либо серверное фото с бастером. null — глиф-плейсхолдер.
 */
export function propertyPhotoDisplay(
  serverPhotoUrl: string | null,
  buster: number,
  stage: PropertyPhotoStage | null,
): string | null {
  if (stage?.kind === 'file') {
    return stage.previewUrl;
  }
  if (stage?.kind === 'remove') {
    return null;
  }
  return serverPhotoUrl !== null ? photoDisplayUrl(serverPhotoUrl, buster) : null;
}
