import { describe, expect, it } from 'vitest';
import type { PaymentOperation, PaymentSchedule } from '@/entities/payment';
import { buildScheduleList } from './schedule-list';

/** Ежемесячное правило 1 числа с 2026-09-01, без окончания и пауз. */
const monthly: PaymentSchedule = {
  recurrence: { kind: 'monthly', dayOfMonth: 1 },
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

describe('buildScheduleList', () => {
  it('начинает с сегодняшнего вхождения, когда сервер ещё ничего не материализовал', () => {
    const list = buildScheduleList(monthly, [], '2026-09-10');
    expect(list[0]).toStrictEqual({ kind: 'projected', date: '2026-10-01' });
  });

  it('ставит материализованные операции первыми — ближайшее совпадает с operations', () => {
    const list = buildScheduleList(
      monthly,
      [op('op1', '2026-09-01'), op('op2', '2026-10-01')],
      '2026-09-10',
    );
    expect(list[0]).toStrictEqual({ kind: 'operation', operation: op('op1', '2026-09-01') });
    expect(list[1]).toStrictEqual({ kind: 'operation', operation: op('op2', '2026-10-01') });
    // Проекция продолжается строго после последней материализованной даты.
    expect(list[2]).toStrictEqual({ kind: 'projected', date: '2026-11-01' });
    expect(list[3]).toStrictEqual({ kind: 'projected', date: '2026-12-01' });
  });

  it('не дублирует дату материализованного вхождения в проекции', () => {
    const list = buildScheduleList(monthly, [op('op1', '2026-10-01')], '2026-09-10');
    const projectedDates = list
      .filter((entry): entry is { kind: 'projected'; date: string } => entry.kind === 'projected')
      .map((entry) => entry.date);
    expect(projectedDates).not.toContain('2026-10-01');
    expect(projectedDates[0]).toBe('2026-11-01');
  });

  it('останавливается на endDate — день окончания включён', () => {
    const bounded: PaymentSchedule = {
      ...monthly,
      endDate: '2026-11-30',
      recurrence: { kind: 'daily' },
    };
    const list = buildScheduleList(bounded, [op('op1', '2026-11-30')], '2026-11-30');
    expect(list).toStrictEqual([{ kind: 'operation', operation: op('op1', '2026-11-30') }]);
  });

  it('проекция ежедневного правила не пересекает активную бессрочную паузу', () => {
    const paused: PaymentSchedule = {
      recurrence: { kind: 'daily' },
      since: '2026-01-01',
      endDate: '2026-12-31',
      pauses: [{ from: '2026-09-10' }],
    };
    const list = buildScheduleList(paused, [], '2026-09-10');
    expect(list).toStrictEqual([]);
  });

  it('вырезает интервал паузы [from, to) из проекции — дыра остаётся', () => {
    const withPause: PaymentSchedule = {
      recurrence: { kind: 'monthly', dayOfMonth: 1 },
      since: '2026-01-01',
      pauses: [{ from: '2026-10-01', to: '2026-12-01' }],
    };
    // Проекция идёт от «сегодня»: октябрь и ноябрь внутри паузы [from, to),
    // декабрь (день возобновления to) уже не в паузе; горизонт бессрочного
    // правила — 5 лет, поэтому проверяем дыру, а не конечность списка.
    const list = buildScheduleList(withPause, [], '2026-09-10');
    const dates = list.map((entry) => (entry.kind === 'projected' ? entry.date : ''));
    expect(dates[0]).toBe('2026-12-01');
    expect(dates).not.toContain('2026-10-01');
    expect(dates).not.toContain('2026-11-01');
    expect(dates).toContain('2027-01-01');
  });
});
