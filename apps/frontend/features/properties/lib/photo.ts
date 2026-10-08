/**
 * Клиентские хелперы фото объекта (ADR 0065, тикет #1228): зеркала
 * серверного контракта для мгновенной обратной связи и бастер кэша
 * выдачи. Чистые функции без сети — решение о валидности остаётся за
 * бэкендом (магические байты), пречек только отсекает заведомо
 * отвергнутое до загрузки.
 */

/** Кап размера файла — зеркало бэкенда (`photo.ErrTooLarge`, «Файл
 * больше 5 МиБ»): ровно кап бэкенд пропускает, отвергается превышение. */
export const PHOTO_FILE_MAX_BYTES = 5 * 1024 * 1024;

/** Тост пречека — тот же текст, что у problem+json бэка для ErrTooLarge. */
export const PHOTO_FILE_TOO_LARGE_MESSAGE = 'Файл больше 5 МиБ';

export function photoFileTooLarge(size: number): boolean {
  return size > PHOTO_FILE_MAX_BYTES;
}

/**
 * URL фото для <img> с бастером браузерного кэша: выдача стримится с
 * `Cache-Control: private, max-age=300` (ADR 0065), путь при замене и
 * удалении не меняется — без бастера <img> показал бы старые байты из
 * кэша до 5 минут. Версия 0 (мутаций в этой сессии не было) —
 * канонический путь без параметра.
 */
export function photoDisplayUrl(photoUrl: string, version: number): string {
  return version > 0 ? `${photoUrl}?v=${version}` : photoUrl;
}
