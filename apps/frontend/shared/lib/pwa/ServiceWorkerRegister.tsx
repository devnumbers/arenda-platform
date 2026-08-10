'use client';

import { useEffect, type JSX } from 'react';

/**
 * Registers the cabinet service worker (`/sw.js`) with scope `/`.
 *
 * The SW is registered on every cabinet route (this component is mounted inside
 * the cabinet layout) and deliberately not registered on landing routes to
 * avoid waking it up unnecessarily. The scope is still `/` (the origin root),
 * and the SW's fetch handler filters cabinet paths itself — see `public/sw.js`.
 *
 * `updateViaCache: 'none'` guarantees the SW script bypasses the HTTP cache so
 * updates are picked up promptly. `public/` is already served with
 * `max-age=0, must-revalidate` by Next.js; the no-cache header is reinforced
 * for `/sw.js` in `next.config.ts` and at the Caddy layer.
 */
export function ServiceWorkerRegister(): JSX.Element | null {
    useEffect(() => {
        if (typeof window === 'undefined') return;
        if (!('serviceWorker' in navigator)) return;

        const register = (): void => {
            navigator.serviceWorker
                .register('/sw.js', { scope: '/', updateViaCache: 'none' })
                .catch(() => {
                    // Registration failure is non-fatal: the app stays usable
                    // as a regular website; only the offline screen and future
                    // push support are unavailable.
                });
        };

        // Defer registration to avoid competing with first-paint work.
        if (document.readyState === 'complete') {
            register();
        } else {
            window.addEventListener('load', register, { once: true });
            return () => window.removeEventListener('load', register);
        }
    }, []);

    return null;
}
