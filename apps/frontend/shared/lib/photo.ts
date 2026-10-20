/**
 * Общие клиентские хелперы приватных фото (ADR 0065): пречек капа файла.
 * Чистые функции без сети — решение о валидности остаётся за бэкендом
 * (магические байты), пречек только отсекает заведомо отвергнутое до
 * загрузки. Слоеные правила показа (stage черновика формы) живут в своих
 * срезах (lib/photo.ts внутри features) — здесь только то, что не зависит
 * от сущности. Бастер кэша выдачи снесён: photoUrl приходит с бэка уже
 * с версией ключа (?v=), замена фото = новый URL без клиентского стейта.
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
 * Файл → data URL для превью staged-замены (CSP `img-src 'self' data:`,
 * ADR 0065 покрывает его без расширений). Promise-обёртка FileReader:
 * не-строковый результат (ArrayBuffer/null) схлопывается в null —
 * сузили для тайпчекера.
 */
export function readPhotoDataUrl(file: File): Promise<string | null> {
  return new Promise((resolve) => {
    const reader = new FileReader();
    reader.onload = (): void => {
      resolve(typeof reader.result === 'string' ? reader.result : null);
    };
    reader.readAsDataURL(file);
  });
}
