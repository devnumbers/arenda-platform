'use client';

import { useCallback, useEffect, useRef, useState, type CSSProperties, type JSX } from 'react';
import { useIsFetching, useQueryClient } from '@tanstack/react-query';
import { useStandalone } from '@/shared/lib/hooks/useStandalone';
import styles from './PullToRefresh.module.css';

/**
 * Pull-to-refresh gesture for the standalone cabinet PWA.
 *
 * Native pull-to-refresh is disabled by the OS in PWA standalone mode on iOS
 * (and not shown on Android either), leaving users no in-app way to refresh
 * data. This component implements a custom touch gesture that triggers a
 * *soft* refresh — `queryClient.invalidateQueries()` — rather than a full page
 * reload. Pulling in new *code* (a new deploy) is the responsibility of the
 * silent service-worker update lifecycle (`ServiceWorkerUpdater`, ADR 0032);
 * this component only refreshes the react-query cache.
 *
 * See `docs/adr/0031-pwa-pull-to-refresh.md` for the full decision record.
 *
 * Activation: only mounted in standalone mode. In a regular browser the user
 * keeps their native platform behaviour.
 *
 * Mechanics:
 * - the gesture starts only when the document is scrolled to the very top
 *   (`scrollTop <= 0`) on `touchstart`;
 * - the visual pull distance is the real drag distance multiplied by the
 *   resistance factor (rubber-band feel);
 * - once the pull exceeds `THRESHOLD_PX`, the indicator enters its "armed"
 *   state — releasing the finger triggers a refresh;
 * - on refresh, `invalidateQueries()` runs (fire-and-forget); the spinner
 *   stays visible while `useIsFetching() > 0`, capped at `MAX_REFRESH_MS` so a
 *   slow/hung backend cannot trap the user under a spinner.
 */

/** Pull distance (after resistance) that arms the refresh. */
const THRESHOLD_PX = 70;
/** Visual pull = real drag × RESISTANCE (0–1). Lower = stiffer. */
const RESISTANCE = 0.5;
/** Hard cap on how long the spinner can stay up, regardless of fetch state. */
const MAX_REFRESH_MS = 3000;
/** Resting position of the indicator while idle (fully hidden above viewport). */
const HIDDEN_Y_PX = -60;
/** Resting position of the indicator while a refresh is in flight. */
const REFRESHING_Y_PX = 0;

type Phase = 'idle' | 'pulling' | 'armed' | 'refreshing';

