import { describe, expect, it } from 'vitest';
import type { PaymentSchedule } from '../model/types';
import {
  firstOccurrence,
  isDatePaused,
  nextOccurrenceAfter,
  nextOccurrencesAfter,
  occurrencesBetween,
} from './occurrences';

const schedule = (over: Partial<PaymentSchedule>): PaymentSchedule => ({
  recurrence: { kind: 'daily' },
  since: '2026-01-01',
  pauses: [],
  ...over,
});

describe('occurrencesBetween — ежемесячное прижатие без сползания (смоук прототипа)', () => {
  const jan31: PaymentSchedule = schedule({
    recurrence: { kind: 'monthly', daysOfMonth: [], lastDay: true },
    since: '2026-01-31',
  });

  it('янв31 → фев28 → мар31: каждый месяц строится от якоря независимо', () => {
    expect(occurrencesBetween(jan31, '2026-01-01', '2026-04-01')).toStrictEqual([
      '2026-01-31',
      '2026-02-28',
      '2026-03-31',
    ]);
  });

  it('график не сползает: после прижатия к февралю март снова 31-е', () => {
    expect(occurrencesBetween(jan31, '2026-02-01', '2027-06-30')).toContain(
      '2026-03-31',
    );
    expect(occurrencesBetween(jan31, '2026-04-01', '2026-05-01')).toStrictEqual([
      '2026-04-30',
    ]);
  });
});

describe('occurrencesBetween — ежегодное прижатие 29 февраля', () => {
  const feb29: PaymentSchedule = schedule({
    recurrence: { kind: 'yearly', month: 2, day: 29 },
    since: '2024-02-29',
  });

  it('29.02 живёт как 28.02 в невисокосные годы и не сдвигает якорь', () => {
    expect(occurrencesBetween(feb29, '2024-01-01', '2030-01-01')).toStrictEqual([
      '2024-02-29',
      '2025-02-28',
      '2026-02-28',
      '2027-02-28',
      '2028-02-29',
      '2029-02-28',
    ]);
  });
});

describe('occurrencesBetween — еженедельное расписание (смоук прототипа)', () => {
  it('заведён в среду с выбором субботы: сама среда не вхождение', () => {
    const saturday = schedule({
      recurrence: { kind: 'weekly', weekdays: [6] },
      since: '2026-08-26',
    });
    expect(occurrencesBetween(saturday, '2026-08-20', '2026-09-05')).toStrictEqual([
      '2026-08-29',
      '2026-09-05',
    ]);
  });

  it('несколько дней недели перечисляются по порядку дат', () => {
    const monFri = schedule({
      recurrence: { kind: 'weekly', weekdays: [5, 1] },
      since: '2026-08-24', // понедельник
    });
    expect(occurrencesBetween(monFri, '2026-08-24', '2026-08-28')).toStrictEqual([
      '2026-08-24',
      '2026-08-28',
    ]);
  });

  it('дни недели — 0=воскресенье..6=суббота', () => {
    const sunday = schedule({
      recurrence: { kind: 'weekly', weekdays: [0] },
      since: '2026-08-23', // воскресенье
    });
    expect(occurrencesBetween(sunday, '2026-08-23', '2026-08-23')).toStrictEqual([
      '2026-08-23',
    ]);
  });
});

