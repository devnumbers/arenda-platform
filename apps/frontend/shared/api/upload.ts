import { ApiError } from './errors';
import { ratioToPercent } from '@/shared/lib/format-money';
import { parseTransportResponse } from './transport-wire';

/**
 * XHR-транспорт загрузки файлов с прогрессом выгрузки (план 036, ADR
 * 0065): единственное место XMLHttpRequest в продукте. fetch не отдаёт
 * upload-progress, duplex-стримы запроса — без Safari/WebKit, поэтому
 * фото-слоты едут через xhr.upload.onprogress. Семантика ошибок — та же,
 * что у fetch-транспорта: problem+json бэка разбирает
 * parseTransportResponse (ApiError с requestId/status/fieldErrors),
 * сетевой провал — network_error с каноническим текстом client.ts.
 * Content-Type не ставится — boundary multipart-формы ставит браузер.
 */
const NETWORK_ERROR_MESSAGE =
  'Не удалось выполнить запрос. Проверьте подключение к интернету.';

export function uploadFile<T>(
  path: string,
  file: File,
  onProgress?: (percent: number) => void,
): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open('POST', `/api${path}`);
    xhr.setRequestHeader('Accept', 'application/json');
    xhr.upload.onprogress = (event): void => {
      if (event.lengthComputable && event.total > 0 && onProgress !== undefined) {
        onProgress(Math.min(100, Math.round(ratioToPercent(event.loaded / event.total))));
      }
    };
    xhr.onerror = (): void => {
      reject(new ApiError('network_error', NETWORK_ERROR_MESSAGE));
    };
    xhr.onabort = (): void => {
      reject(new ApiError('network_error', NETWORK_ERROR_MESSAGE));
    };
    xhr.onload = (): void => {
      // Заголовки ответа — из сырой CRLF-строки XHR (X-Request-ID и
      // Content-Type нужны разбору problem+json).
      const headers = new Headers();
      for (const line of xhr.getAllResponseHeaders().trim().split(/[\r\n]+/)) {
        const separator = line.indexOf(':');
        if (separator > 0) {
          headers.append(line.slice(0, separator).trim(), line.slice(separator + 1).trim());
        }
      }
      void parseTransportResponse<T>(
        new Response(xhr.response, {
          status: xhr.status,
          statusText: xhr.statusText,
          headers,
        }),
      ).then(resolve, reject);
    };
    const body = new FormData();
    body.append('file', file);
    xhr.send(body);
  });
}
