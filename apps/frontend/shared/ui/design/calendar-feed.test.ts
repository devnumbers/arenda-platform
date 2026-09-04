import { describe, expect, it } from 'vitest';
import {
  calendarFeedStart,
  calendarMonthIndex,
  calendarMonthOf,
  calendarMonthOfIndex,
  clampMonthToMin,
  isoDateOf,
  listCalendarMonths,
} from './calendar-feed';

describe('calendar-feed', () => {
  it('месяц ISO-даты (UTC-якорь, без таймзон)', () => {
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

  it('сборка ISO из год/месяц/день', () => {
    expect(isoDateOf(2026, 7, 17)).toBe('2026-08-17');
    expect(isoDateOf(2026, 0, 1)).toBe('2026-01-01');
    expect(isoDateOf(2026, 11, 31)).toBe('2026-12-31');
  });

  it('месяц с учётом нижней границы: в минимальном году не раньше min', () => {
    const min = { year: 2026, month0: 8 };
    expect(clampMonthToMin(min, 2026, 3)).toBe(8);
    expect(clampMonthToMin(min, 2026, 8)).toBe(8);
    expect(clampMonthToMin(min, 2026, 9)).toBe(9);
    expect(clampMonthToMin(min, 2027, 3)).toBe(3);
  });
});
