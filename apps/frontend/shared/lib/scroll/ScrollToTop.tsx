'use client';

import {useEffect, useLayoutEffect} from 'react';
import {usePathname} from 'next/navigation';
import {isAnchoredRoute, notePendingReadingPosition} from './reading-position';

const useIsoLayoutEffect = typeof window !== 'undefined' ? useLayoutEffect : useEffect;

export function ScrollToTop(): null {
    const pathname = usePathname();

    useEffect(() => {
        const previous = window.history.scrollRestoration;
        // App Router restores scroll on back/forward; take manual control so pages always open at the top.
        window.history.scrollRestoration = 'manual';
        // Позиция чтения лент истории (карта #1324, D8.2): popstate на
        // якорёный роут помечает цель — лента при первом позиционировании
        // вместо якоря тянет сохранённый scrollTop (reading-position.ts).
        const onPop = (): void => {
            if (isAnchoredRoute(window.location.pathname)) {
                notePendingReadingPosition();
            }
        };
        window.addEventListener('popstate', onPop);
        return () => {
            window.history.scrollRestoration = previous;
            window.removeEventListener('popstate', onPop);
        };
    }, []);

    useIsoLayoutEffect(() => {
        if (window.location.hash) {
            return;
        }
        // Страницы истории позиционирует сама лента (якорь «с конца» D8.1,
        // реставрация позиции чтения D8.2) — глобальный скролл к верху их
        // не трогает, иначе он затирает якорь-лейаут-эффект ленты.
        if (isAnchoredRoute(pathname)) {
            return;
        }
        window.scrollTo(0, 0);
    }, [pathname]);

    return null;
}
