'use client';

import {useEffect, useLayoutEffect} from 'react';
import {usePathname} from 'next/navigation';

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
        window.scrollTo(0, 0);
    }, [pathname]);

    return null;
}
