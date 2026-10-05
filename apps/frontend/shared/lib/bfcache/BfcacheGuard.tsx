'use client';

import {useEffect, type JSX} from 'react';
import {armBfcacheReload} from './bfcache-reload';

/**
 * Гард bfcache (#1098) — монтируется в корневом layout, как ErrorReporter и
 * ScrollToTop: клиентский компонент без разметки, вся логика в чистой
 * {@link armBfcacheReload} (см. там — почему reload при любом persisted и
 * почему заголовком это не решается). Восстановление страницы из
 * back/forward cache кнопкой «Назад»/«Вперёд» заменяется полной
 * перезагрузкой, поэтому ЛК после выхода из bfcache не возвращается ни в
 * одном браузере.
 */
export function BfcacheGuard(): JSX.Element | null {
    useEffect(() => armBfcacheReload(window), []);
    return null;
}
