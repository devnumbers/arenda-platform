'use client';

import {
    useCallback,
    useEffect,
    useRef,
    useState,
    type CSSProperties,
    type JSX,
    type RefObject,
} from 'react';
import { useIsFetching, useQueryClient } from '@tanstack/react-query';
import clsx from 'clsx';
import { useStandalone } from '@/shared/lib/hooks/useStandalone';
import styles from './PullToRefresh.module.css';

/**
 * Native-feel pull-to-refresh for the standalone cabinet PWA.
 *
 * Native pull-to-refresh is disabled by the OS in PWA standalone mode on iOS
 * (and not shown on Android either). This component reproduces the iOS
 * `UIRefreshControl` feel: pulling down moves the page content itself (not an
 * overlay), a thin grey spinner scales up and accumulates rotation above the
 * content, and on release past the threshold the content stays pinned while
 * the spinner spins until the refresh settles.
 *
 * The gesture triggers a *soft* refresh — `queryClient.invalidateQueries()` —
 * not a page reload. Pulling in new *code* (a new deploy) is owned by the
 * silent service-worker update lifecycle (`ServiceWorkerUpdater`, ADR 0032).
 *
 * See `docs/adr/0031-pwa-pull-to-refresh.md` for the full decision record.
 *
 * The component needs to drive `transform` on the cabinet content node, so it
 * accepts a `contentRef` that the layout attaches to its `.content` element.
 * `Sidebar` and `BottomNav` are `position: fixed` and deliberately stay put —
 * exactly like native apps, where the status bar and tab bar do not move.
 */

/** Real drag distance (after resistance) that arms the refresh. */
const THRESHOLD_PX = 70;
/** Visual pull = real drag × RESISTANCE (0–1). Lower = stiffer. */
const RESISTANCE = 0.5;
/** Hard cap on how long the spinner can stay up, regardless of fetch state. */
const MAX_REFRESH_MS = 3000;
/** Content offset pinned while a refresh is in flight. */
const REFRESHING_OFFSET_PX = 60;
/** CSS back-out curve with a light overshoot — rubber-band snap-back feel. */
const SNAP_BACK_TRANSITION = 'transform 0.3s cubic-bezier(0.34, 1.56, 0.64, 1)';
/** Degrees of spinner rotation per pixel of pull — "winding" feel. */
const ROTATE_DEG_PER_PX = 2;
/** Class toggled on <html> during the gesture — fallback overscroll guard
 *  (see PullToRefresh.module.css). */
const OVERSCROLL_SUPPRESS_CLASS = 'ptr-suppress-overscroll';

type Phase = 'idle' | 'pulling' | 'armed' | 'refreshing';

export type PullToRefreshProps = {
    /** Ref to the cabinet content node that the gesture translates. */
    readonly contentRef: RefObject<HTMLDivElement | null>;
};

