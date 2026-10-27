/**
 * Политика приватного SW-кеша photo-thumb (карта #1587, тикет #1608;
 * решение #1593 — открытые подвопросы решены здесь, аменд ADR 0065).
 *
 * Service worker (`public/sw.js`) держит cache-first кеш thumb-выдачи
 * (cookie-scoped стриминг ADR 0065): повторные показы после полной
 * перезагрузки и «Назад» приходят из Cache Storage, а не по сети — в том
 * числе офлайн и после вытеснения HTTP-кеша (Safari ITP ~7 дней).
 *
 * Модуль — источник истины чистых правил (матчинг, ключ, политика очистки);
 * сам SW — статический скрипт и TypeScript в раннтайме импортировать не
 * может, поэтому держит инлайн-копию с теми же именами. Guard-тесты
 * `photo-cache-policy.test.ts` («service worker photo cache stays in sync»)
 * сверяют ключевые маркеры копий (имена, константы, критические ветки) —
 * канон паритета sw.js ↔ TS (ADR 0032, app-routes/push-payload); тела
 * чистых функций остаются ручной синхронизацией. Меняя правило — меняй
 * обе копии.
 */

/** Имя приватного кеша тумбов. Бамп версии — при несовместимой смене
 * правил матчинга/ключа: старый кеш снесёт activate SW (принцип
 * «смена стратегии — бамп версии», ADR 0065/0032). Вне себя держит
 * offline-кеш (`rentli-offline-*`) и публичный `static-assets-v1`
 * (#1600) — кеши не пересекаются. */
export const PHOTO_CACHE_NAME = 'photo-thumbs-v1';

/** Потолок записей кеша (FIFO-эвикция по порядку вставки Cache API —
 * записи иммутабельны: ключ = версионный URL, LRU-семантика не нужна).
 * Thumb ≤320px jpeg — десятки КБ, 300 записей — единицы МБ на устройство. */
export const PHOTO_CACHE_MAX_ENTRIES = 300;

/** Владельцы фото с потоковой выдачей `…/{id}/photo` (ADR 0065); отдельная
 * форма — `/api/me/photo`. Ровно эти пути и матчит кеш. */
export const PHOTO_CACHE_SCOPES: ReadonlyArray<string> = ['properties', 'contacts', 'users'];

/** Логаут: фото-кеш чистится по факту любого завершённого ответа —
 * инвариант приватности ADR 0065 («фото одного аккаунта не достаются
 * другому»). Сетевой провал запроса сессию не закрывает — чистки нет. */
export const PHOTO_CACHE_CLEAR_ALWAYS_PATH = '/api/auth/logout';

/** Логин (verify): чистка при успехе — закрывает сценарий «вкладку закрыли
 * не выходя» и смену аккаунта: новый пользователь устройства всегда
 * стартует с пустым фото-кешем. Цена — один холодный показ на вход
 * (смягчается immutable HTTP-кешем). */
export const PHOTO_CACHE_CLEAR_ON_SUCCESS_PATH = '/api/auth/verify';

/** Что делает SW с фото-кешем на auth-запросе. */
export type PhotoCacheClearPolicy = 'always' | 'on-success' | 'never';

/** Кешируется ли ответ на этот запрос: GET по одной из форм пути выдачи
 * (ADR 0065) с `size=thumb` и непустой версией `?v=`. Полный объект
 * (запрос без `size=thumb`) не кешируется — единственный потребитель
 * фото сегодня `CirclePhoto`, всегда запрашивающий тумб; крупного
 * просмотра нет. Cache-first безопасен только для версионных URL:
 * замена фото — новый `?v=`, кеш никогда не отдаёт прежние байты. */
export function isPhotoThumbCacheableRequest(
    method: string,
    pathname: string,
    searchParams: URLSearchParams,
): boolean {
    if (method !== 'GET') {
        return false;
    }
    if (searchParams.get('size') !== 'thumb') {
        return false;
    }
    if ((searchParams.get('v') ?? '') === '') {
        return false;
    }
    return isPhotoStreamPath(pathname);
}

function isPhotoStreamPath(pathname: string): boolean {
    // Ведущие/хвостовые слэши не считаются сегментами: «/api/me/photo» и
    // «/api/properties/<id>/photo» — ровно 3 и 4 сегмента; /api/v1/…
    // (снятая форма) и вложенные пути длиной 5+ не матчатся.
    const segments = pathname.split('/').filter((segment) => segment !== '');
    if (segments.length === 3) {
        return segments[0] === 'api' && segments[1] === 'me' && segments[2] === 'photo';
    }
    if (segments.length === 4) {
        return (
            segments[0] === 'api' &&
            PHOTO_CACHE_SCOPES.includes(segments[1] ?? '') &&
            segments[3] === 'photo'
        );
    }
    return false;
}

/** Ключ записи кеша для thumb-URL: ретрай-нонс `&r=N` (CirclePhoto, ADR
 * 0065 аменд 2026-10-20) вырезается — успешный ретрай становится
 * каноничным тумбом, и последующие загрузки без нонса попадают в кеш.
 * Остальные параметры (включая порядок) не трогаются. */
export function photoThumbCacheKey(href: string): string {
    let url: URL;
    try {
        url = new URL(href);
    } catch {
        return href;
    }
    if (!url.searchParams.has('r')) {
        return href;
    }
    url.searchParams.delete('r');
    return url.href;
}

/** Ретрай-запрос (с нонсом `&r=N`) идёт мимо cache.match — network-first:
 * смысл ретрая в перепросе сети, и если в кеше лежат байты, которые не
 * декодируются, отдача того же содержимого ломает двухударный фолбэк
 * (#1275). Результат успешного ретрая при этом кешируется под вырезанным
 * ключом (см. photoThumbCacheKey) — каноничным тумбом. */
export function photoThumbRequestSkipsCache(searchParams: URLSearchParams): boolean {
    return searchParams.has('r');
}

/** Ответ годится для записи в кеш: 200 проверяется в SW отдельно, здесь —
 * контент-тип. Ошибочная страница прокси с 200 и text/html не должна
 * навсегда прятать живое фото за кешем. */
export function isCacheablePhotoResponse(contentType: string | null): boolean {
    return contentType != null && contentType.startsWith('image/');
}

/** Политика очистки фото-кеша на этом запросе: логаут — всегда; логин —
 * при успехе; остальные запросы кеш не трогают. */
export function photoCacheClearPolicy(method: string, pathname: string): PhotoCacheClearPolicy {
    if (method !== 'POST') {
        return 'never';
    }
    if (pathname === PHOTO_CACHE_CLEAR_ALWAYS_PATH) {
        return 'always';
    }
    if (pathname === PHOTO_CACHE_CLEAR_ON_SUCCESS_PATH) {
        return 'on-success';
    }
    return 'never';
}
