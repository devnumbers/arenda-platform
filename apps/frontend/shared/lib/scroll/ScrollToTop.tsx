'use client';

import {useEffect, useLayoutEffect} from 'react';
import {usePathname} from 'next/navigation';
import {isAnchoredRoute} from './anchored-routes';

const useIsoLayoutEffect = typeof window !== 'undefined' ? useLayoutEffect : useEffect;

export function ScrollToTop(): null {
    const pathname = usePathname();

    useEffect(() => {
        const previous = window.history.scrollRestoration;
        // App Router restores scroll on back/forward; take manual control so pages always open at the top.
        window.history.scrollRestoration = 'manual';
        return () => {
            window.history.scrollRestoration = previous;
        };
    }, []);

    useIsoLayoutEffect(() => {
        if (window.location.hash) {
            return;
        }
        // Страницы истории позиционирует сама лента — якорь «с конца»
        // лейаут-эффектом до пэйнта (карта #1324, D8.1): глобальный скролл
        // к верху их не трогает, иначе он затирает якорь в том же коммите.
        // Решение владельца 10.10.2026 (#1319): всегда открытие «с конца»,
        // реставрации позиции чтения нет.
        if (isAnchoredRoute(pathname)) {
            return;
        }
        window.scrollTo(0, 0);
    }, [pathname]);

    return null;
}
