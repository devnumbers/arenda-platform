import { describe, expect, it } from 'vitest';
import {
  addDays,
  calendarFeedStart,
  calendarMonthIndex,
  calendarMonthOf,
  calendarMonthOfIndex,
  clampMonthToMin,
  cmp,
  dateInMonth,
  dateToIso,
  daysOverdue,
  fromIso,
  inclusiveDays,
  isoDateOf,
  isoDayOfMonth,
  isoMonthNumber,
  isoYear,
  lastDayOfMonth,
  listCalendarMonths,
} from './calendar';

describe('calendar: ISO-парсинг', () => {
  it('fromIso — UTC-полночь, без таймзонных сюрпризов', () => {
    const date = fromIso('2026-08-17');
    expect(date.toISOString()).toBe('2026-08-17T00:00:00.000Z');
  });

  it('строковые парсеры без заводимого Date', () => {
    expect(isoYear('2026-08-17')).toBe(2026);
    expect(isoMonthNumber('2026-08-17')).toBe(8);
    expect(isoDayOfMonth('2026-08-17')).toBe(17);
  });

  it('dateToIso — Date в ISO-строку (UTC)', () => {
    expect(dateToIso(fromIso('2026-08-17'))).toBe('2026-08-17');
  });
});

describe('calendar: сравнение и арифметика', () => {
  it('cmp — лексикографическое сравнение ISO-строк', () => {
    expect(cmp('2026-08-17', '2026-08-17')).toBe(0);
    expect(cmp('2026-08-16', '2026-08-17')).toBe(-1);
    expect(cmp('2026-09-01', '2026-08-31')).toBe(1);
  });

  it('addDays — через границу месяца и года', () => {
    expect(addDays('2026-08-30', 3)).toBe('2026-09-02');
    expect(addDays('2026-12-30', 5)).toBe('2027-01-04');
    expect(addDays('2026-01-05', -10)).toBe('2025-12-26');
  });

  it('inclusiveDays — длина периода включительно', () => {
    expect(inclusiveDays('2026-08-03', '2026-08-14')).toBe(12);
    expect(inclusiveDays('2026-08-03', '2026-08-03')).toBe(1);
  });

  it('daysOverdue — просрочка в днях (сегодня параметром, ADR 0048)', () => {
    expect(daysOverdue('2026-08-14', '2026-08-17')).toBe(3);
    expect(daysOverdue('2026-08-17', '2026-08-17')).toBe(0);
    expect(daysOverdue('2026-08-20', '2026-08-17')).toBe(-3);
  });
});

describe('calendar: месяцы', () => {
  it('lastDayOfMonth: 31/30/28 и високосный февраль', () => {
    expect(lastDayOfMonth(2026, 7)).toBe(31);
    expect(lastDayOfMonth(2026, 8)).toBe(30);
    expect(lastDayOfMonth(2026, 1)).toBe(28);
    expect(lastDayOfMonth(2024, 1)).toBe(29);
  });

  it('dateInMonth — прижимание дня к концу месяца (31-е → 28/29/30-е)', () => {
    expect(dateInMonth(2026, 1, 31)).toBe('2026-02-28');
    expect(dateInMonth(2024, 1, 31)).toBe('2024-02-29');
    expect(dateInMonth(2026, 0, 31)).toBe('2026-01-31');
    // month0 нормализуется за пределами 0..11.
    expect(dateInMonth(2026, 12, 5)).toBe('2027-01-05');
  });
});

describe('calendar: месяцы ленты', () => {
  it('месяц ISO-даты (UTC-якорь)', () => {
    expect(calendarMonthOf('2026-08-17')).toEqual({ year: 2026, month0: 7 });
    expect(calendarMonthOf('2027-01-01')).toEqual({ year: 2027, month0: 0 });
    expect(calendarMonthOf('2024-02-29')).toEqual({ year: 2024, month0: 1 });
  });

  it('абсолютный индекс месяца: year*12 + month0, туда-обратно', () => {
    expect(calendarMonthIndex({ year: 2026, month0: 7 })).toBe(2026 * 12 + 7);
    expect(calendarMonthOfIndex(2026 * 12 + 11)).toEqual({ year: 2026, month0: 11 });
    // Декабрь → январь следующего года.
    expect(calendarMonthOfIndex(2026 * 12 + 12)).toEqual({ year: 2027, month0: 0 });
    const ref = { year: 2031, month0: 4 };
    expect(calendarMonthOfIndex(calendarMonthIndex(ref))).toEqual(ref);
  });

  it('лента месяцев через границу года', () => {
    expect(listCalendarMonths({ year: 2026, month0: 10 }, 4)).toEqual([
      { year: 2026, month0: 10 },
      { year: 2026, month0: 11 },
      { year: 2027, month0: 0 },
      { year: 2027, month0: 1 },
    ]);
    expect(listCalendarMonths({ year: 2026, month0: 7 }, 0)).toEqual([]);
  });

  it('старт ленты — самый ранний из «сегодня» и значения (якорь правки #502)', () => {
    const today = '2026-09-04';
    expect(calendarFeedStart(today, null)).toEqual({ year: 2026, month0: 8 });
    expect(calendarFeedStart(today, '2026-12-01')).toEqual({ year: 2026, month0: 8 });
    expect(calendarFeedStart(today, '2025-03-10')).toEqual({ year: 2025, month0: 2 });
    expect(calendarFeedStart(today, '2028-01-01')).toEqual({ year: 2026, month0: 8 });
  });

  it('месяц с учётом нижней границы: в минимальном году не раньше min', () => {
    const min = { year: 2026, month0: 8 };
    expect(clampMonthToMin(min, 2026, 3)).toBe(8);
    expect(clampMonthToMin(min, 2026, 8)).toBe(8);
    expect(clampMonthToMin(min, 2026, 9)).toBe(9);
    expect(clampMonthToMin(min, 2027, 3)).toBe(3);
  });

  it('сборка ISO из год/месяц/день', () => {
    expect(isoDateOf(2026, 7, 17)).toBe('2026-08-17');
    expect(isoDateOf(2026, 0, 1)).toBe('2026-01-01');
    expect(isoDateOf(2026, 11, 31)).toBe('2026-12-31');
  });
});
