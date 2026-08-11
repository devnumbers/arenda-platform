'use client';

import { useLayoutEffect, type JSX } from 'react';

/**
 * Hides the inline boot screen (`#boot-screen` in `app/layout.tsx`) once React
 * has hydrated.
 *
 * The boot screen is a branded splash (glass mark + Rentle wordmark + spinner)
 * rendered as static HTML inside the Server Component layout, so it lands in
 * the first server response and is visible during the gap between first paint
 * and React hydration — otherwise the user sees a white screen, especially in a
 * PWA launch where the native `apple-touch-startup-image` is dismissed the
 * moment the webview begins to draw.
 *
 * Hiding is done purely via CSS (`body.hydrated #boot-screen { display: none }`
 * in `globals.css`): `useLayoutEffect` runs synchronously after hydration but
 * before the browser paints, so the swap is flicker-free and React never has
 * to reconcile the boot node. The node is NOT removed from the DOM — it stays
 * mounted and is hidden by the CSS class. `suppressHydrationWarning` on the
 * node in the layout tolerates it being static markup that React never
 * re-renders.
 *
 * Rendered as `null` — this component exists only for its effect.
 */
export function BootScreen(): JSX.Element | null {
    useLayoutEffect(() => {
        document.body.classList.add('hydrated');
    }, []);

    return null;
}
