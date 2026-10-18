/**
 * Pure math of the native-feel pull-to-refresh gesture (ADR 0031, rotated
 * 12.10.2026; research/ios-ptr-native-feel.md).
 *
 * Every value here is a measured or de-facto canonical constant of iOS:
 *
 * - The rubber band is the reverse-engineered UIScrollView curve
 *   `b = (1 − 1/(x·c/H + 1))·H` with `c = 0.55` (beaucollins, 2013) —
 *   asymptotic, no hard clamp. WKWebView delegates its own bounce to the
 *   same UIScrollView physics, so this is literally what the platform does.
 * - The release is a spring with the finger's release velocity fed in
 *   (WWDC23 "Animate with springs"): stiffness/damping/mass, near-critical
 *   damping with a light overshoot.
 * - During the pull the spinner only *reveals* (opacity/scale ramp); native
 *   UIRefreshControl does not rotate it until the refreshing phase starts.
 * - A fast flick triggers refresh by the spring overshooting past the
 *   threshold — the native path, replicated analytically here.
 *
 * The component (PullToRefresh.tsx) runs the same constants through
 * `animate()` of the `motion` library (semi-implicit spring integrator);
 * `springPeak` below is the closed form of that model (mass = 1), so the
 * flick decision matches what the animation will actually do. Сюда же
 * живёт чистый словарь гейтинга жеста (решение направления, чтение
 * overflow цепочки цели) — DOM читает компонент, решает предикат.
 */

/** Armed threshold: visual rubber-band offset that arms the refresh. */
export const PTR_THRESHOLD_PX = 70;
/** Content pin while refreshing — the UIRefreshControl height (~60pt). */
export const PTR_PIN_PX = 60;
/** UIScrollView rubber-band constant (measured de facto). */
export const RUBBER_BAND_C = 0.55;
/** Release/pin spring (mass = 1): ratio ≈ 0.92 — light overshoot. */
export const SPRING_STIFFNESS = 200;
export const SPRING_DAMPING = 26;
/** Velocity samples older than this stop counting (ms). */
export const VELOCITY_WINDOW_MS = 100;
/** Hysteresis before the gesture intent commits (~10px — канон жестов:
 *  направление фиксируется по доминанте оси, пока движение не дошло до
 *  порога, ни один жест не решён). */
export const PTR_INTENT_LOCK_PX = 10;
/** The reveal completes this far into the threshold (native feel: the
 *  control is fully visible about half-way to the trigger). */
const REVEAL_THRESHOLD_FRACTION = 0.5;

/** Итог разрешения направления касания. `horizontal` для PTR мёртв: жест
 *  завершается (свайпы каруселей/чипов не должны тянуть контент). */
export type GestureIntent = 'undecided' | 'vertical' | 'horizontal';

/**
 * Разрешение направления по суммарному смещению от точки касания. Вертикаль
 * — когда |dy| достигла гистерезиса и доминирует; при равенстве осей
 * (|dx| = |dy| ≥ порога) — горизонталь: fail-closed, неоднозначный жест не
 * захватываем. Знак не учитывается: тягу вверх отменяет компонент (жест
 * кончается), а не разрешатель направления.
 */
export function resolveGestureIntent(dx: number, dy: number): GestureIntent {
    const absX = Math.abs(dx);
    const absY = Math.abs(dy);
    if (Math.max(absX, absY) < PTR_INTENT_LOCK_PX) return 'undecided';
    return absY > absX ? 'vertical' : 'horizontal';
}

/** Чтение overflow одной поверхности цепочки касания (getComputedStyle). */
export type OverflowReading = { readonly x: string; readonly y: string };

/**
 * Перехватывает ли цепочка цели внутренний скроллер (конвенция изоляции
 * скроллеров, карта #1416): auto/scroll по любой оси у любого элемента
 * между точкой касания и корнем документа означает, что жест принадлежит
 * внутренней ленте/шиту/пикеру — а не документу, который тянет PTR.
 * Корневые значения (visible на документной поверхности, clip на html/body,
 * hidden под локом модальной поверхности) перехватом не считаются: html/body —
 * сам документный скроллер, а «открыта модальная поверхность» решается в
 * компоненте отдельной проверкой лока. */
export function interceptedByInnerScroller(overflows: ReadonlyArray<OverflowReading>): boolean {
    return overflows.some(
        ({ x, y }) => x === 'auto' || x === 'scroll' || y === 'auto' || y === 'scroll',
    );
}

/** Native rubber band: visual offset for `dragPx` beyond the edge. */
export function rubberBandOffset(dragPx: number, viewportPx: number): number {
    if (dragPx <= 0 || viewportPx <= 0) return 0;
    return (1 - 1 / ((dragPx * RUBBER_BAND_C) / viewportPx + 1)) * viewportPx;
}

/** Spinner reveal: opacity/scale ramp over the first part of the threshold. */
export function spinnerReveal(pullPx: number): number {
    const ratio = pullPx / (PTR_THRESHOLD_PX * REVEAL_THRESHOLD_FRACTION);
    return Math.min(Math.max(ratio, 0), 1);
}

/**
 * Peak displacement of the release spring launched at `x0` px with `v0` px/s
 * (downward positive). Closed form of the underdamped oscillator our
 * constants produce: γ = damping/2, ωd = sqrt(stiffness − γ²); the first
 * velocity zero is at t_p = atan2(v0·ωd, x0·ωd² + γ(v0 + γ·x0))/ωd.
 * When the velocity zero lies in the past (v0 ≤ 0 from a non-negative
 * position), the displacement only decays — the peak is `x0` itself.
 */
export function springPeak(x0: number, v0: number): number {
    if (v0 <= 0 && x0 >= 0) return x0;
    const gamma = SPRING_DAMPING / 2;
    const omegaD = Math.sqrt(SPRING_STIFFNESS - gamma * gamma);
    const numerator = v0 * omegaD;
    const denominator = x0 * omegaD * omegaD + gamma * (v0 + gamma * x0);
    let tPeak = Math.atan2(numerator, denominator) / omegaD;
    if (tPeak <= 0) return x0;
    const c = (v0 + gamma * x0) / omegaD;
    tPeak = Math.min(tPeak, Math.PI / omegaD);
    return Math.exp(-gamma * tPeak) * (x0 * Math.cos(omegaD * tPeak) + c * Math.sin(omegaD * tPeak));
}

/**
 * Would releasing at `pullPx` with `velocityPxPerS` carry the spring past
 * the armed threshold? This is the native flick: no long drag, the release
 * velocity itself overshoots the rubber band into the refresh.
 */
export function flickTriggersRefresh(pullPx: number, velocityPxPerS: number): boolean {
    return springPeak(pullPx, velocityPxPerS) >= PTR_THRESHOLD_PX;
}

/** Sliding-window velocity tracker over (timeMs, positionPx) touch samples. */
export type VelocityTracker = {
    readonly add: (timeMs: number, positionPx: number) => void;
    /** Signed px/s over the window. `nowMs` — момент жеста, на который
     * считается скорость (отпускание указателя): сэмплы старше окна от
     * ЭТОГО момента мертвы — пауза перед отпусканием гасит скорость
     * (без этого stale-всплеск последнего движения переживал паузу и давал
     * ложный флик, #1349). Без аргумента — прежнее поведение (окно от
     * последнего сэмпла, PTR). */
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
