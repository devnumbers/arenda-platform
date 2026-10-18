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
  daysUntil,
  fromIso,
  fullMonthsBetween,
  inclusiveDays,
  isoDateOf,
  isoTodayInTimezone,
  isoDayOfMonth,
  isoMonthNumber,
  isoYear,
  lastDayOfMonth,
  listCalendarMonths,
  booleanRunSegments,
  calendarDayDisabled,
  calendarFeedWindow,
  calendarPastFloor,
  type CalendarMonthRef,
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
    expect(dateToIsoLocal(new Date(2026, 7, 27, 23, 59))).toBe('2026-08-27');
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

  it('daysUntil — дни до даты, зеркало серверного daysUntil (ADR 0053 §2)', () => {
    expect(daysUntil('2026-08-17', '2026-08-20')).toBe(3);
    expect(daysUntil('2026-08-17', '2026-08-17')).toBe(0);
    expect(daysUntil('2026-08-20', '2026-08-17')).toBe(-3);
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

describe('calendar: окно ленты пикера диапазона (план 054 — низ жёсткий, верх бесконечен)', () => {
  it('свежий период — от месяца начала минус месяц до пары приглушённых за «сегодня»', () => {
    expect(rangeFeedWindow({ from: '2026-09-01', to: '2026-09-30' }, '2026-09-19')).toEqual({
      first: { year: 2025, month0: 9 },
      count: 14,
      startEdge: null,
      forwardEdge: { year: 2026, month0: 10 },
    });
  });

  it('глубокий период — низ окна у его начала, глубже дорастёт prepend\'ом', () => {
    expect(rangeFeedWindow({ from: '2024-03-10', to: '2024-03-25' }, '2026-09-19')).toEqual({
      first: { year: 2024, month0: 1 },
      count: 34,
      startEdge: null,
      forwardEdge: { year: 2026, month0: 10 },
    });
  });

  it('пустой выбор (#670) — стартовый запас назад без преселекта', () => {
    expect(rangeFeedWindow(null, '2026-09-19')).toEqual({
      first: { year: 2025, month0: 9 },
      count: 14,
      startEdge: null,
      forwardEdge: { year: 2026, month0: 10 },
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
    expect(calendarPastFloor(today, '2026-01-23', true)).toBe('2026-01-23');
    // allowPast без minDate — не этот режим: пол снимается целиком (#1393),
    // см. describe «задним числом без пола».
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

describe('calendar: задним числом без пола (#1393)', () => {
  const today = '2026-10-13';

  it('allowPast без minDate — пола нет: calendarPastFloor отдаёт null', () => {
    expect(calendarPastFloor(today, undefined, true)).toBeNull();
  });

  it('без пола весь прошедший тапабелен, будущее гасит maxDate', () => {
    const day = (iso: string) =>
      calendarDayDisabled({
        iso,
        today,
        value: null,
        minDate: undefined,
        maxDate: today,
        allowPast: true,
      });
    expect(day('2026-10-12')).toBe(false);
    expect(day('2020-01-01')).toBe(false);
    expect(day('1999-12-31')).toBe(false);
    expect(day(today)).toBe(false);
    expect(day('2026-10-14')).toBe(true);
  });

  it('без пола и без maxDate закрытых дней нет вовсе', () => {
    const day = (iso: string) =>
      calendarDayDisabled({
        iso,
        today,
        value: null,
        minDate: undefined,
        maxDate: undefined,
        allowPast: true,
      });
    expect(day('2026-10-12')).toBe(false);
    expect(day('2000-02-29')).toBe(false);
    expect(day('2026-10-14')).toBe(false);
  });

  it('старт ленты без пола — месяц today; якорь значения в прошлом уводит старт к себе', () => {
    expect(calendarFeedStart(today, null)).toEqual({ year: 2026, month0: 9 });
    expect(calendarFeedStart(today, '2020-03-05')).toEqual({ year: 2020, month0: 2 });
    // Якорь глубже любого технического пола — лента открывается у значения,
    // назад дорисуется по доскроллу.
    expect(calendarFeedStart(today, '2020-03-05', { year: 2021, month0: 5 })).toEqual({
      year: 2020,
      month0: 2,
    });
  });
});

describe('calendar: «сегодня» в поясе профиля (#1394)', () => {
  it('считает календарную дату момента в заданном IANA-поясе', () => {
    // UTC 12:00 13-го — на Камчатке (UTC+12) уже 00:00 14-го.
    expect(isoTodayInTimezone('UTC', new Date('2026-10-13T12:00:00Z'))).toBe('2026-10-13');
    expect(
      isoTodayInTimezone('Asia/Kamchatka', new Date('2026-10-13T12:00:00Z')),
    ).toBe('2026-10-14');
    // UTC 06:00 13-го — в Лос-Анджелесе (UTC-7) ещё 23:00 12-го.
    expect(
      isoTodayInTimezone('America/Los_Angeles', new Date('2026-10-13T06:00:00Z')),
    ).toBe('2026-10-12');
    // Москва (UTC+3): граница суток в 21:00 UTC.
    expect(isoTodayInTimezone('Europe/Moscow', new Date('2026-10-13T20:59:00Z'))).toBe(
      '2026-10-13',
    );
    expect(isoTodayInTimezone('Europe/Moscow', new Date('2026-10-13T21:00:00Z'))).toBe(
      '2026-10-14',
    );
  });

  it('без пояса — платформенный дефолт Europe/Moscow (ADR 0048)', () => {
    const now = new Date('2026-10-13T21:00:00Z');
    expect(isoTodayInTimezone(null, now)).toBe('2026-10-14');
    expect(isoTodayInTimezone(undefined, now)).toBe(isoTodayInTimezone('Europe/Moscow', now));
  });

  it('невалидный пояс не роняет рендер — локальная дата устройства', () => {
    const now = new Date('2026-10-13T12:00:00Z');
    expect(isoTodayInTimezone('Not/AZone', now)).toBe(dateToIsoLocal(now));
  });
});

describe('calendar: окно ленты одиночного пикера (план 054 — Т-Банк: низ будущее, верх прошлое)', () => {
  const october2026: CalendarMonthRef = { year: 2026, month0: 9 };

  it('дефолт (пол = сегодня): оба края жёсткие, ровно +2 приглушённых за «сегодня»', () => {
    const window = calendarFeedWindow({
      anchorMonth: october2026,
      todayMonth: october2026,
      pastFloorMonth: october2026,
      maxDateMonth: undefined,
    });
    expect(window.startEdge).toEqual({ year: 2026, month0: 7 });
    expect(window.forwardEdge).toBeNull();
    expect(window.first).toEqual({ year: 2026, month0: 7 });
    expect(window.count).toBe(5);
  });

  it('завершение аренды (#805: minDate + maxDate): края у пола и у «сегодня»', () => {
    const window = calendarFeedWindow({
      anchorMonth: { year: 2026, month0: 5 },
      todayMonth: october2026,
      pastFloorMonth: { year: 2026, month0: 5 },
      maxDateMonth: october2026,
    });
    expect(window.startEdge).toEqual({ year: 2026, month0: 3 });
    expect(window.forwardEdge).toEqual({ year: 2026, month0: 11 });
    expect(window.count).toBe(9);
  });

  it('без пола (#1393, операции задним числом): верх бесконечен, старт — запас 12 до якоря', () => {
    const window = calendarFeedWindow({
      anchorMonth: october2026,
      todayMonth: october2026,
      maxDateMonth: undefined,
    });
    expect(window.startEdge).toBeNull();
    expect(window.forwardEdge).toBeNull();
    expect(window.first).toEqual({ year: 2025, month0: 9 });
    expect(window.count).toBe(15);
  });

  it('начало аренды (пол = сегодня, будущее открыто): верх жёсткий, низ бесконечен', () => {
    const window = calendarFeedWindow({
      anchorMonth: october2026,
      todayMonth: october2026,
      pastFloorMonth: october2026,
      maxDateMonth: undefined,
    });
    expect(window.startEdge).toEqual({ year: 2026, month0: 7 });
    expect(window.forwardEdge).toBeNull();
  });

  it('якорь-значение в прошлом при правке (#502): запас уходит от якоря через границу года', () => {
    const window = calendarFeedWindow({
      anchorMonth: { year: 2020, month0: 2 },
      todayMonth: october2026,
      maxDateMonth: undefined,
    });
    expect(window.first).toEqual({ year: 2019, month0: 2 });
    expect(window.count).toBe(94);
  });
});
