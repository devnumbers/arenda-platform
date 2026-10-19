/**
 * Pure gesture math shared by the app's custom scroll gestures (канон —
 * DESIGN.md §8; физика по эталону Apple/UIScrollView).
 *
 * Единственный потребитель — snap-rail (drag-прокси горизонтальных лент:
 * use-snap-rail-drag.ts, snap-rail-physics.ts). Модуль вырос из кастомного
 * pull-to-refresh (ADR 0031, вырезан 19.10.2026 — решение владельца); от
 * него остались две общие примитивы:
 *
 * - The rubber band is the reverse-engineered UIScrollView curve
 *   `b = (1 − 1/(x·c/H + 1))·H` with `c = 0.55` (beaucollins, 2013) —
 *   asymptotic, no hard clamp. WKWebView delegates its own bounce to the
 *   same UIScrollView physics, so this is literally what the platform does.
 * - The sliding-window velocity tracker feeds the release spring with the
 *   gesture's release velocity (WWDC23 "Animate with springs"); сэмплы
 *   старше окна мертвы — пауза перед отпусканием гасит скорость (#1349).
 */

/** UIScrollView rubber-band constant (measured de facto). */
export const RUBBER_BAND_C = 0.55;
/** Velocity samples older than this stop counting (ms). */
export const VELOCITY_WINDOW_MS = 100;

/** Native rubber band: visual offset for `dragPx` beyond the edge. */
export function rubberBandOffset(dragPx: number, viewportPx: number): number {
    if (dragPx <= 0 || viewportPx <= 0) return 0;
    return (1 - 1 / ((dragPx * RUBBER_BAND_C) / viewportPx + 1)) * viewportPx;
}

/** Sliding-window velocity tracker over (timeMs, positionPx) touch samples. */
export type VelocityTracker = {
    readonly add: (timeMs: number, positionPx: number) => void;
    /** Signed px/s over the window. `nowMs` — момент жеста, на который
     * считается скорость (отпускание указателя): сэмплы старше окна от
     * ЭТОГО момента мертвы — пауза перед отпусканием гасит скорость
     * (без этого stale-всплеск последнего движения переживал паузу и давал
     * ложный флик, #1349). Без аргумента — окно от последнего сэмпла. */
    readonly velocityPxPerS: (nowMs?: number) => number;
    readonly reset: () => void;
};

/** Tracks the last ~`windowMs` of samples; returns signed px/s (down +). */
export function createVelocityTracker(windowMs = VELOCITY_WINDOW_MS): VelocityTracker {
    let samples: Array<{ timeMs: number; positionPx: number }> = [];
    return {
        add: (timeMs, positionPx) => {
            samples.push({ timeMs, positionPx });
            const cutoff = timeMs - windowMs;
            samples = samples.filter((sample) => sample.timeMs >= cutoff);
        },
        velocityPxPerS: (nowMs) => {
            // Чистое чтение: окно считается на локальной копии, кэш сэмплов
            // не переписывается (геттер без побочных эффектов).
            const windowed =
                nowMs !== undefined
                    ? samples.filter((sample) => sample.timeMs >= nowMs - windowMs)
                    : samples;
            if (windowed.length < 2) return 0;
            const first = windowed[0];
            const last = windowed[windowed.length - 1];
            if (first === undefined || last === undefined) return 0;
            const dtMs = last.timeMs - first.timeMs;
            if (dtMs <= 0) return 0;
            return ((last.positionPx - first.positionPx) / dtMs) * 1000;
        },
        reset: () => {
            samples = [];
        },
    };
}