export function PullToRefresh({ contentRef }: PullToRefreshProps): JSX.Element | null {
    const isStandalone = useStandalone();
    const queryClient = useQueryClient();
    const isFetching = useIsFetching();

    const [phase, setPhase] = useState<Phase>('idle');
    const [pullY, setPullY] = useState(0);

    // Touch tracking + current phase live in refs so window-level listeners
    // bind once and are never recreated mid-gesture (a recreated listener
    // would miss the in-flight `touchmove` stream).
    const startYRef = useRef<number | null>(null);
    const trackingRef = useRef(false);
    const phaseRef = useRef<Phase>(phase);
    useEffect(() => {
        phaseRef.current = phase;
    }, [phase]);

    // Deadline caps spinner visibility so a hung backend can't trap the user.
    const refreshDeadlineRef = useRef<number | null>(null);

    const applyContentTransform = useCallback(
        (offsetPx: number, withTransition: boolean): void => {
            const el = contentRef.current;
            if (!el) return;
            el.style.transform = offsetPx === 0 ? '' : `translateY(${offsetPx}px)`;
            el.style.transition = withTransition ? SNAP_BACK_TRANSITION : '';
        },
        [contentRef],
    );

    // Fallback overscroll guard: toggled on <html> while a pull is in flight.
    // The primary mechanism is preventDefault() on touchmove; this covers any
    // residual native bounce on devices where preventDefault is not enough.
    const setOverscrollGuard = useCallback((active: boolean): void => {
        document.documentElement.classList.toggle(OVERSCROLL_SUPPRESS_CLASS, active);
    }, []);

    const beginRefresh = useCallback((): void => {
        setPhase('refreshing');
        refreshDeadlineRef.current = Date.now() + MAX_REFRESH_MS;
        applyContentTransform(REFRESHING_OFFSET_PX, true);
        // Fire-and-forget: invalidate marks all active queries stale and
        // triggers their refetch. Spinner visibility is bound to
        // `useIsFetching()` below, not to this promise.
        void queryClient.invalidateQueries();
    }, [applyContentTransform, queryClient]);

    const reset = useCallback(
        (withTransition: boolean): void => {
            setPhase('idle');
            setPullY(0);
            applyContentTransform(0, withTransition);
            setOverscrollGuard(false);
        },
        [applyContentTransform, setOverscrollGuard],
    );

    // Resolve the spinner once fetches settle OR the deadline elapses.
    useEffect(() => {
        if (phase !== 'refreshing') return;

        const deadline = refreshDeadlineRef.current;
        const finish = (): void => reset(true);

        if (isFetching === 0) {
            finish();
            return;
        }

        if (deadline !== null) {
            const remaining = deadline - Date.now();
            const timer = window.setTimeout(finish, Math.max(0, remaining));
            return (): void => window.clearTimeout(timer);
        }
        // No deadline recorded for this refresh cycle — nothing to clean up.
        return undefined;
    }, [phase, isFetching, reset]);

    // Bind touch listeners on `window` once per standalone state.
    useEffect(() => {
        if (!isStandalone) return;

        const onTouchStart = (event: TouchEvent): void => {
            // Only single-finger gestures qualify; ignore multi-touch.
            if (event.touches.length !== 1) return;
            const touch = event.touches[0];
            if (touch === undefined) return;
            // A refresh is already in flight (content pinned, spinner
            // spinning) — ignore a new touch so it can't overwrite the
            // pinned offset mid-refresh.
            if (phaseRef.current === 'refreshing') return;
            // Gesture begins only when the page is pinned to the top.
            if (window.scrollY > 0) return;
            startYRef.current = touch.clientY;
            trackingRef.current = true;
            setOverscrollGuard(true);
        };

        const onTouchMove = (event: TouchEvent): void => {
            if (!trackingRef.current || startYRef.current === null) return;
            const touch = event.touches[0];
            if (touch === undefined) return;
            const deltaY = touch.clientY - startYRef.current;
            // Downward pull is positive. An upward drag aborts tracking.
            if (deltaY <= 0) {
                if (phaseRef.current !== 'refreshing') {
                    reset(false);
                }
                return;
            }

            const resisted = deltaY * RESISTANCE;
            const clamped = Math.min(resisted, THRESHOLD_PX * 1.8);
            setPullY(clamped);
            setPhase(clamped >= THRESHOLD_PX ? 'armed' : 'pulling');
            // Content follows the finger with no transition during the pull.
            applyContentTransform(clamped, false);

            // Suppress native overscroll bounce only while actively pulling at
            // the top of the page.
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
                // Guard stays on: content is pinned at the refresh offset and
                // we still don't want native overscroll fighting it.
            } else if (phaseRef.current !== 'refreshing') {
                // Released below threshold — snap content back with overshoot.
                reset(true);
            }
        };

        const onTouchEnd = (): void => endTracking();
        const onTouchCancel = (): void => endTracking();

        // `passive: false` on touchmove is required for `preventDefault()`.
        window.addEventListener('touchstart', onTouchStart, { passive: true });
        window.addEventListener('touchmove', onTouchMove, { passive: false });
        window.addEventListener('touchend', onTouchEnd, { passive: true });
        window.addEventListener('touchcancel', onTouchCancel, { passive: true });

        return (): void => {
            window.removeEventListener('touchstart', onTouchStart);
            window.removeEventListener('touchmove', onTouchMove);
            window.removeEventListener('touchend', onTouchEnd);
            window.removeEventListener('touchcancel', onTouchCancel);
        };
    }, [isStandalone, applyContentTransform, beginRefresh, reset, setOverscrollGuard]);

    // Clean up the inline transform and overscroll guard if the component
    // unmounts mid-gesture.
    useEffect(() => {
        return (): void => {
            applyContentTransform(0, false);
            setOverscrollGuard(false);
        };
    }, [applyContentTransform, setOverscrollGuard]);

    if (!isStandalone) return null;

    // Progressive spinner transform during the pull: scale ramps 0→1 as the
    // pull approaches the threshold; rotation accumulates for a "winding"
    // feel. `translate(-50%, 0)` keeps the spinner horizontally centered and
    // must be preserved by every transform variant.
    const ratio = Math.min(pullY / THRESHOLD_PX, 1);
    const spinnerStyle: CSSProperties =
        phase === 'refreshing'
            ? {}
            : {
                  transform: `translate(-50%, 0) rotate(${pullY * ROTATE_DEG_PER_PX}deg) scale(${ratio})`,
              };

    const spinnerClass = clsx(
        styles.spinner,
        phase === 'refreshing' && styles.refreshing,
        (phase === 'pulling' || phase === 'armed') && styles[phase],
    );

    return <div className={spinnerClass} style={spinnerStyle} aria-hidden="true" />;
}
