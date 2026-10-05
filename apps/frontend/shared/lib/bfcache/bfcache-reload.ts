/**
 * Шов гарда bfcache (#1098): слушатель `pageshow` перезагружает документ,
 * если страница восстановлена из back/forward cache (`event.persisted`).
 *
 * Зачем: Chrome (bfcache для no-store-страниц по HTTPS, CCNS) и Safari
 * умеют возвращать страницу кнопкой «Назад» из in-memory снимка — вместе
 * с JS-heap, где живут RSC-кэш Next и react-query. После выхода такой
 * возврат показывал бы кабинет без единого сетевого запроса. Снимок
 * заголовком не контролируется (research #1097), единственный гард —
 * полная перезагрузка: свежий документ запрашивает сервер, и без сессии
 * proxy.ts уводит его на /login. Проверку по document.cookie не делаем —
 * сессия httpOnly; reload при любом persisted, цикла нет: свежезагруженный
 * документ из bfcache не восстанавливается.
 *
 * Вынесено из компонента в чистую функцию: vitest ходит в node-окружении
 * без DOM (vitest.config.mts), окно подставляется параметром.
 */
export type BfcacheWindow = {
    addEventListener(
        type: 'pageshow',
        listener: (event: {persisted: boolean}) => void,
        options?: AddEventListenerOptions,
    ): void;
    removeEventListener(
        type: 'pageshow',
        listener: (event: {persisted: boolean}) => void,
        options?: AddEventListenerOptions,
    ): void;
    location: {reload(): void};
};

/** Вешает гард и возвращает функцию снятия (расходуется как cleanup useEffect). */
export function armBfcacheReload(win: BfcacheWindow): () => void {
    const onPageShow = (event: {persisted: boolean}): void => {
        if (event.persisted) {
            win.location.reload();
        }
    };
    win.addEventListener('pageshow', onPageShow);
    return () => win.removeEventListener('pageshow', onPageShow);
}
