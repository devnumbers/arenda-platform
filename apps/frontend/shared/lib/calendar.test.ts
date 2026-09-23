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
  dateToIsoLocal,
  daysOverdue,
  fromIso,
  fullMonthsBetween,
  inclusiveDays,
  isoDateOf,
  isoDayOfMonth,
  isoMonthNumber,
  isoYear,
  lastDayOfMonth,
  listCalendarMonths,
  booleanRunSegments,
  calendarDayDisabled,
  calendarPastFloor,
  pickIsoRange,
  rangeFeedWindow,
  settleIsoRange,
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

  it('dateToIsoLocal — календарный день по локальным часам браузера (#624)', () => {
    const wallClock = new Date(2026, 7, 17, 13, 40);
    expect(dateToIsoLocal(wallClock)).toBe('2026-08-17');
    expect(dateToIsoLocal(new Date(2026, 0, 3, 0, 5))).toBe('2026-01-03');
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

describe('calendar: выбор диапазона (пикер периода операций)', () => {
  it('первый тап задаёт границу без конца; применение сводится к одному дню', () => {
    const draft = pickIsoRange({ start: '2026-09-01', end: '2026-09-30' }, '2026-08-15');
    expect(draft).toEqual({ start: '2026-08-15', end: null });
    expect(settleIsoRange(draft)).toEqual({ from: '2026-08-15', to: '2026-08-15' });
  });

  it('второй тап правее — завершает диапазон, следующий тап перезапускает', () => {
    let draft = pickIsoRange({ start: '2026-09-01', end: null }, '2026-09-19');
    expect(draft).toEqual({ start: '2026-09-01', end: '2026-09-19' });
    draft = pickIsoRange(draft, '2026-09-20');
    expect(draft).toEqual({ start: '2026-09-20', end: null });
  });

  it('второй тап левее завершает диапазон задом наперёд: 5→1 = период 1–5', () => {
    expect(pickIsoRange({ start: '2026-09-10', end: null }, '2026-09-05')).toEqual({
      start: '2026-09-05',
      end: '2026-09-10',
    });
    expect(pickIsoRange({ start: '2026-09-05', end: null }, '2026-09-01')).toEqual({
      start: '2026-09-01',
      end: '2026-09-05',
    });
  });

  it('тап по тому же числу — период одного дня', () => {
    expect(pickIsoRange({ start: '2026-09-05', end: null }, '2026-09-05')).toEqual({
      start: '2026-09-05',
      end: '2026-09-05',
    });
  });

  it('завершённый черновик применяется как есть', () => {
    expect(settleIsoRange({ start: '2026-09-01', end: '2026-09-30' })).toEqual({
      from: '2026-09-01',
      to: '2026-09-30',
    });
  });

  it('пустой старт без применённого периода (#670): первый тап задаёт границу', () => {
    const draft = pickIsoRange(null, '2026-09-15');
    expect(draft).toEqual({ start: '2026-09-15', end: null });
    expect(settleIsoRange(draft)).toEqual({ from: '2026-09-15', to: '2026-09-15' });
  });

  it('пустой старт: второй тап левее завершает диапазон задом наперёд', () => {
    let draft = pickIsoRange(null, '2026-09-10');
    expect(draft).toEqual({ start: '2026-09-10', end: null });
    draft = pickIsoRange(draft, '2026-09-05');
    expect(draft).toEqual({ start: '2026-09-05', end: '2026-09-10' });
  });
});

describe('calendar: окно ленты пикера диапазона', () => {
  it('свежий период — от 5 месяцев назад до 2 месяцев вперёд от текущего', () => {
    expect(rangeFeedWindow({ from: '2026-09-01', to: '2026-09-30' }, '2026-09-19')).toEqual({
      first: { year: 2026, month0: 3 },
      last: { year: 2026, month0: 10 },
    });
  });

  it('глубокий период — старт от его начала минус месяц (глубже дорисуется скроллом)', () => {
    expect(rangeFeedWindow({ from: '2024-03-10', to: '2024-03-25' }, '2026-09-19')).toEqual({
      first: { year: 2024, month0: 1 },
      last: { year: 2026, month0: 10 },
    });
  });

  it('пустой выбор (#670) — контекст пяти месяцев назад, преселекта нет', () => {
    expect(rangeFeedWindow(null, '2026-09-19')).toEqual({
      first: { year: 2026, month0: 3 },
      last: { year: 2026, month0: 10 },
    });
  });
});

describe('calendar: отрезки подложки недели диапазона', () => {
  it('находит непрерывные отрезки true', () => {
    expect(booleanRunSegments([false, true, true, true, false, true, true])).toEqual([
      [1, 3],
      [5, 6],
    ]);
  });

  it('полная неделя, пустая неделя, одиночный день', () => {
    expect(booleanRunSegments([true, true, true, true, true, true, true])).toEqual([[0, 6]]);
    expect(booleanRunSegments([false, false, false, false, false, false, false])).toEqual([]);
    expect(booleanRunSegments([false, false, true, false, false, false, false])).toEqual([[2, 2]]);
  });
});

describe('calendar: полных месяцев между датами', () => {
  it('считает целые календарные месяцы по дню начала', () => {
    expect(fullMonthsBetween('2026-05-10', '2026-09-07')).toBe(3);
    expect(fullMonthsBetween('2026-05-10', '2026-09-10')).toBe(4);
    expect(fullMonthsBetween('2025-09-07', '2026-09-07')).toBe(12);
  });

  it('меньше месяца и та же дата — ноль', () => {
    expect(fullMonthsBetween('2026-09-05', '2026-09-07')).toBe(0);
    expect(fullMonthsBetween('2026-09-07', '2026-09-07')).toBe(0);
  });

  it('границы месяцев прижимаются к коротким месяцам', () => {
    // 31 января + 1 месяц = 28 февраля (не високосный) — полный месяц прошёл.
    expect(fullMonthsBetween('2026-01-31', '2026-02-28')).toBe(1);
    expect(fullMonthsBetween('2026-01-31', '2026-02-27')).toBe(0);
  });

  it('обратный интервал не уходит в минус', () => {
    expect(fullMonthsBetween('2026-09-07', '2026-05-10')).toBe(0);
  });
});

describe('calendar: пикер завершения — прошлое открыто до начала (#805)', () => {
  const today = '2026-09-23';
  const completionArgs = {
    today,
    value: null,
    minDate: '2026-01-23' as const,
    maxDate: '2026-09-23' as const,
  };

  it('пол прошлого: сегодня по умолчанию, minDate при allowPast', () => {
    expect(calendarPastFloor(today, undefined, false)).toBe(today);
    expect(calendarPastFloor(today, '2026-01-23', false)).toBe(today);
    expect(calendarPastFloor(today, undefined, true)).toBe(today);
    expect(calendarPastFloor(today, '2026-01-23', true)).toBe('2026-01-23');
  });

  it('allowPast: период [minDate, today] тапабелен, края погашены', () => {
    const day = (iso: string) => calendarDayDisabled({ iso, ...completionArgs, allowPast: true });
    expect(day('2026-03-10')).toBe(false);
    expect(day('2026-01-23')).toBe(false);
    expect(day(today)).toBe(false);
    expect(day('2026-01-22')).toBe(true);
    expect(day('2026-09-24')).toBe(true);
  });

  it('без allowPast прошлое закрыто, значение-якорь тапабельно (правка #502)', () => {
    const day = (iso: string, value: string | null) =>
      calendarDayDisabled({ iso, today, value, minDate: '2026-01-23', maxDate: undefined, allowPast: false });
    expect(day('2026-09-22', null)).toBe(true);
    expect(day(today, null)).toBe(false);
    expect(day('2026-08-01', '2026-08-01')).toBe(false);
    expect(day('2026-08-01', null)).toBe(true);
    expect(day('2026-01-10', '2026-01-10')).toBe(true);
  });

  it('старт ленты уходит под пол allowPast, не теряя якорь значения', () => {
    const floor = { year: 2026, month0: 0 };
    expect(calendarFeedStart(today, null, floor)).toEqual({ year: 2026, month0: 0 });
    expect(calendarFeedStart(today, '2026-03-10', floor)).toEqual({ year: 2026, month0: 0 });
    expect(calendarFeedStart(today, '2025-12-01', floor)).toEqual({ year: 2025, month0: 11 });
    expect(calendarFeedStart(today, null)).toEqual({ year: 2026, month0: 8 });
  });
});
