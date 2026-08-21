'use client';

import { useEffect, type JSX } from 'react';
import { resolveClickTarget } from './push-payload';
import { isStandaloneMode } from './standalone';
import { markStandaloneClient } from './standalone-store';

/**
 * Registers the cabinet service worker (`/sw.js`) with scope `/`.
 *
 * The SW is registered on every cabinet route (this component is mounted inside
 * the cabinet layout) and deliberately not registered on landing routes to
 * avoid waking it up unnecessarily. The scope is still `/` (the origin root),
 * and the SW's fetch handler filters cabinet paths itself — see `public/sw.js`.
 *
 * When the app runs in PWA standalone mode, a standalone flag is written to
 * IndexedDB after registration. The SW reads it to redirect any navigation to
 * `/` back to `/dashboard`, keeping PWA users inside the app (hard isolation).
 *
 * `updateViaCache: 'none'` guarantees the SW script bypasses the HTTP cache so
 * updates are picked up promptly. `public/` is already served with
 * `max-age=0, must-revalidate` by Next.js; the no-cache header is reinforced
 * for `/sw.js` in `next.config.ts` and at the Caddy layer.
 *
 * A `message` listener forwards push-notification click targets (posted by the
 * SW `notificationclick` handler when an existing cabinet window is already
 * open) to Next.js App Router, so tapping a notification navigates the focused
 * tab instead of opening a duplicate.
 */
export function ServiceWorkerRegister(): JSX.Element | null {
    useEffect(() => {
        if (typeof window === 'undefined') return;
        if (!('serviceWorker' in navigator)) return;

        const register = (): void => {
            navigator.serviceWorker
                .register('/sw.js', { scope: '/', updateViaCache: 'none' })
                .then(() => {
                    // Mark this context as a PWA client so the SW can redirect
                    // navigations to `/` back into the app. No-op outside
                    // standalone mode. Failures are non-fatal: the worst case
                    // is the hard-isolation redirect not firing.
                    if (isStandaloneMode()) {
                        void markStandaloneClient();
                    }
                })
                .catch(() => {
                    // Registration failure is non-fatal: the app stays usable
                    // as a regular website; only the offline screen and future
                    // push support are unavailable.
                });
        };

        // Defer registration to avoid competing with first-paint work.
        if (document.readyState === 'complete') {
            register();
            return undefined;
        }
        window.addEventListener('load', register, { once: true });
        return () => window.removeEventListener('load', register);
    }, []);

    useEffect(() => {
        if (typeof window === 'undefined') return;
        if (!('serviceWorker' in navigator)) return;

        const onMessage = (event: MessageEvent): void => {
            // The SW posts `{ type: 'PUSH_NOTIFICATION_CLICK', url }`; held as
            // unknown because MessageEvent.data is `any` on the DOM side.
            const data: unknown = event.data;
            if (!data || typeof data !== 'object') return;
            if (!('type' in data) || data.type !== 'PUSH_NOTIFICATION_CLICK') return;
            // `resolveClickTarget` re-validates the URL the SW sent — same-origin
            // absolute path only, fallback to /dashboard otherwise.
            const url = resolveClickTarget('url' in data ? data.url : undefined);
            // Client-side navigation via the History API. The SW already focused
            // this window; we only need to move it to the click target.
            if (url !== window.location.pathname + window.location.search) {
                window.location.assign(url);
            }
        };

        navigator.serviceWorker.addEventListener('message', onMessage);
        return () => {
            navigator.serviceWorker.removeEventListener('message', onMessage);
        };
    }, []);

    return null;
}
