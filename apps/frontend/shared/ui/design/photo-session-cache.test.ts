import { describe, expect, it } from 'vitest';
import { isPhotoThumbLoaded, markPhotoThumbLoaded, photoThumbKey } from './photo-session-cache';

/** Канон тестов фронта — чистая логика без DOM (vitest.config.mts):
 * реестр — модульное состояние, ключи в тестах уникальные, чтобы прогоны
 * не зависели от порядка (прецедент circle-photo.test.ts). Проводка
 * реестра в CirclePhoto (старт photoReady по факту загрузки) — экранная
 * приёмка и e2e. */

describe('photoThumbKey — thumb-ключ версионированного URL', () => {
  it('URL без query — size=thumb первым параметром', () => {
    expect(photoThumbKey('/api/me/photo')).toBe('/api/me/photo?size=thumb');
  });

  it('URL с query (?v=) — size=thumb в хвосте', () => {
    expect(photoThumbKey('/api/me/photo?v=abc')).toBe('/api/me/photo?v=abc&size=thumb');
  });

  it('ключ не зависит от чужих хвостов — ретрай-нонс (&r=N) в ключ не входит', () => {
    // &r= добавляется к imgSrc поверх ключа; реестр ключует базовый thumb,
    // чтобы успех ретрая тоже помечал исходное фото.
    expect(photoThumbKey('/api/me/photo?v=abc')).not.toContain('r=');
  });
});

describe('сессионный реестр загруженных фото', () => {
  it('непомеченное фото — не загружено', () => {
    expect(isPhotoThumbLoaded('/api/me/photo?v=never-seen')).toBe(false);
  });

  it('помеченное — загружено; другой URL остаётся незагруженным', () => {
    markPhotoThumbLoaded('/api/me/photo?v=seen-1');
    expect(isPhotoThumbLoaded('/api/me/photo?v=seen-1')).toBe(true);
    expect(isPhotoThumbLoaded('/api/me/photo?v=other')).toBe(false);
  });

  it('смена фото (новый ?v=) — новый ключ, старая отметка не перекидывается', () => {
    markPhotoThumbLoaded('/api/me/photo?v=old-hash');
    expect(isPhotoThumbLoaded('/api/me/photo?v=old-hash')).toBe(true);
    expect(isPhotoThumbLoaded('/api/me/photo?v=new-hash')).toBe(false);
  });

  it('успех ретрая (&r=1) помечает базовый thumb — ключ строится без нонса', () => {
    // CirclePhoto зовёт markPhotoThumbLoaded(photoUrl) — без нонса; проверяем,
    // что ключ от того же URL совпадает с ключом загруженного src без &r.
    markPhotoThumbLoaded('/api/me/photo?v=retry');
    expect(isPhotoThumbLoaded('/api/me/photo?v=retry')).toBe(true);
  });
});
