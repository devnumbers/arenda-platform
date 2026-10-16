'use client';

import {
    useCallback,
    useEffect,
    useRef,
    useState,
    type JSX,
    type RefObject,
} from 'react';
import { useIsFetching, useQueryClient } from '@tanstack/react-query';
import { animate, type AnimationPlaybackControls } from 'framer-motion';
import clsx from 'clsx';
import { isIosDevice } from '@/shared/lib/pwa/ios';
import { useStandalone } from '@/shared/lib/hooks/useStandalone';
import {
    PTR_PIN_PX,
    PTR_THRESHOLD_PX,
    SPRING_DAMPING,
    SPRING_STIFFNESS,
    createVelocityTracker,
    flickTriggersRefresh,
    rubberBandOffset,
    spinnerReveal,
} from './pull-math';
import styles from './PullToRefresh.module.css';

/**
 * Native-feel pull-to-refresh for the iOS standalone app PWA.
 *
 * WebKit disables the OS pull-to-refresh for `display: standalone/fullscreen`
 * home-screen apps with no way to re-enable it, so on iOS this component
 * reproduces the iOS `UIRefreshControl` feel with the platform's own math
 * (ADR 0031, rotated 12.10.2026; research/ios-ptr-native-feel.md):
 *
 * - The pull tracks the finger through the reverse-engineered UIScrollView
 *   rubber band `(1 − 1/(x·0.55/H + 1))·H` — resistance grows with distance,
 *   no hard clamp (pull-math.ts).
 * - The spinner is a system-style 12-spoke activity indicator. During the
 *   pull it only *reveals* (opacity/scale ramp over the first half of the
 *   threshold); native UIRefreshControl does not rotate it until the
 *   refreshing phase starts.
 * - Release runs a real spring with the finger's release velocity fed in
 *   (framer-motion `animate`, stiffness/damping from pull-math): a fast flick
 *   overshoots past the threshold and triggers the refresh — the native
 *   path, decided analytically by `flickTriggersRefresh`.
 * - On trigger the content springs to a 60px pin while queries refetch,
 *   then springs back.
 *
 * On every other platform the component stays inactive on purpose (owner
 * decision 12.10.2026 — ADR 0031): an installed standalone WebAPK on Android
 * keeps the browser's native pull-to-refresh (a full document reload — the
 * T-Bank PWA experience), and binding our own listeners would suppress that
 * system gesture.
 *
 * The gesture triggers a *soft* refresh — `queryClient.invalidateQueries()` —
 * not a page reload. Pulling in new *code* (a new deploy) is owned by the
 * silent service-worker update lifecycle (`ServiceWorkerUpdater`, ADR 0032).
 *
 * See `docs/adr/0031-pwa-pull-to-refresh.md` for the full decision record.
 *
 * The component needs to drive `transform` on the app content node, so it
 * accepts a `contentRef` that the layout attaches to its content wrapper
 * (ScreenLayout since #568, ex-CabinetLayout). Fixed chrome (TabBar,
 * desktop sidebar and pills) is `position: fixed` and deliberately stays
 * put — exactly like native apps, where the status bar and tab bar do not
 * move.
 */

/** Hard cap on how long the spinner can stay up, regardless of fetch state. */
const MAX_REFRESH_MS = 3000;
/** `invalidateQueries()` registers refetches asynchronously — the first
 *  `refreshing` render can still see `isFetching === 0`. Without this grace
 *  the spinner would dismiss before the refetches even start. */
const REFETCH_GRACE_MS = 120;
/** Class toggled on <html> during the gesture — fallback overscroll guard
 *  (see PullToRefresh.module.css). */
const OVERSCROLL_SUPPRESS_CLASS = 'ptr-suppress-overscroll';

type Phase = 'idle' | 'pulling' | 'armed' | 'refreshing';

export type PullToRefreshProps = {
    /** Ref to the app content node that the gesture translates. */
    readonly contentRef: RefObject<HTMLDivElement | null>;
};

