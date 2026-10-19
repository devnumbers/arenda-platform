import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// Тестовое окружение — node без DOM (vitest.config.mts): XMLHttpRequest
// подменяется фейком, ловящим вызовы; события запускаются тестом вручную.
// Фейк реализует ровно поверхность, которую читает uploadFile: open/send,
// setRequestHeader, upload.onprogress, onload/onerror, response/status.

class FakeXhr {
  static instances: FakeXhr[] = [];

  open = vi.fn();
  send = vi.fn();
  setRequestHeader = vi.fn();
  response = '';
  status = 0;
  statusText = '';
  responseHeaders: Record<string, string> = {};
  getAllResponseHeaders = vi.fn((): string =>
    Object.entries(this.responseHeaders)
      .map(([name, value]) => `${name}: ${value}`)
      .join('\r\n'),
  );
  upload = {
    onprogress: null as
      | ((event: { lengthComputable: boolean; loaded: number; total: number }) => void)
      | null,
  };
  onload: (() => void) | null = null;
  onerror: (() => void) | null = null;

  constructor() {
    FakeXhr.instances.push(this);
  }

  respond(status: number, body: string, contentType?: string): void {
    this.status = status;
    this.response = body;
    this.responseHeaders = contentType === undefined ? {} : { 'Content-Type': contentType };
    this.onload?.();
  }

  emitProgress(loaded: number, total: number): void {
    this.upload.onprogress?.({ lengthComputable: true, loaded, total });
  }

  fail(): void {
    this.onerror?.();
  }
}

// Стаб — сам класс: uploadFile зовёт new XMLHttpRequest(), стаб обязан
// быть конструктором (стрелка/функция-завод с new не дружат); ассерты
// ведутся по FakeXhr.instances.
vi.stubGlobal('XMLHttpRequest', FakeXhr);

import { ApiError } from './errors';
import { uploadFile } from './upload';

const file = new File(['bytes'], 'photo.jpg', { type: 'image/jpeg' });

/** Последний созданный XHR: uploadFile синхронно конструирует его
 * внутри промиса — к моменту первого ассерта инстанс уже есть. */
function lastXhr(): FakeXhr {
  const xhr = FakeXhr.instances[FakeXhr.instances.length - 1];
  if (xhr === undefined) {
    throw new Error('uploadFile не создал XHR');
  }
  return xhr;
}

describe('uploadFile — XHR-транспорт фото-загрузок (план 036, ADR 0065)', () => {
  beforeEach(() => {
    vi.stubGlobal('XMLHttpRequest', FakeXhr);
    FakeXhr.instances.length = 0;
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('POST multipart-формой с полем file на /api-путь, Accept: application/json', async () => {
    const pending = uploadFile('/properties/prop-1/photo', file);
    const xhr = lastXhr();
    expect(xhr).toBeDefined();
    expect(xhr.open).toHaveBeenCalledWith('POST', '/api/properties/prop-1/photo');
    expect(xhr.setRequestHeader).toHaveBeenCalledWith('Accept', 'application/json');
    expect(xhr.send).toHaveBeenCalledTimes(1);
    const body = xhr.send.mock.calls[0]?.[0];
    expect(body).toBeInstanceOf(FormData);
    if (body instanceof FormData) {
      expect(body.get('file')).toBe(file);
    }
    xhr.respond(200, JSON.stringify({ ok: true }));
    await expect(pending).resolves.toEqual({ ok: true });
  });

  it('onProgress отдаёт проценты по событиям выгрузки', async () => {
    const seen: number[] = [];
    const pending = uploadFile('/me/photo', file, (percent) => seen.push(percent));
    const xhr = lastXhr();
    xhr.emitProgress(0, 1000);
    xhr.emitProgress(500, 1000);
    xhr.emitProgress(1000, 1000);
    expect(seen).toEqual([0, 50, 100]);
    xhr.respond(200, '{}');
    await pending;
  });

  it('lengthComputable=false проценты не отдаёт', async () => {
    const seen: number[] = [];
    const pending = uploadFile('/me/photo', file, (percent) => seen.push(percent));
    const xhr = lastXhr();
    xhr.upload.onprogress?.({ lengthComputable: false, loaded: 10, total: 100 });
    expect(seen).toEqual([]);
    xhr.respond(200, '{}');
    await pending;
  });

  it('problem+json ошибки бэка — ApiError с кодом и детилом бэка', async () => {
    const pending = uploadFile('/me/photo', file);
    const xhr = lastXhr();
    xhr.respond(
      400,
      JSON.stringify({
        type: 'about:blank',
        title: 'Bad Request',
        status: 400,
        detail: 'Неподдерживаемый формат файла',
      }),
      'application/problem+json',
    );
    const error = await pending.catch((caught: unknown) => caught);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).status).toBe(400);
    expect((error as ApiError).detail).toBe('Неподдерживаемый формат файла');
  });

  it('сетевой провал — ApiError network_error с каноническим текстом client.ts', async () => {
    const pending = uploadFile('/me/photo', file);
    const xhr = lastXhr();
    xhr.fail();
    const error = await pending.catch((caught: unknown) => caught);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).code).toBe('network_error');
    expect((error as ApiError).detail).toBe(
      'Не удалось выполнить запрос. Проверьте подключение к интернету.',
    );
  });
});
