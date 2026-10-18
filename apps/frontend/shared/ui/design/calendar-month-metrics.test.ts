import { describe, expect, it } from 'vitest';

import {
  calendarMonthGridRows,
  calendarMonthIntrinsicPx,
  calendarMonthRowHeights,
  rowOffset,
} from './calendar-month-metrics';

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

describe('calendar: высоты виртуальных строк (план 052; отступ у всех — план 053)', () => {
  it('высоты строк: межстрочный отступ 32 у каждой строки', () => {
    const heights = calendarMonthRowHeights(
      [
        { year: 2026, month0: 7 },
        { year: 2026, month0: 8 },
        { year: 2026, month0: 9 },
      ],
      390,
    );
    const sixRows = calendarMonthIntrinsicPx({ rows: 6, columnWidth: 390 });
    const fiveRows = calendarMonthIntrinsicPx({ rows: 5, columnWidth: 390 });
    expect(heights[0]).toBeCloseTo(sixRows + 32, 1);
    expect(heights[1]).toBeCloseTo(fiveRows + 32, 1);
    // Отступ у всех, включая последнюю: при append'е бывшая последняя
    // строка обязана сохранить свой отступ, иначе новые строки поедут
    // на 32px выше суммы высот. Хвостовой отступ растворяется в FEED_TAIL.
    expect(heights[2]).toBeCloseTo(fiveRows + 32, 1);
  });

  it('rowOffset: смещение строки = сумма предыдущих высот', () => {
    const heights = [100, 132, 120];
    expect(rowOffset(heights, 0)).toBe(0);
    expect(rowOffset(heights, 1)).toBe(100);
    expect(rowOffset(heights, 2)).toBe(232);
    expect(rowOffset(heights, 5)).toBe(352);
  });

  it('append (план 054): рост в конец идёт за краем вьюпорта — новые строки продолжают старый итог', () => {
    const width = 390;
    const oldMonths = [
      { year: 2026, month0: 7 },
      { year: 2026, month0: 8 },
    ];
    const newMonths = [
      ...oldMonths,
      { year: 2026, month0: 9 },
      { year: 2026, month0: 10 },
      { year: 2026, month0: 11 },
    ];
    const oldHeights = calendarMonthRowHeights(oldMonths, width);
    const newHeights = calendarMonthRowHeights(newMonths, width);
    // Офсеты существующих строк неизменны: append не трогает начало окна.
    for (let i = 0; i < oldMonths.length; i += 1) {
      expect(rowOffset(newHeights, i)).toBeCloseTo(rowOffset(oldHeights, i), 1);
    }
    expect(rowOffset(newHeights, oldMonths.length)).toBeCloseTo(
      rowOffset(oldHeights, oldMonths.length),
      1,
    );
  });

  it('prepend (план 054): досыпка истории сверху сдвигает все существующие строки на дельту — позицию держит библиотека по ключам', () => {
    const width = 390;
    const oldMonths = [
      { year: 2026, month0: 7 },
      { year: 2026, month0: 8 },
    ];
    const newMonths = [
      { year: 2026, month0: 4 },
      { year: 2026, month0: 5 },
      { year: 2026, month0: 6 },
      ...oldMonths,
    ];
    const oldHeights = calendarMonthRowHeights(oldMonths, width);
    const newHeights = calendarMonthRowHeights(newMonths, width);
    const prepended = newMonths.length - oldMonths.length;
    // Смещение первой старой строки = сумма высот новых строк (у всех
    // равномерный межстрочный отступ — дельта чисто аддитивна).
    const delta = rowOffset(newHeights, prepended);
    for (let i = 0; i < oldMonths.length; i += 1) {
      const before = rowOffset(oldHeights, i);
      const after = rowOffset(newHeights, prepended + i);
      expect(after - before).toBeCloseTo(delta, 1);
    }
  });
});