describe('occurrencesBetween — границы генерации', () => {
  it('задним числом вхождений нет: нижняя граница — дата заведения', () => {
    const daily = schedule({ since: '2026-01-10' });
    expect(occurrencesBetween(daily, '2026-01-01', '2026-01-12')).toStrictEqual([
      '2026-01-10',
      '2026-01-11',
      '2026-01-12',
    ]);
  });

  it('endDate останавливает генерацию, сам день endDate включён', () => {
    const daily = schedule({ since: '2026-01-01', endDate: '2026-01-03' });
    expect(occurrencesBetween(daily, '2026-01-01', '2026-01-10')).toStrictEqual([
      '2026-01-01',
      '2026-01-02',
      '2026-01-03',
    ]);
    expect(nextOccurrenceAfter(daily, '2026-01-03')).toBeNull();
  });

  it('endDate раньше окна — пусто', () => {
    const daily = schedule({ since: '2026-01-01', endDate: '2026-01-03' });
    expect(occurrencesBetween(daily, '2026-01-05', '2026-01-20')).toStrictEqual([]);
  });

  it('ежедневный переносит месяц ровно по одному дню', () => {
    const daily = schedule({ since: '2026-08-30' });
    expect(occurrencesBetween(daily, '2026-08-30', '2026-09-01')).toStrictEqual([
      '2026-08-30',
      '2026-08-31',
      '2026-09-01',
    ]);
  });
});

describe('occurrencesBetween — интервалы пауз [from, to)', () => {
  it('даты внутри паузы вырезаются: ни операций, ни долга, ни догона', () => {
    const daily = schedule({
      since: '2026-01-01',
      pauses: [{ from: '2026-01-05', to: '2026-01-08' }],
    });
    expect(occurrencesBetween(daily, '2026-01-04', '2026-01-10')).toStrictEqual([
      '2026-01-04',
      '2026-01-08',
      '2026-01-09',
      '2026-01-10',
    ]);
  });

  it('границы полуоткрытые: from включительно, to (возобновление) уже не в паузе', () => {
    const daily = schedule({
      since: '2026-01-01',
      pauses: [{ from: '2026-01-05', to: '2026-01-07' }],
    });
    const dates = occurrencesBetween(daily, '2026-01-01', '2026-01-10');
    expect(dates).not.toContain('2026-01-05');
    expect(dates).not.toContain('2026-01-06');
    expect(dates).toContain('2026-01-07');
  });

  it('бессрочная пауза (без to) вырезает всё до горизонта', () => {
    const daily = schedule({
      since: '2026-01-01',
      pauses: [{ from: '2026-01-05' }],
    });
    expect(occurrencesBetween(daily, '2026-01-01', '2026-02-15')).toStrictEqual([
      '2026-01-01',
      '2026-01-02',
      '2026-01-03',
      '2026-01-04',
    ]);
  });

  it('дыры навсегда: вхождение внутри закрытой прошлой паузы не возвращается', () => {
    const monthly = schedule({
      recurrence: { kind: 'monthly', daysOfMonth: [10], lastDay: false },
      since: '2026-01-01',
      endDate: '2026-12-31',
      pauses: [{ from: '2026-03-10', to: '2026-04-10' }],
    });
    expect(occurrencesBetween(monthly, '2026-01-01', '2026-06-30')).toStrictEqual([
      '2026-01-10',
      '2026-02-10',
      '2026-04-10',
      '2026-05-10',
      '2026-06-10',
    ]);
  });
});

describe('nextOccurrenceAfter (смоук прототипа)', () => {
  const jan31: PaymentSchedule = schedule({
    recurrence: { kind: 'monthly', daysOfMonth: [], lastDay: true },
    since: '2026-01-31',
  });

  it('январский платёж: после 1 февраля ближайшее — прижатое 28 февраля', () => {
    expect(nextOccurrenceAfter(jan31, '2026-02-01')).toBe('2026-02-28');
  });

  it('после endDate вхождений больше нет', () => {
    const ended = schedule({
      recurrence: { kind: 'weekly', weekdays: [3] },
      since: '2026-01-01',
      endDate: '2026-01-15',
    });
    expect(nextOccurrenceAfter(ended, '2026-01-16')).toBeNull();
  });

  it('следующий за сегодняшним — через полный цикл (сегодня исключается)', () => {
    const weekly = schedule({
      recurrence: { kind: 'weekly', weekdays: [1] },
      since: '2026-08-17', // понедельник
    });
    expect(nextOccurrenceAfter(weekly, '2026-08-17')).toBe('2026-08-24');
  });
});

