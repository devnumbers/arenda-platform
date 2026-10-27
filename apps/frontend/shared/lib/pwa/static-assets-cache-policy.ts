/**
 * Политика публичного SW-кеша статики (карта #1587, тикет #1600; аменд
 * ADR 0065 от 2026-10-23 — SW-кеш публичной статики как слой выдачи).
 *
 * Иллюстрации пустых состояний (`public/images/**`) и PWA-иконки со
 * сплешами (`public/icons/**`) — публичная статика, одинаковая для любого
 * аккаунта. HTTP-кеш уже иммутабелен на год (next.config.ts, канон #865
 * «новая версия ассета — новое имя файла»), но браузерный кеш не закрывает
 * первый заход на устройство, вытеснение (Safari ITP чистит хранилища
 * через ~7 дней неактивности, дисковое давление Chrome) и офлайн в
 * установленном PWA — их закрывает runtime cache-first в `public/sw.js`.
 *
 * Модуль — источник истины чистых правил (матчинг, контент-тип ответа);
 * сам SW — статический скрипт и TypeScript в рантайме импортировать не
 * может, поэтому держит инлайн-копию с теми же именами. Guard-тесты
 * `static-assets-cache-policy.test.ts` («service worker static assets
 * cache stays in sync») сверяют ключевые маркеры копий (имена, константы,
 * критические ветки) — канон паритета sw.js ↔ TS (ADR 0032; app-routes/
 * push-payload/photo-cache-policy). Меняя правило — меняй обе копии.
 *
 * Ключ записи — полный URL без заголовков. Оптимизатор `/_next/image`
 * отвечает с `Vary: Accept`, но SW-кеш пер-профильный, а Accept одного
 * браузера на картинки стабилен — одна запись на URL осознанно; попадание
 * в кеш не зависит от Vary (обе стороны операции синтетические
 * header-less). Смена стратегии оптимизатора — бамп версии кеша (канон
 * ADR 0032/0065).
 */

/** Имя публичного кеша статики. Бамп версии — при смене стратегии
 * матчинга или оптимизатора (канон ADR 0032/0065): старый кеш снесёт
 * activate SW. Вне себя держит offline-кеш (`rentli-offline-*`) и
 * приватный фото-кеш (`photo-thumbs-v1`) — кеши не пересекаются; при
 * логауте этот кеш НЕ чистится — публичная статика одинакова для любого
 * аккаунта, при логауте чистится только фото-кеш. */
export const STATIC_ASSETS_CACHE_NAME = 'static-assets-v1';

/** Префиксы публичной статики: и прямой путь, и параметр `url`
 * оптимизатора должны начинаться с одного из них. Список явный и
 * консервативный: remotePatterns пуст, внешних картинок через
 * `/_next/image` нет, а явное правило самодокументируется и не заденет
 * будущую динамику (#1600). */
export const STATIC_ASSET_PUBLIC_PREFIXES: ReadonlyArray<string> = ['/images/', '/icons/'];

/** Кешируется ли ответ на этот запрос: GET прямой публичной статики
 * (`/images/*`, `/icons/*`) или ответ оптимизатора `/_next/image`, чей
 * параметр `url` указывает туда же (матчинг по декодированному значению).
 * Выбор `w`-варианта next/image зависит от DPR/зума устройства —
 * precache-списка нет, кешируются фактические URL по мере показов; квот
 * и eviction нет (сет конечен — десятки вариантов на устройство, сотни
 * КБ). Свежесть — общий инвариант с HTTP-заголовками: новая версия
 * ассета — новое имя файла (канон #865), менять байты под именем нельзя. */
export function isStaticAssetCacheableRequest(
    method: string,
    pathname: string,
    searchParams: URLSearchParams,
): boolean {
    if (method !== 'GET') {
        return false;
    }
    if (isPublicStaticPath(pathname)) {
        return true;
    }
    if (pathname !== '/_next/image') {
        return false;
    }
    return isPublicStaticPath(searchParams.get('url') ?? '');
}

function isPublicStaticPath(path: string): boolean {
    return STATIC_ASSET_PUBLIC_PREFIXES.some((prefix) => path.startsWith(prefix));
}

/** Ответ годится для записи в кеш: 200 проверяется в SW отдельно, здесь —
 * контент-тип. Ошибочная страница с 200 и text/html не должна навсегда
 * спрятать живую иллюстрацию за кешем. */
export function isCacheableStaticResponse(contentType: string | null): boolean {
    return contentType != null && contentType.startsWith('image/');
}
