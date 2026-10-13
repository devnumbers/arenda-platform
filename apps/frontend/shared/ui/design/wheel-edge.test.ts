import { describe, expect, it } from 'vitest';
import {
  WHEEL_MAX_FLING_ROWS,
  WHEEL_NEAR_EDGE_ROWS,
  isNearEnd,
  isNearStart,
} from './wheel-edge';

describe('wheel-edge — зона триггера у края линейной ленты', () => {
  it('зона шире максимального пролёта одного флика — стена недостижима одним жестом', () => {
    // Жёсткий флик покрывает v₀·τ ≈ 8000px/s · 325ms ≈ 2600px ≈ 54 ряда.
    // Зона триггера обязана быть больше: из зоны физический край ленты
    // не долететь одним фликом, достройка всегда успевает (баг #1427:
    // триггер за 2 ряда — инерция пролетала их за ~12мс и гасилась
    // о стену раньше, чем React достраивал ленту).
    expect(WHEEL_NEAR_EDGE_ROWS).toBeGreaterThan(WHEEL_MAX_FLING_ROWS);
  });

  it('isNearEnd: правда с ряда length − зона и глубже (раньше было только за 2 ряда)', () => {
    const length = 100;
    expect(isNearEnd(length - WHEEL_NEAR_EDGE_ROWS, length)).toBe(true);
    expect(isNearEnd(length - WHEEL_NEAR_EDGE_ROWS - 1, length)).toBe(false);
    expect(isNearEnd(length - 1, length)).toBe(true);
    // Короткая лента целиком в зоне: первый же скролл удлиняет её.
    expect(isNearEnd(0, 26)).toBe(true);
  });

  it('isNearStart: правда с ряда зона − 1 и ближе к началу', () => {
    expect(isNearStart(WHEEL_NEAR_EDGE_ROWS - 1)).toBe(true);
    expect(isNearStart(WHEEL_NEAR_EDGE_ROWS)).toBe(false);
    expect(isNearStart(0)).toBe(true);
  });
});
