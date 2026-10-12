import { describe, expect, it } from 'vitest';
import {
    PTR_PIN_PX,
    PTR_THRESHOLD_PX,
    RUBBER_BAND_C,
    SPRING_DAMPING,
    SPRING_STIFFNESS,
    VELOCITY_WINDOW_MS,
    createVelocityTracker,
    flickTriggersRefresh,
    rubberBandOffset,
    spinnerReveal,
    springPeak,
} from './pull-math';

// Математика нативного PTR-жеста (ADR 0031, ротация 12.10.2026;
// research/ios-ptr-native-feel.md): резинка UIScrollView, аналитика пика
// пружины отпускания, флик-триггер, скользящий velocity-трекер. Все функции
// чистые — канон node-env без DOM.

const H = 800; // высота вьюпорта-примера

describe('rubberBandOffset — нативная резинка UIScrollView', () => {
    it('ноль и отрицательный перетаск дают 0', () => {
        expect(rubberBandOffset(0, H)).toBe(0);
        expect(rubberBandOffset(-50, H)).toBe(0);
    });

    it('формула b = (1 − 1/(x·c/H + 1))·H: контрольные точки c = 0.55', () => {
        expect(RUBBER_BAND_C).toBe(0.55);
        // x = H → (1 − 1/1.55)·H
        expect(rubberBandOffset(H, H)).toBeCloseTo(283.87, 1);
        // x = 2H → (1 − 1/2.1)·H
        expect(rubberBandOffset(2 * H, H)).toBeCloseTo(419.05, 1);
    });

    it('порог 70px достигается перетаском ~140px — как нативный триггер', () => {
        expect(rubberBandOffset(139.5, H)).toBeCloseTo(PTR_THRESHOLD_PX, 0);
    });

    it('монотонна и асимптотична: потолка нет, но визуал < вьюпорта', () => {
        const at = (x: number): number => rubberBandOffset(x, H);
        expect(at(100)).toBeLessThan(at(200));
        expect(at(200)).toBeLessThan(at(400));
        expect(at(100_000)).toBeLessThan(H);
        // жёсткого клампа нет: дальний тяг продолжает расти
        expect(at(2_000)).toBeGreaterThan(at(1_000));
    });
});

describe('spinnerReveal — проявление без вращения при тяге', () => {
    it('рамп 0→1 на первых 50% порога, дальше клампится', () => {
        expect(spinnerReveal(0)).toBe(0);
        expect(spinnerReveal(-10)).toBe(0);
        expect(spinnerReveal(PTR_THRESHOLD_PX * 0.25)).toBeCloseTo(0.5, 6);
        expect(spinnerReveal(PTR_THRESHOLD_PX * 0.5)).toBe(1);
        expect(spinnerReveal(PTR_THRESHOLD_PX)).toBe(1);
        expect(spinnerReveal(PTR_THRESHOLD_PX * 4)).toBe(1);
    });
});

describe('springPeak — пик пружины отпускания (mass = 1)', () => {
    it('без скорости пик — текущая позиция', () => {
        expect(springPeak(70, 0)).toBeCloseTo(70, 6);
        expect(springPeak(0, 0)).toBe(0);
    });

    it('отрицательная скорость от неотрицательной позиции — уже за пиком', () => {
        expect(springPeak(50, -100)).toBeCloseTo(50, 6);
    });

    it('контрольная точка: x0=0, v0=1000px/s → пик ≈ 27.5px', () => {
        // γ = damping/2 = 13, ωd = sqrt(200 − 169) ≈ 5.568
        expect(SPRING_STIFFNESS).toBe(200);
        expect(SPRING_DAMPING).toBe(26);
        expect(springPeak(0, 1000)).toBeCloseTo(27.5, 0);
    });

    it('монотонен по скорости', () => {
        const p1 = springPeak(0, 1000);
        const p2 = springPeak(0, 2000);
        const p3 = springPeak(0, 3000);
        expect(p1).toBeLessThan(p2);
        expect(p2).toBeLessThan(p3);
    });
});

describe('flickTriggersRefresh — флик через overshoot резинки', () => {
    it('медленное отпускание ниже порога не триггерит', () => {
        expect(flickTriggersRefresh(50, 0)).toBe(false);
        expect(flickTriggersRefresh(50, 300)).toBe(false);
    });

    it('резкий флик досылает за порог и триггерит', () => {
        // peak(50, 3000) ≈ 120px ≥ 70
        expect(flickTriggersRefresh(50, 3000)).toBe(true);
        // чистый флик от верха без длинной тяги
        expect(flickTriggersRefresh(10, 4000)).toBe(true);
    });
});

describe('createVelocityTracker — скользящее окно сэмплов', () => {
    it('скорость по окну по умолчанию (VELOCITY_WINDOW_MS), в px/s', () => {
        const tracker = createVelocityTracker();
        tracker.add(0, 0);
        tracker.add(50, 100);
        tracker.add(100, 190);
        expect(VELOCITY_WINDOW_MS).toBe(100);
        expect(tracker.velocityPxPerS()).toBeCloseTo(1900, 0);
    });

    it('один сэмпл и нулевой интервал дают 0 (без деления на ноль)', () => {
        const tracker = createVelocityTracker();
        expect(tracker.velocityPxPerS()).toBe(0);
        tracker.add(10, 0);
        tracker.add(10, 50);
        expect(tracker.velocityPxPerS()).toBe(0);
    });

    it('reset обнуляет историю', () => {
        const tracker = createVelocityTracker();
        tracker.add(0, 0);
        tracker.add(100, 200);
        expect(tracker.velocityPxPerS()).toBeGreaterThan(0);
        tracker.reset();
        expect(tracker.velocityPxPerS()).toBe(0);
    });

    it('старые сэмплы за пределами окна выпадают', () => {
        const tracker = createVelocityTracker(100);
        tracker.add(0, 0);
        tracker.add(1000, 100);
        tracker.add(1050, 150);
        // окно [950..1050]: скорость только по свежим сэмплам = 1000px/s
        expect(tracker.velocityPxPerS()).toBeCloseTo(1000, 0);
    });
});

describe('константы канона', () => {
    it('порог 70px, пин 60px — высота UIRefreshControl', () => {
        expect(PTR_THRESHOLD_PX).toBe(70);
        expect(PTR_PIN_PX).toBe(60);
    });
});