describe('isDatePaused', () => {
  it.each([
    ['2026-01-05', true],
    ['2026-01-06', true],
    ['2026-01-07', false],
    ['2026-01-04', false],
    ['2026-02-10', true], // внутри бессрочной
    ['2026-03-01', true],
  ] as const)('%s → %s', (date, expected) => {
    expect(
      isDatePaused(
        [
          { from: '2026-01-05', to: '2026-01-07' },
          { from: '2026-02-01' },
        ],
        date,
      ),
    ).toBe(expected);
  });
});

describe('firstOccurrence — превью визарда (история 9 спеки #453)', () => {
  it('ежедневное правило: первое вхождение — сама дата заведения', () => {
    const daily = schedule({ recurrence: { kind: 'daily' }, since: '2026-08-27' });
    expect(firstOccurrence(daily)).toBe('2026-08-27');
  });

  it('еженедельное по понедельникам: следующий понедельник после четверга', () => {
    const weekly = schedule({
      recurrence: { kind: 'weekly', weekdays: [1] },
      since: '2026-08-27', // четверг
    });
    expect(firstOccurrence(weekly)).toBe('2026-08-31');
  });

  it('ежемесячное 31-е прижимается к последнему дню короткого месяца', () => {
    const monthly = schedule({
      recurrence: { kind: 'monthly', daysOfMonth: [], lastDay: true },
      since: '2026-02-10',
    });
    expect(firstOccurrence(monthly)).toBe('2026-02-28');
  });

  it('ежегодное 29 февраля: ближайшее прижатое после даты заведения', () => {
    const yearly = schedule({
      recurrence: { kind: 'yearly', month: 2, day: 29 },
      since: '2026-05-01',
    });
    expect(firstOccurrence(yearly)).toBe('2027-02-28');
  });

  it('вхождение внутри паузы пропускается', () => {
    const paused = schedule({
      recurrence: { kind: 'daily' },
      since: '2026-08-27',
      pauses: [{ from: '2026-08-27', to: '2026-09-05' }],
    });
    expect(firstOccurrence(paused)).toBe('2026-09-05');
  });

  it('окончание раньше заведения — вхождений нет', () => {
    const ended = schedule({
      recurrence: { kind: 'daily' },
      since: '2026-08-27',
      endDate: '2026-08-25',
    });
    expect(firstOccurrence(ended)).toBeNull();
  });

  it('день окончания включён', () => {
    const ended = schedule({
      recurrence: { kind: 'daily' },
      since: '2026-08-27',
      endDate: '2026-08-29',
    });
    expect(firstOccurrence(ended)).toBe('2026-08-27');
  });
});

describe('monthly с несколькими днями месяца', () => {
  const rec = { kind: 'monthly', daysOfMonth: [18, 1], lastDay: true } as const;
  const schedule: PaymentSchedule = {
    recurrence: rec,
    since: '2026-08-01',
    pauses: [],
  };

  it('вхождение на каждый выбранный день и на последний день месяца, по порядку', () => {
    expect(occurrencesBetween(schedule, '2026-08-01', '2026-09-30')).toStrictEqual([
      '2026-08-01',
      '2026-08-18',
      '2026-08-31',
      '2026-09-01',
      '2026-09-18',
      '2026-09-30',
    ]);
  });

  it('короткий месяц прижимает выбранный день и последний день к 28 февраля', () => {
    const february: PaymentSchedule = {
      recurrence: { kind: 'monthly', daysOfMonth: [30], lastDay: true },
      since: '2026-02-01',
      pauses: [],
    };
    expect(occurrencesBetween(february, '2026-02-01', '2026-02-28')).toStrictEqual([
      '2026-02-28',
    ]);
  });

  it('догрузка идёт по всем дням месяца подряд', () => {
    expect(nextOccurrencesAfter(schedule, '2026-08-10', 3)).toStrictEqual([
      '2026-08-18',
      '2026-08-31',
      '2026-09-01',
    ]);
  });
});
