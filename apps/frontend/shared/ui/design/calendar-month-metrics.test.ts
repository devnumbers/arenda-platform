import { describe, expect, it } from 'vitest';

import { calendarMonthGridRows, calendarMonthIntrinsicPx } from './calendar-month-metrics';

describe('calendar: геометрия блока месяца (план 051)', () => {
  it('ряды: пустые клетки + дни, по 7 на ряд', () => {
    expect(calendarMonthGridRows(28, 0)).toBe(4);
    expect(calendarMonthGridRows(31, 0)).toBe(5);
    expect(calendarMonthGridRows(31, 3)).toBe(5);
    expect(calendarMonthGridRows(30, 6)).toBe(6);
    expect(calendarMonthGridRows(31, 6)).toBe(6);
  });

  it('высота по ширине колонки: телефон 390', () => {
    // клетка (390−44)/7 ≈ 49.43; 6 рядов: 24+16+6×49.43+5×2 ≈ 346.6
    expect(calendarMonthIntrinsicPx({ rows: 6, columnWidth: 390 })).toBeCloseTo(346.57, 1);
    // 4 ряда (февраль без пустых): 24+16+4×49.43+3×2 ≈ 243.7
    expect(calendarMonthIntrinsicPx({ rows: 4, columnWidth: 390 })).toBeCloseTo(243.71, 1);
  });

  it('десктопная колонка 560 — прежняя оценка 480 была близка к 6 рядам', () => {
    expect(calendarMonthIntrinsicPx({ rows: 6, columnWidth: 560 })).toBeCloseTo(492.29, 1);
  });

  it('ширина ещё не замерена — телефонный фолбэк 360 (план 050)', () => {
    expect(calendarMonthIntrinsicPx({ rows: 6, columnWidth: null })).toBe(360);
  });
});
