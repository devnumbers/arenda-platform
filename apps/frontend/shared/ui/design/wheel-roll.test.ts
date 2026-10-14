import { describe, expect, it } from 'vitest';
import { WHEEL_ROLL_MAX_MS, WHEEL_ROLL_MIN_MS, rollDurationMs, rollEase } from './wheel-roll';

describe('rollDurationMs', () => {
  it('короткая докрутка — нижний порог', () => {
    expect(rollDurationMs(48)).toBe(WHEEL_ROLL_MIN_MS);
    expect(rollDurationMs(0)).toBe(WHEEL_ROLL_MIN_MS);
  });

  it('длительность растёт с дистанцией', () => {
    expect(rollDurationMs(240)).toBeGreaterThan(rollDurationMs(48));
    expect(rollDurationMs(240)).toBeCloseTo(144, 0);
  });

  it('дальний прыжок зажат верхним порогом, знак не важен', () => {
    expect(rollDurationMs(5000)).toBe(WHEEL_ROLL_MAX_MS);
    expect(rollDurationMs(-5000)).toBe(WHEEL_ROLL_MAX_MS);
  });
});

describe('rollEase (cubic-bezier(0.32, 0.72, 0, 1) = --dl-ease)', () => {
  it('края точны', () => {
    expect(rollEase(0)).toBe(0);
    expect(rollEase(1)).toBe(1);
  });

  it('монотонна и выше диагонали в начале (ease-out характер)', () => {
    let prev = 0;
    for (let i = 1; i <= 20; i += 1) {
      const x = i / 20;
      const y = rollEase(x);
      expect(y).toBeGreaterThanOrEqual(prev);
      prev = y;
    }
    expect(rollEase(0.25)).toBeGreaterThan(0.25);
    expect(rollEase(0.5)).toBeGreaterThan(0.5);
  });
});
