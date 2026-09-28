'use client';

import { useEffect, useState } from 'react';

const REDUCED_MOTION_MEDIA_QUERY = '(prefers-reduced-motion: reduce)';

/**
 * Reactive `prefers-reduced-motion` flag.
 *
 * Returns `false` on the server and during the first client render (to match
 * hydration), then resolves to the real media value and re-subscribes on the
 * `change` event. Motion consumers use it to shorten their JS-side waits —
 * the CSS side shortens itself through the `prefers-reduced-motion` media
 * overrides on the design tokens (tokens.css, §8: анимацию не отключаем,
 * а укорачиваем до ~150ms-порядка).
 */
export function useReducedMotion(): boolean {
    const [reduced, setReduced] = useState(false);

    useEffect(() => {
        const mql = window.matchMedia(REDUCED_MOTION_MEDIA_QUERY);
        const onChange = (): void => setReduced(mql.matches);
        onChange();
        mql.addEventListener('change', onChange);
        return () => mql.removeEventListener('change', onChange);
    }, []);

    return reduced;
}
