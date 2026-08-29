import { describe, expect, it } from 'vitest';
import type { PaymentOperation, PaymentSchedule } from '@/entities/payment';
import {
  extendProjection,
  materializedEntries,
  projectionCursor,
} from './schedule-list';

/** Ежемесячное правило 1 числа с 2026-09-01, без окончания и пауз. */
const monthly: PaymentSchedule = {
  recurrence: { kind: 'monthly', daysOfMonth: [1], lastDay: false },
  since: '2026-09-01',
  pauses: [],
};

const op = (id: string, date: string): PaymentOperation => ({
  id,
  propertyId: 'p1',
  paymentId: 'pay1',
  date,
  status: 'planned',
  type: 'expense',
  title: 'Аренда',
  amountKopecks: 4500000,
  categoryLabel: 'Арендная плата',
  categorySlug: 'rent',
});

describe('materializedEntries / projectionCursor', () => {
  it('материализованные операции идут первыми, курсор — строго после последней', () => {
    const entries = materializedEntries([op('op1', '2026-09-01'), op('op2', '2026-10-01')]);
    expect(entries.map((entry) => entry.kind)).toStrictEqual(['operation', 'operation']);
    expect(projectionCursor([op('op1', '2026-09-01'), op('op2', '2026-10-01')], '2026-09-10')).toBe('2026-10-01');
  });

  it('без материализованных курсор — день перед «сегодня» (сегодня включается)', () => {
    expect(projectionCursor([], '2026-09-10')).toBe('2026-09-09');
  });
});

describe('extendProjection', () => {
  it('первая страница начинается с сегодня, когда сервер ничего не материализовал', () => {
    const page = extendProjection(monthly, projectionCursor([], '2026-09-10'), 50);
    expect(page.dates[0]).toBe('2026-10-01');
    expect(page.exhausted).toBe(false);
  });

  it('страницы идут подряд без дублей: курсор = последняя дата страницы', () => {
    const first = extendProjection(monthly, projectionCursor([], '2026-09-10'), 3);
    expect(first.dates).toStrictEqual(['2026-10-01', '2026-11-01', '2026-12-01']);
    const second = extendProjection(monthly, first.nextCursor, 3);
    expect(second.dates).toStrictEqual(['2027-01-01', '2027-02-01', '2027-03-01']);
  });

  it('31-е прижимается без сползания и не теряется между страницами', () => {
    const rule: PaymentSchedule = {
      recurrence: { kind: 'monthly', daysOfMonth: [], lastDay: true },
      since: '2026-01-31',
      pauses: [],
    };
    let cursor = '2026-01-31';
    const seen: string[] = [];
    for (let i = 0; i < 14; i++) {
      const page = extendProjection(rule, cursor, 2);
      seen.push(...page.dates);
      cursor = page.nextCursor;
    }
    expect(seen.slice(0, 6)).toStrictEqual([
      '2026-02-28',
      '2026-03-31',
      '2026-04-30',
      '2026-05-31',
      '2026-06-30',
      '2026-07-31',
    ]);
  });

  it('endDate останавливает генерацию — день окончания включён', () => {
    const bounded: PaymentSchedule = {
      recurrence: { kind: 'daily' },
      since: '2026-11-28',
      endDate: '2026-11-30',
      pauses: [],
    };
    const page = extendProjection(bounded, projectionCursor([], '2026-11-28'), 50);
    expect(page.dates).toStrictEqual(['2026-11-28', '2026-11-29', '2026-11-30']);
    expect(page.exhausted).toBe(true);
  });

  it('паузы вырезаются из страниц, но не двигают курсор назад', () => {
    const paused: PaymentSchedule = {
      recurrence: { kind: 'daily' },
      since: '2026-09-01',
      pauses: [{ from: '2026-09-10', to: '2026-09-13' }],
    };
    const page = extendProjection(paused, '2026-09-08', 3);
    expect(page.dates).toStrictEqual(['2026-09-09', '2026-09-13', '2026-09-14']);
  });
});

describe('extendProjection — остановка на открытой паузе', () => {
  const pausedForever: PaymentSchedule = {
    recurrence: { kind: 'monthly', daysOfMonth: [10], lastDay: false },
    since: '2026-06-29',
    pauses: [{ from: '2026-08-25' }],
  };

  it('активная бессрочная пауза исчерпывает правило — генерация не зависает', () => {
    const page = extendProjection(pausedForever, '2026-08-27', 50);
    expect(page.dates).toStrictEqual([]);
    expect(page.exhausted).toBe(true);
  });

  it('пауза в будущем: собираются только даты до неё', () => {
    const pausesLater: PaymentSchedule = {
      recurrence: { kind: 'monthly', daysOfMonth: [10], lastDay: false },
      since: '2026-09-01',
      pauses: [{ from: '2026-11-01' }],
    };
    const page = extendProjection(pausesLater, '2026-09-15', 50);
    expect(page.dates).toStrictEqual(['2026-10-10']);
    expect(page.exhausted).toBe(true);
  });

  it('ограниченная пауза — дыра, догрузка продолжается за ней', () => {
    const bounded: PaymentSchedule = {
      recurrence: { kind: 'daily' },
      since: '2026-09-01',
      pauses: [{ from: '2026-09-10', to: '2026-09-13' }],
    };
    const page = extendProjection(bounded, '2026-09-08', 4);
    expect(page.dates).toStrictEqual(['2026-09-09', '2026-09-13', '2026-09-14', '2026-09-15']);
    expect(page.exhausted).toBe(false);
  });
});
