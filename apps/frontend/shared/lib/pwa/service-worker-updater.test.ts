import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

/**
 * Drift-guard: the page-side ServiceWorkerUpdater sends
 * `{ type: 'SKIP_WAITING' }` to the waiting SW, and `public/sw.js` must keep a
 * matching message handler that calls `self.skipWaiting()`. This is the exact
 * seam that was broken before ADR 0032 — the handler existed but was never
 * triggered. If either side drifts, the update lifecycle silently stops
 * working again. Mirrors the cabinet-routes.test.ts sync guard.
 */
describe('SKIP_WAITING handler sync between page and service worker', () => {
    const swPath = resolve(process.cwd(), 'public/sw.js');
    const swSource = readFileSync(swPath, 'utf8');
    const updaterPath = resolve(
        process.cwd(),
        'shared/lib/pwa/ServiceWorkerUpdater.tsx',
    );
    const updaterSource = readFileSync(updaterPath, 'utf8');

    it('public/sw.js listens for the SKIP_WAITING message type', () => {
        // The page posts { type: 'SKIP_WAITING' }; the SW must match on this
        // exact string. Renaming it on one side breaks activation silently.
        expect(swSource, "public/sw.js missing 'SKIP_WAITING' marker").toContain(
            "'SKIP_WAITING'",
        );
    });

    it('public/sw.js calls self.skipWaiting() inside the message handler', () => {
        // The handler must actually activate — a listener that swallows the
        // message without calling skipWaiting() is the original bug.
        expect(swSource, 'public/sw.js missing self.skipWaiting() call').toContain(
            'self.skipWaiting()',
        );
    });

    it('ServiceWorkerUpdater posts the same SKIP_WAITING message type', () => {
        // Drift catcher on the page side: if the postMessage literal is ever
        // renamed/extracted to a constant without updating the SW, this fails.
        expect(
            updaterSource,
            "ServiceWorkerUpdater missing postMessage({ type: 'SKIP_WAITING' })",
        ).toContain("{ type: 'SKIP_WAITING' }");
    });
});
