/**
 * Клиентские хелперы фото профиля (ADR 0065, тикет #1230): правило показа
 * с летящей мутацией. Экран аккаунта — автосейв-форма, изменения
 * применяются сразу (решение владельца: «при изменении фото оно сразу
 * должно меняться» — пересмотр стейдж-канона #1228 для этого экрана):
 * stage живёт только пока загрузка летит, держа data-URL превью в круге
 * (CSP `img-src 'self' data:`, ADR 0065); ответ/отказ гасят его. Кап
 * файла и бастер кэша выдачи — общие хелперы shared/lib/photo.
 */

import { photoDisplayUrl } from '@/shared/lib/photo';

/**
 * Летящая загрузка фото профиля: data-URL превью выбранного файла держит
 * круг, пока POST не ответил. Сбой гасит stage — круг возвращается к
 * серверному состоянию (решение владельца: тост отказа, ретрай —
 * повторный выбор), нависшего превью без момента сохранения здесь нет.
 */
export type MePhotoStage = {
  readonly kind: 'file';
  readonly file: File;
  readonly previewUrl: string;
};

/**
 * Что показывать в круге: превью летящей загрузки либо серверное фото с
 * бастером. null — глиф-плейсхолдер.
 */
export function mePhotoDisplay(
  serverPhotoUrl: string | null,
  buster: number,
  stage: MePhotoStage | null,
): string | null {
  if (stage?.kind === 'file') {
    return stage.previewUrl;
  }
  return serverPhotoUrl !== null ? photoDisplayUrl(serverPhotoUrl, buster) : null;
}
