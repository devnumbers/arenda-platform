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

  it('фото есть, onLoad ещё не стрелял — img скрыт (глиф-подложка в ожидании)', () => {
    const view = circlePhotoView('/api/properties/p1/photo', {
      photoBroken: false,
      photoReady: false,
    });
    expect(view.imgSrc).toBe('/api/properties/p1/photo');
    expect(view.imgHidden).toBe(true);
  });

  it('onLoad стрелял — img показан', () => {
    const view = circlePhotoView('/api/properties/p1/photo', {
      photoBroken: false,
      photoReady: true,
    });
    expect(view.imgSrc).toBe('/api/properties/p1/photo');
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
