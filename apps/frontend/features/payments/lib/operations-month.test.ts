import { describe, expect, it } from 'vitest';

import {
  operationsMonthOf,
  operationsMonthRange,
  shiftOperationsMonth,
} from './operations-month';

describe('operationsMonthOf', () => {
  it('разбирает ISO-дату в месяц навигации', () => {
    expect(operationsMonthOf('2026-09-01')).toEqual({ year: 2026, month: 8 });
    expect(operationsMonthOf('2026-12-31')).toEqual({ year: 2026, month: 11 });
  });
});

describe('shiftOperationsMonth', () => {
  it('сдвигает внутри года', () => {
    expect(shiftOperationsMonth({ year: 2026, month: 8 }, -1)).toEqual({ year: 2026, month: 7 });
    expect(shiftOperationsMonth({ year: 2026, month: 8 }, 1)).toEqual({ year: 2026, month: 9 });
  });

  it('переходит через границу года в обе стороны', () => {
    expect(shiftOperationsMonth({ year: 2026, month: 0 }, -1)).toEqual({ year: 2025, month: 11 });
    expect(shiftOperationsMonth({ year: 2025, month: 11 }, 1)).toEqual({ year: 2026, month: 0 });
  });
});

describe('operationsMonthRange', () => {
  it('даёт границы месяца и подпись чипа «Месяц год»', () => {
    expect(operationsMonthRange({ year: 2026, month: 8 })).toEqual({
      from: '2026-09-01',
      to: '2026-09-30',
      label: 'Сентябрь 2026',
    });
  });

  it('февраль: високосный год — 29 дней, обычный — 28', () => {
    expect(operationsMonthRange({ year: 2024, month: 1 }).to).toBe('2024-02-29');
    expect(operationsMonthRange({ year: 2025, month: 1 }).to).toBe('2025-02-28');
  });

  it('декабрь заканчивается 31-м, не выходя за год', () => {
    expect(operationsMonthRange({ year: 2025, month: 11 })).toEqual({
      from: '2025-12-01',
      to: '2025-12-31',
      label: 'Декабрь 2025',
    });
  });
});