export function PullToRefresh(): JSX.Element | null {
    const isStandalone = useStandalone();
    const queryClient = useQueryClient();
    // `useIsFetching` is reactive on the query cache — it ticks down to 0 once
    // every active refetch triggered by the invalidation settles.
    const isFetching = useIsFetching();

    const [phase, setPhase] = useState<Phase>('idle');
    const [pullY, setPullY] = useState<number>(HIDDEN_Y_PX);

    // Touch tracking and the current phase live in refs so the window-level
    // touch listeners are bound once and never re-created mid-gesture. If the
    // effect re-ran on every phase change, a `touchmove` stream could be lost
    // because the new listener would be waiting for a fresh `touchstart`.
    const startYRef = useRef<number | null>(null);
    const trackingRef = useRef(false);
    const phaseRef = useRef<Phase>(phase);
    // Keep the ref in sync with the state outside of render (eslint rule
    // `react-hooks/refs` forbids writing refs during render).
    useEffect(() => {
        phaseRef.current = phase;
    }, [phase]);

    // Guard against the spinner lingering forever when a query is stuck in
    // retry-backoff or the backend is unresponsive.
    const refreshDeadlineRef = useRef<number | null>(null);

    const beginRefresh = useCallback(() => {
        setPullY(REFRESHING_Y_PX);
        setPhase('refreshing');
        refreshDeadlineRef.current = Date.now() + MAX_REFRESH_MS;
        // Fire-and-forget: invalidate marks all active queries stale and
        // triggers their refetch. Spinner visibility is bound to
        // `useIsFetching()` below, not to this promise.
        void queryClient.invalidateQueries();
    }, [queryClient]);

    // Resolve the spinner once fetches settle OR the deadline elapses.
    useEffect(() => {
        if (phase !== 'refreshing') return;

        const deadline = refreshDeadlineRef.current;
        const finish = (): void => {
            setPhase('idle');
            setPullY(HIDDEN_Y_PX);
        };

        if (isFetching === 0) {
            finish();
            return;
        }

        if (deadline !== null) {
            const remaining = deadline - Date.now();
            const timer = window.setTimeout(finish, Math.max(0, remaining));
            return () => window.clearTimeout(timer);
        }
    }, [phase, isFetching]);

    // Bind touch listeners on `window` exactly once per standalone state.
    // The document is the scroll container (no nested overflow scroller in the
    // cabinet layout), so window-level touch events are correct here.
    useEffect(() => {
        if (!isStandalone) return;

        const onTouchStart = (event: TouchEvent): void => {
            // Only single-finger gestures qualify; ignore multi-touch.
            if (event.touches.length !== 1) return;
            // Gesture only begins when the page is pinned to the top — pulling
            // from the middle of a scrolled page must scroll, not refresh.
            if (window.scrollY > 0) return;
            startYRef.current = event.touches[0].clientY;
            trackingRef.current = true;
        };

        const onTouchMove = (event: TouchEvent): void => {
            if (!trackingRef.current || startYRef.current === null) return;
            const deltaY = event.touches[0].clientY - startYRef.current;
            // Downward pull is positive deltaY. An upward drag aborts tracking.
            if (deltaY <= 0) {
                if (phaseRef.current !== 'refreshing') {
                    setPhase('idle');
                    setPullY(HIDDEN_Y_PX);
                }
                return;
            }

            const resisted = deltaY * RESISTANCE;
            // Clamp the visual pull so the indicator never drags absurdly far.
            const clamped = Math.min(resisted, THRESHOLD_PX * 1.8);
            setPullY(clamped);
            setPhase(clamped >= THRESHOLD_PX ? 'armed' : 'pulling');

            // Prevent the browser's native overscroll bounce while we are
            // actively rendering a custom pull gesture at the top of the page.
            if (window.scrollY <= 0) {
                event.preventDefault();
            }
        };

        const endTracking = (): void => {
            if (!trackingRef.current) return;
            trackingRef.current = false;
            startYRef.current = null;
            if (phaseRef.current === 'armed') {
                beginRefresh();
            } else if (phaseRef.current !== 'refreshing') {
                setPhase('idle');
                setPullY(HIDDEN_Y_PX);
            }
        };

        const onTouchEnd = (): void => endTracking();
        const onTouchCancel = (): void => endTracking();

        // `passive: false` is required on `touchmove` because we call
        // `preventDefault()` to suppress native overscroll during the pull.
        window.addEventListener('touchstart', onTouchStart, { passive: true });
        window.addEventListener('touchmove', onTouchMove, { passive: false });
        window.addEventListener('touchend', onTouchEnd, { passive: true });
        window.addEventListener('touchcancel', onTouchCancel, { passive: true });

        return () => {
            window.removeEventListener('touchstart', onTouchStart);
            window.removeEventListener('touchmove', onTouchMove);
            window.removeEventListener('touchend', onTouchEnd);
            window.removeEventListener('touchcancel', onTouchCancel);
        };
    }, [isStandalone, beginRefresh]);

    if (!isStandalone) return null;

    const isPulling = phase === 'pulling' || phase === 'armed';
    const indicatorClass = [
        styles.indicator,
        phase === 'armed' || phase === 'refreshing' ? styles.active : '',
        phase === 'refreshing' ? styles.refreshing : '',
    ]
        .filter(Boolean)
        .join(' ');

    // While pulling, the indicator follows the finger (transform from CSS via
    // the `--pull-y` variable). While refreshing, the `.refreshing` class
    // overrides the transform to pin the spinner at the resting position.
    const cssVars = isPulling
        ? ({ '--pull-y': `${pullY}px` } as CSSProperties)
        : undefined;

    return (
        <div className={indicatorClass} style={cssVars} aria-hidden="true">
            <div className={styles.spinner} />
        </div>
    );
}
