import { describe, expect, it } from 'vitest';
import { circlePhotoView, photoBlurSrc } from './circle-photo';

/** Канон тестов фронта — чистая логика без DOM (vitest.config.mts):
 * решается только «что показывает круг» (circle-photo-variants-подход,
 * прецедент circle-icon-variants); сам рендер — экранная приёмка и e2e. */

describe('circlePhotoView — что показывает круг фото', () => {
  it('без фото (null/undefined/ пустая строка) — только глиф: imgSrc null', () => {
    // Гард пустой строки — WebKit: <img src=""> не стреляет onError
    // (канон UserAvatar, history-feed-screen.tsx:760-762).
    expect(
      circlePhotoView(null, null, { photoBroken: false, photoReady: false }).imgSrc,
    ).toBeNull();
    expect(
      circlePhotoView(undefined, null, { photoBroken: false, photoReady: false }).imgSrc,
    ).toBeNull();
    expect(
      circlePhotoView('', null, { photoBroken: false, photoReady: false }).imgSrc,
    ).toBeNull();
  });

  it('фото есть, onLoad ещё не стрелял — img скрыт (глиф-подложка в ожидании); запрашивается thumb', () => {
    const view = circlePhotoView('/api/properties/p1/photo?v=abc', null, {
      photoBroken: false,
      photoReady: false,
    });
    expect(view.imgSrc).toBe('/api/properties/p1/photo?v=abc&size=thumb');
    expect(view.imgHidden).toBe(true);
  });

  it('onLoad стрелял — img показан', () => {
    const view = circlePhotoView('/api/properties/p1/photo?v=abc', null, {
      photoBroken: false,
      photoReady: true,
    });
    expect(view.imgSrc).toBe('/api/properties/p1/photo?v=abc&size=thumb');
    expect(view.imgHidden).toBe(false);
  });

  it('битое фото — круг возвращается к глифу (imgSrc null), даже если ready', () => {
    const view = circlePhotoView('/api/properties/p1/photo', null, {
      photoBroken: true,
      photoReady: true,
    });
    expect(view.imgSrc).toBeNull();
    expect(view.imgHidden).toBe(false);
  });

  it('инвариант: скрытый img всегда с непустым src (скрывать нечего иначе)', () => {
    const cases: Array<
      [string | null | undefined, string | null | undefined, { photoBroken: boolean; photoReady: boolean }]
    > = [
      [null, null, { photoBroken: false, photoReady: false }],
      ['', null, { photoBroken: false, photoReady: true }],
      ['/api/x', null, { photoBroken: true, photoReady: false }],
      [undefined, null, { photoBroken: true, photoReady: true }],
    ];
    for (const [url, blur, flags] of cases) {
      const view = circlePhotoView(url, blur, flags);
      if (view.imgHidden) {
        expect(view.imgSrc).not.toBeNull();
      }
    }
  });
});

