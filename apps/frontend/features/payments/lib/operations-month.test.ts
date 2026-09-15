import { describe, expect, it } from 'vitest';

import {
  operationsMonthOf,
  operationsMonthRange,
} from './operations-month';

describe('operationsMonthOf', () => {
  it('разбирает ISO-дату в календарный месяц', () => {
    expect(operationsMonthOf('2026-09-01')).toEqual({ year: 2026, month: 8 });
    expect(operationsMonthOf('2026-12-31')).toEqual({ year: 2026, month: 11 });
  });
});

describe('operationsMonthRange', () => {
  it('даёт включительные границы месяца', () => {
    expect(operationsMonthRange({ year: 2026, month: 8 })).toEqual({
      from: '2026-09-01',
      to: '2026-09-30',
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
    });
  });
});

