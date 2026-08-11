'use client';

import { useEffect, useState } from 'react';
import { isStandaloneMode } from '@/shared/lib/pwa/standalone';

const STANDALONE_MEDIA_QUERY = '(display-mode: standalone)';

/**
 * Reactive standalone (PWA) mode flag.
 *
 * Returns `false` on the server and during the first client render (to match
 * hydration), then resolves to the real value via `matchMedia` (Android/Chrome)
 * and `navigator.standalone` (iOS Safari). Re-subscribes on the `change` event
 * so a later transition (e.g. user adds the site to the home screen mid-session)
 * is reflected, though in practice standalone state rarely changes at runtime.
 *
 * See `shared/lib/pwa/standalone.ts` for the underlying imperative check.
 */
export function useStandalone(): boolean {
    const [isStandalone, setIsStandalone] = useState(false);

    useEffect(() => {
        const update = (): void => setIsStandalone(isStandaloneMode());
        update();

        const mql = window.matchMedia?.(STANDALONE_MEDIA_QUERY);
        if (!mql) return;
        const onChange = (): void => update();
        mql.addEventListener('change', onChange);
        return () => mql.removeEventListener('change', onChange);
    }, []);

    return isStandalone;
}
