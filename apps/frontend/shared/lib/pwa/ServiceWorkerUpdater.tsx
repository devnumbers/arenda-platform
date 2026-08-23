'use client';

import { useEffect, useRef, type JSX } from 'react';
import { usePathname } from 'next/navigation';
import { reportClientError } from '@/shared/lib/error-reporting/report-client-error';

/**
 * Silent service worker update lifecycle.
 *
 * Closes the gap left by `ServiceWorkerRegister`: after a deploy, the browser
 * installs the new SW in the background and it sits in `waiting` because the
 * page never asked it to take over. The `SKIP_WAITING` message handler in
 * `public/sw.js` was dead code — this component wires it up.
 *
 * Strategy is silent (no toast, no UI): the new SW is activated only at
 * moments guaranteed not to interrupt the user — on a route change (the user
 * is not mid-form) or when the app is hidden (backgrounded / tab switched).
 * Reload happens on the same route change, or deferred until the user returns
 * if the SW took over while the app was hidden. See ADR 0032.
 *
 * Guards:
 * - First visit (`navigator.serviceWorker.controller === null` on mount): the
 *   SW is only being registered now, there is nothing to update — bail out so
 *   we never reload a freshly-opened app in the user's face.
 * - `isReloading` ref prevents a double reload when `controllerchange` fires
 *   in multiple cabinet tabs simultaneously.
 *
 * This component is independent of `ServiceWorkerRegister`: it obtains the
 * registration via `navigator.serviceWorker.ready`, so it does not matter who
 * performed the registration. It must be mounted on every cabinet route (it
 * lives in `CabinetLayout` next to `ServiceWorkerRegister`) so the route-change
 * trigger fires across the whole cabinet.
 */
export function ServiceWorkerUpdater(): JSX.Element | null {
    const pathname = usePathname();
    // Refs survive re-renders without triggering them. Used as mutable flags
    // across the listeners attached in the effects below.
    const registrationRef = useRef<ServiceWorkerRegistration | null>(null);
    const isReloading = useRef(false);
    const pendingSkipWaiting = useRef(false);
    const needsReloadOnVisible = useRef(false);
    // Marks the first run of the pathname effect so we don't treat the initial
    // mount as a "route change" — otherwise a SW that was already waiting at
    // mount time would be activated instantly on the first render.
    const isFirstPathnameRun = useRef(true);

    // Shared activation helper: posts SKIP_WAITING to the waiting worker and
    // clears the pending flag. Used by both the route-change and the
    // visibility-change triggers so the message format lives in one place.
    const activateWaiting = (): void => {
        const waiting = registrationRef.current?.waiting;
        if (!waiting) return;
        waiting.postMessage({ type: 'SKIP_WAITING' });
        pendingSkipWaiting.current = false;
    };

    // Trigger: route change. When the user navigates between cabinet screens
    // (so is not mid-form), activate the waiting SW if one is ready. pathname
    // is an effect dependency, so this runs on every navigation. The first run
    // (mount) is skipped — it is not a real route change.
    useEffect(() => {
        if (isFirstPathnameRun.current) {
            isFirstPathnameRun.current = false;
            return;
        }
        if (pendingSkipWaiting.current) {
            activateWaiting();
        }
    }, [pathname]);

    useEffect(() => {
        if (typeof window === 'undefined') return;
        if (!('serviceWorker' in navigator)) return;
        // First visit: no controller yet, the SW is only being registered by
        // ServiceWorkerRegister. Nothing to update — activating the logic now
        // could reload a freshly-opened app.
        if (navigator.serviceWorker.controller === null) return;

        let cancelled = false;
        let intervalId: number | undefined;
        // Captured per updatefound: the specific installing worker we attached
        // a statechange listener to. Kept so cleanup can remove it even after
        // the worker has moved on to waiting/active (registration.installing
        // would then be null).
        let trackedWorker: ServiceWorker | null = null;

        const reload = (): void => {
            if (isReloading.current) return;
            isReloading.current = true;
            window.location.reload();
        };

        const onStateChange = (event: Event): void => {
            const sw = event.target as ServiceWorker;
            if (sw.state === 'installed') {
                // A new SW finished installing and is now waiting. Flag it so
                // the next route change or visibilitychange→hidden activates it.
                pendingSkipWaiting.current = true;
            }
        };

        const onUpdateFound = (): void => {
            const installing = registrationRef.current?.installing;
            if (!installing) return;
            trackedWorker = installing;
            installing.addEventListener('statechange', onStateChange);
        };

        const onControllerChange = (): void => {
            if (document.hidden) {
                // SW took over while the app was in the background — defer the
                // reload until the user returns, so we don't reload a tab the
                // user is not looking at (and so the reload is visible).
                needsReloadOnVisible.current = true;
            } else {
                reload();
            }
        };

        const onVisibilityChange = (): void => {
            if (document.visibilityState === 'hidden') {
                // User is leaving — safe to activate the waiting SW in the
                // background. The reload, if any, happens on return.
                if (pendingSkipWaiting.current) {
                    activateWaiting();
                }
            } else {
                // User returned. If the SW took over while hidden, reload now.
                if (needsReloadOnVisible.current) {
                    needsReloadOnVisible.current = false;
                    reload();
                }
            }
        };

        // navigator.serviceWorker.ready resolves once there is an active
        // registration. Decouples this component from ServiceWorkerRegister.
        navigator.serviceWorker.ready.then((registration) => {
            if (cancelled) return;
            registrationRef.current = registration;

            // A SW may already be waiting at mount time (installed before this
            // component mounted, e.g. after a re-navigation within the cabinet).
            if (registration.waiting) {
                pendingSkipWaiting.current = true;
            }

            registration.addEventListener('updatefound', onUpdateFound);
            navigator.serviceWorker.addEventListener('controllerchange', onControllerChange);
            document.addEventListener('visibilitychange', onVisibilityChange);

            // Poll for updates every 30 minutes. Without this, the browser only
            // checks on navigation and at most once per 24h. The browser itself
            // throttles excessive update() calls, so this is safe.
            intervalId = window.setInterval(() => {
                registration.update().catch(() => {
                    // update() rejects on transient failures (e.g. offline);
                    // the next interval or the next navigation will retry.
                });
            }, UPDATE_INTERVAL_MS);
        }).catch((error: unknown) => {
            // Covers both failure sources of the chain: `ready` rejecting
            // (SW infrastructure broken — registration failed / browser gave
            // up) and an exception inside the then-callback itself; either way
            // silent-update watching is simply off. Reported for diagnostics,
            // not shown to the user: the update strategy must never interrupt
            // the session.
            reportClientError(
                `serviceWorker update wiring failed: ${error instanceof Error ? error.message : String(error)}`,
            );
        });

        return () => {
            cancelled = true;
            if (intervalId !== undefined) {
                window.clearInterval(intervalId);
            }
            const registration = registrationRef.current;
            if (registration) {
                registration.removeEventListener('updatefound', onUpdateFound);
            }
            // Remove the statechange listener from the worker we actually
            // attached it to — not from registration.installing, which may by
            // now be null (the worker has advanced to waiting/active).
            if (trackedWorker) {
                trackedWorker.removeEventListener('statechange', onStateChange);
            }
            navigator.serviceWorker.removeEventListener('controllerchange', onControllerChange);
            document.removeEventListener('visibilitychange', onVisibilityChange);
        };
    }, []);

    return null;
}

/** Interval between periodic `registration.update()` checks (30 minutes). */
const UPDATE_INTERVAL_MS = 30 * 60 * 1000;
