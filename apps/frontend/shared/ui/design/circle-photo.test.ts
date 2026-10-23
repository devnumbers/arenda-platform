import { describe, expect, it } from 'vitest';
import { circlePhotoView } from './circle-photo';

/** Канон тестов фронта — чистая логика без DOM (vitest.config.mts):
 * решается только «что показывает круг» (circle-photo-variants-подход,
 * прецедент circle-icon-variants); сам рендер — экранная приёмка и e2e. */

describe('circlePhotoView — что показывает круг фото', () => {
  it('без фото (null/undefined/ пустая строка) — только глиф: imgSrc null', () => {
    // Гард пустой строки — WebKit: <img src=""> не стреляет onError
    // (канон UserAvatar, history-feed-screen.tsx:760-762).
    expect(circlePhotoView(null, { photoBroken: false, photoReady: false }).imgSrc).toBeNull();
    expect(circlePhotoView(undefined, { photoBroken: false, photoReady: false }).imgSrc).toBeNull();
    expect(circlePhotoView('', { photoBroken: false, photoReady: false }).imgSrc).toBeNull();
  });

  it('фото есть, onLoad ещё не стрелял — img скрыт (глиф-подложка в ожидании); запрашивается thumb', () => {
    const view = circlePhotoView('/api/properties/p1/photo?v=abc', {
      photoBroken: false,
      photoReady: false,
    });
    expect(view.imgSrc).toBe('/api/properties/p1/photo?v=abc&size=thumb');
    expect(view.imgHidden).toBe(true);
  });

  it('onLoad стрелял — img показан', () => {
    const view = circlePhotoView('/api/properties/p1/photo?v=abc', {
      photoBroken: false,
      photoReady: true,
    });
    expect(view.imgSrc).toBe('/api/properties/p1/photo?v=abc&size=thumb');
    expect(view.imgHidden).toBe(false);
  });

  it('битое фото — круг возвращается к глифу (imgSrc null), даже если ready', () => {
    const view = circlePhotoView('/api/properties/p1/photo', {
      photoBroken: true,
      photoReady: true,
    });
    expect(view.imgSrc).toBeNull();
    expect(view.imgHidden).toBe(false);
  });

  it('инвариант: скрытый img всегда с непустым src (скрывать нечего иначе)', () => {
    const cases: Array<[string | null | undefined, { photoBroken: boolean; photoReady: boolean }]> = [
      [null, { photoBroken: false, photoReady: false }],
      ['', { photoBroken: false, photoReady: true }],
      ['/api/x', { photoBroken: true, photoReady: false }],
      [undefined, { photoBroken: true, photoReady: true }],
    ];
    for (const [url, flags] of cases) {
      const view = circlePhotoView(url, flags);
      if (view.imgHidden) {
        expect(view.imgSrc).not.toBeNull();
      }
    }
  });
});

describe('circlePhotoView — ретрай транзиентной ошибки (ADR 0065)', () => {
  it('ретрай нулевой — thumb поверх версионного URL от бэка', () => {
    const view = circlePhotoView('/api/me/photo?v=abc', {
      photoBroken: false,
      photoReady: false,
      retryNonce: 0,
    });
    expect(view.imgSrc).toBe('/api/me/photo?v=abc&size=thumb');
  });

  it('первая ошибка — src с ретрай-нонсом в хвосте существующего query', () => {
    const view = circlePhotoView('/api/me/photo?v=abc', {
      photoBroken: false,
      photoReady: false,
      retryNonce: 1,
    });
    expect(view.imgSrc).toBe('/api/me/photo?v=abc&size=thumb&r=1');
  });

  it('ретрай на URL без query — нонс последним параметром', () => {
    const view = circlePhotoView('/api/me/photo', {
      photoBroken: false,
      photoReady: false,
      retryNonce: 2,
    });
    expect(view.imgSrc).toBe('/api/me/photo?size=thumb&r=2');
  });

  it('битое после ретрая — глиф, нонс не рисуется', () => {
    const view = circlePhotoView('/api/me/photo?v=abc', {
      photoBroken: true,
      photoReady: true,
      retryNonce: 1,
    });
    expect(view.imgSrc).toBeNull();
  });
});

describe('circlePhotoView — непроизводные URL: превью data:/blob: (тикет #1607)', () => {
  const dataUrl = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUg==';
  const blobUrl = 'blob:http://localhost:3029/0b32e13b-f2a1-4f36-9f0e-2f7ddb3d1d34';

  it('data:-превью летящей загрузки — src как есть, без thumb-суффикса (иначе ERR_INVALID_URL); img ждёт декод под глифом', () => {
    const view = circlePhotoView(dataUrl, { photoBroken: false, photoReady: false });
    expect(view.imgSrc).toBe(dataUrl);
    expect(view.imgHidden).toBe(true);
  });

  it('готовое превью — показано как есть', () => {
    const view = circlePhotoView(dataUrl, { photoBroken: false, photoReady: true });
    expect(view.imgSrc).toBe(dataUrl);
    expect(view.imgHidden).toBe(false);
  });

  it('blob:-URL — как есть, без суффикса', () => {
    expect(circlePhotoView(blobUrl, { photoBroken: false, photoReady: false }).imgSrc).toBe(blobUrl);
  });
});