describe('circlePhotoView — ретрай транзиентной ошибки (ADR 0065)', () => {
  it('ретрай нулевой — thumb поверх версионного URL от бэка', () => {
    const view = circlePhotoView('/api/me/photo?v=abc', null, {
      photoBroken: false,
      photoReady: false,
      retryNonce: 0,
    });
    expect(view.imgSrc).toBe('/api/me/photo?v=abc&size=thumb');
  });

  it('первая ошибка — src с ретрай-нонсом в хвосте существующего query', () => {
    const view = circlePhotoView('/api/me/photo?v=abc', null, {
      photoBroken: false,
      photoReady: false,
      retryNonce: 1,
    });
    expect(view.imgSrc).toBe('/api/me/photo?v=abc&size=thumb&r=1');
  });

  it('ретрай на URL без query — нонс последним параметром', () => {
    const view = circlePhotoView('/api/me/photo', null, {
      photoBroken: false,
      photoReady: false,
      retryNonce: 2,
    });
    expect(view.imgSrc).toBe('/api/me/photo?size=thumb&r=2');
  });

  it('битое после ретрая — глиф, нонс не рисуется', () => {
    const view = circlePhotoView('/api/me/photo?v=abc', null, {
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
    const view = circlePhotoView(dataUrl, null, { photoBroken: false, photoReady: false });
    expect(view.imgSrc).toBe(dataUrl);
    expect(view.imgHidden).toBe(true);
  });

  it('готовое превью — показано как есть', () => {
    const view = circlePhotoView(dataUrl, null, { photoBroken: false, photoReady: true });
    expect(view.imgSrc).toBe(dataUrl);
    expect(view.imgHidden).toBe(false);
  });

  it('blob:-URL — как есть, без суффикса', () => {
    expect(
      circlePhotoView(blobUrl, null, { photoBroken: false, photoReady: false }).imgSrc,
    ).toBe(blobUrl);
  });
});

describe('photoBlurSrc — кадр DTO в src blur-слоя (бэк едет голым base64, #1604 приёмка)', () => {
  const bare = '/9j/4AAQSkZJRg==';

  it('голый base64 с провода (контракт #1603) — оборачивается в data:image/jpeg;base64', () => {
    expect(photoBlurSrc(bare)).toBe(`data:image/jpeg;base64,${bare}`);
  });

  it('готовый data-URL — как есть (не двойное обёртывание)', () => {
    const ready = `data:image/jpeg;base64,${bare}`;
    expect(photoBlurSrc(ready)).toBe(ready);
  });

  it('NULL/пустой кадр — null (фазы нет)', () => {
    expect(photoBlurSrc(null)).toBeNull();
    expect(photoBlurSrc(undefined)).toBeNull();
    expect(photoBlurSrc('')).toBeNull();
  });
});

describe('circlePhotoView — blur-up фазовая модель (решение #1594, тикет #1604)', () => {
  const blur = 'data:image/jpeg;base64,/9j/4AAQSkZJRg==';
  const url = '/api/properties/p1/photo?v=abc';

  it('blur приехал в DTO, thumb ещё не готов — blur-фаза активна с первого кадра (без ожидания)', () => {
    const view = circlePhotoView(url, blur, { photoBroken: false, photoReady: false });
    expect(view.blurActive).toBe(true);
    // img-семантика не меняется: thumb по-прежнему скрыт под blur-фазой.
    expect(view.imgSrc).toBe(`${url}&size=thumb`);
    expect(view.imgHidden).toBe(true);
  });

  it('photoBlur NULL (старые фото до ленивой материализации, webp) — деградация к сегодняшнему «глиф → fade»', () => {
    expect(
      circlePhotoView(url, null, { photoBroken: false, photoReady: false }).blurActive,
    ).toBe(false);
    expect(
      circlePhotoView(url, undefined, { photoBroken: false, photoReady: false }).blurActive,
    ).toBe(false);
    expect(
      circlePhotoView(url, '', { photoBroken: false, photoReady: false }).blurActive,
    ).toBe(false);
  });

  it('кеш-попадание по photo-session-cache (photoReady с первого рендера) — blur-фазы нет целиком (инвариант анти-мигания)', () => {
    const view = circlePhotoView(url, blur, { photoBroken: false, photoReady: true });
    expect(view.blurActive).toBe(false);
    expect(view.imgHidden).toBe(false);
  });

  it('onLoad (thumb готов) — blur-фаза гаснет, «blur → thumb» уходит в fade 250ms', () => {
    expect(
      circlePhotoView(url, blur, { photoBroken: false, photoReady: true }).blurActive,
    ).toBe(false);
  });

  it('ошибка загрузки при активной blur-фазе — чистый фолбэк: глиф без blur', () => {
    const view = circlePhotoView(url, blur, { photoBroken: true, photoReady: false });
    expect(view.blurActive).toBe(false);
    expect(view.imgSrc).toBeNull();
  });

  it('без фото blur-фазы нет вовсе, каким бы ни был кадр', () => {
    expect(
      circlePhotoView(null, blur, { photoBroken: false, photoReady: false }).blurActive,
    ).toBe(false);
    expect(
      circlePhotoView('', blur, { photoBroken: false, photoReady: false }).blurActive,
    ).toBe(false);
  });

  it('blur-фаза не меняет решение по ретрай-нонсу: активна и на &r=1', () => {
    const view = circlePhotoView(url, blur, {
      photoBroken: false,
      photoReady: false,
      retryNonce: 1,
    });
    expect(view.blurActive).toBe(true);
    expect(view.imgSrc).toBe(`${url}&size=thumb&r=1`);
  });
});
