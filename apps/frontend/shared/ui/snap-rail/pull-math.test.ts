import { describe, expect, it } from 'vitest';
import {
    RUBBER_BAND_C,
    VELOCITY_WINDOW_MS,
    createVelocityTracker,
    rubberBandOffset,
} from './pull-math';

// Общая математика жестов (канон DESIGN.md §8): резинка UIScrollView и
// скользящий velocity-трекер. Все функции чистые — канон node-env без DOM.
// Модуль вырос из кастомного pull-to-refresh (ADR 0031, вырезан
// 19.10.2026); PTR-специфика (спиннер, флик, гейтинг) удалена вместе с ним.

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

    it('монотонна и асимптотична: потолка нет, но визуал < вьюпорта', () => {
        const at = (x: number): number => rubberBandOffset(x, H);
        expect(at(100)).toBeLessThan(at(200));
        expect(at(200)).toBeLessThan(at(400));
        expect(at(100_000)).toBeLessThan(H);
        // жёсткого клампа нет: дальний тяг продолжает расти
        expect(at(2_000)).toBeGreaterThan(at(1_000));
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