export function PullToRefresh({ contentRef }: PullToRefreshProps): JSX.Element | null {
    const isStandalone = useStandalone();
    // Platform gate (owner decision 12.10.2026, ADR 0031): the custom gesture
    // runs only on iOS, where WebKit kills the native pull-to-refresh in
    // standalone mode. Everywhere else — Android WebAPK above all — the
    // system/browser gesture stays available and must not be suppressed.
    // `isStandalone` is false during SSR and the hydration render, so the
    // navigator-reading `isIosDevice()` only evaluates after mount.
    const isActive = isStandalone && isIosDevice();
    const queryClient = useQueryClient();
    const isFetching = useIsFetching();

    const [phase, setPhase] = useState<Phase>('idle');

    // Touch tracking + current phase live in refs so window-level listeners
    // bind once and are never recreated mid-gesture (a recreated listener
    // would miss the in-flight `touchmove` stream).
    const startYRef = useRef<number | null>(null);
    const viewportRef = useRef<number>(0);
    const trackingRef = useRef(false);
    const phaseRef = useRef<Phase>(phase);
    const pullRef = useRef<number>(0);
    // Latest visual offset actually applied to the content node — the pull
    // and every spring write through `applyContentTransform`, so this is the
    // truthful "where the content is now" (a pin spring can still be in
    // flight with overshoot when the refresh settles).
    const offsetRef = useRef<number>(0);
    // Release/pin spring in flight (framer-motion); stopped on a new gesture
    // or phase change so animations never fight each other.
    const animRef = useRef<AnimationPlaybackControls | null>(null);
    useEffect(() => {
        phaseRef.current = phase;
    }, [phase]);

    // Deadline caps spinner visibility so a hung backend can't trap the user.
    const refreshDeadlineRef = useRef<number | null>(null);

    const applyContentTransform = useCallback(
        (offsetPx: number): void => {
            offsetRef.current = offsetPx;
            const el = contentRef.current;
            if (!el) return;
            // Springs drive the value per frame — no CSS transition ever
            // participates; during the pull the same call follows the finger.
            el.style.transition = '';
            el.style.transform = offsetPx === 0 ? '' : `translateY(${offsetPx}px)`;
        },
        [contentRef],
    );

    const stopAnimation = useCallback((): void => {
        animRef.current?.stop();
        animRef.current = null;
    }, []);

    // The spinner reveal follows the gesture imperatively — the same
    // frame-by-frame path as the content transform. Writing it through
    // React state re-rendered the component on every `touchmove` (up to
    // 120/s on ProMotion) for a style change (#1349).
    const spinnerRef = useRef<HTMLSpanElement | null>(null);
    const spinnerFadeRef = useRef<Animation | null>(null);

    const applySpinnerReveal = useCallback((visualPx: number): void => {
        const el = spinnerRef.current;
        if (el === null) return;
        const reveal = spinnerReveal(visualPx);
        el.style.opacity = String(reveal);
        el.style.transform = `translate(-50%, 0) scale(${reveal})`;
    }, []);

    /** Mirrored exit for the `.refreshing` fade-in: the spinner dissolves
     * over the house duration/curve (WAAPI не читает var() — зеркало
     * `--dl-duration-fast`/`--dl-ease`, как в LiveValue; #1349), while the
     * content springs back, instead of hard-cutting the moment the class
     * drops. */
    const fadeSpinnerOut = useCallback((): void => {
        const el = spinnerRef.current;
        if (el === null) return;
        spinnerFadeRef.current?.cancel();
        // Pin the currently visible opacity inline BEFORE the `.refreshing`
        // class drops, or the base class would cut it to 0 first.
        if (el.style.opacity === '') {
            el.style.opacity = '1';
        }
        const fade = el.animate(
            [{ opacity: el.style.opacity }, { opacity: '0' }],
            { duration: 250, easing: 'cubic-bezier(0.32, 0.72, 0, 1)' },
        );
        spinnerFadeRef.current = fade;
        fade.onfinish = (): void => {
            el.style.opacity = '';
            el.style.transform = '';
            if (spinnerFadeRef.current === fade) {
                spinnerFadeRef.current = null;
            }
        };
    }, []);

    /** Spring the content from `from` to `to` with the release velocity. */
    const springTo = useCallback(
        (from: number, to: number, velocityPxPerS: number, onDone?: () => void): void => {
            stopAnimation();
            animRef.current = animate(from, to, {
                type: 'spring',
                stiffness: SPRING_STIFFNESS,
                damping: SPRING_DAMPING,
                velocity: velocityPxPerS,
                onUpdate: (latest: number) => {
                    applyContentTransform(latest);
                    // Below-threshold release: the reveal rides the same
                    // spring down instead of freezing at the last pull
                    // value (#1349). During refreshing the spinner is owned
                    // by the `.refreshing` class — reveal writes stay off.
                    if (to === 0 && phaseRef.current !== 'refreshing') {
                        applySpinnerReveal(latest);
                    }
                },
                onComplete: () => {
                    animRef.current = null;
                    onDone?.();
                },
            });
        },
        [applyContentTransform, applySpinnerReveal, stopAnimation],
    );

    // Fallback overscroll guard: toggled on <html> while a pull is in flight.
    // The primary mechanism is preventDefault() on touchmove; this covers any
    // residual native bounce on devices where preventDefault is not enough.
    const setOverscrollGuard = useCallback((active: boolean): void => {
        document.documentElement.classList.toggle(OVERSCROLL_SUPPRESS_CLASS, active);
    }, []);

    const beginRefresh = useCallback(
        (velocityPxPerS: number): void => {
            setPhase('refreshing');
            refreshDeadlineRef.current = Date.now() + MAX_REFRESH_MS;
            // The release overshoot carries into the pin: the same spring
            // settles the content at the pin offset.
            springTo(pullRef.current, PTR_PIN_PX, velocityPxPerS);
            // Fire-and-forget: invalidate marks all active queries stale and
            // triggers their refetch. Spinner visibility is bound to
            // `useIsFetching()` below, not to this promise.
            void queryClient.invalidateQueries();
        },
        [queryClient, springTo],
    );

    const reset = useCallback((): void => {
        fadeSpinnerOut();
        setPhase('idle');
        setOverscrollGuard(false);
    }, [fadeSpinnerOut, setOverscrollGuard]);

    // Resolve the spinner once fetches settle OR the deadline elapses.
    useEffect(() => {
        if (phase !== 'refreshing') return;

        const deadline = refreshDeadlineRef.current;
        const finish = (): void => {
            // From the offset the content is actually at right now — the pin
            // spring may still be in flight past/before the pin value.
            springTo(offsetRef.current, 0, 0, reset);
        };

        if (isFetching === 0) {
            // Refetches may not have registered yet — dismiss only after the
            // grace window (cancelled if `isFetching` flips to > 0 in between,
            // which re-runs this effect and clears the timer below).
            const grace = window.setTimeout(finish, REFETCH_GRACE_MS);
            return (): void => window.clearTimeout(grace);
        }

        if (deadline !== null) {
            const remaining = deadline - Date.now();
            const timer = window.setTimeout(finish, Math.max(0, remaining));
            return (): void => window.clearTimeout(timer);
        }
        // No deadline recorded for this refresh cycle — nothing to clean up.
        return undefined;
    }, [phase, isFetching, reset, springTo]);

    // Bind touch listeners on `window` once per active state.
    useEffect(() => {
        if (!isActive) return;
        // The velocity tracker lives with the listeners: nothing outside
        // this effect reads it.
        const velocity = createVelocityTracker();

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
            stopAnimation();
            // A spinner fade-out from the previous refresh may still be
            // running — a new pull owns the inline styles from here.
            spinnerFadeRef.current?.cancel();
            startYRef.current = touch.clientY;
            viewportRef.current = window.innerHeight;
            velocity.reset();
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
                    stopAnimation();
                    applyContentTransform(0);
                    applySpinnerReveal(0);
                    reset();
                }
                return;
            }

            // Native rubber band: resistance grows with distance, no clamp.
            const visual = rubberBandOffset(deltaY, viewportRef.current);
            velocity.add(event.timeStamp, visual);
            pullRef.current = visual;
            setPhase(visual >= PTR_THRESHOLD_PX ? 'armed' : 'pulling');
            // Content follows the finger frame-by-frame — both the content
            // transform and the spinner reveal are imperative writes.
            applyContentTransform(visual);
            applySpinnerReveal(visual);

            // Suppress native overscroll bounce only while actively pulling at
            // the top of the page.
            if (window.scrollY <= 0) {
                event.preventDefault();
            }
        };

        const onTouchEnd = (): void => {
            if (!trackingRef.current) return;
            trackingRef.current = false;
            startYRef.current = null;
            const releaseVelocity = velocity.velocityPxPerS();
            if (phaseRef.current === 'armed' || flickTriggersRefresh(pullRef.current, releaseVelocity)) {
                // Armed release or a flick whose spring overshoots the
                // threshold — the release velocity carries into the pin.
                beginRefresh(releaseVelocity);
                // Guard stays on: content is pinned and we still don't want
                // native overscroll fighting the spring.
            } else {
                // Released below threshold — spring back with the release
                // velocity; a fast release overshoots further, natively.
                setOverscrollGuard(false);
                springTo(pullRef.current, 0, releaseVelocity, reset);
            }
        };

        const onTouchCancel = (): void => {
            if (!trackingRef.current) return;
            trackingRef.current = false;
            startYRef.current = null;
            if (phaseRef.current !== 'refreshing') {
                stopAnimation();
                setOverscrollGuard(false);
                springTo(pullRef.current, 0, 0, reset);
            }
        };

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
    }, [isActive, applyContentTransform, applySpinnerReveal, beginRefresh, reset, setOverscrollGuard, springTo, stopAnimation]);

    // Clean up the inline transform, running animation and overscroll guard
    // if the component unmounts mid-gesture.
    useEffect(() => {
        return (): void => {
            stopAnimation();
            applyContentTransform(0);
            setOverscrollGuard(false);
        };
    }, [applyContentTransform, setOverscrollGuard, stopAnimation]);

    if (!isActive) return null;

    // The reveal is written imperatively (applySpinnerReveal): the pull
    // path never re-renders. `phase` re-renders only on phase changes; the
    // `.refreshing` class takes the spinner to full opacity with its
    // fade-in transition.

    const spinnerClass = clsx(styles.spinner, phase === 'refreshing' && styles.refreshing);

    return (
        <span ref={spinnerRef} className={spinnerClass} aria-hidden="true">
            <svg viewBox="0 0 24 24" width="24" height="24" fill="none">
                {Array.from({ length: 12 }, (_, spoke) => (
                    <line
                        key={spoke}
                        x1="12"
                        y1="1.2"
                        x2="12"
                        y2="4.4"
                        stroke="currentColor"
                        strokeWidth="2"
                        strokeLinecap="round"
                        transform={`rotate(${spoke * 30} 12 12)`}
                        // Graduated spoke opacity — the system activity
                        // indicator's trailing fade, animated by the spin.
                        opacity={1 - (spoke / 11) * 0.75}
                    />
                ))}
            </svg>
        </span>
    );
}
