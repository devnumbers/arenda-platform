import { describe, expect, it } from 'vitest';

import { hasNoPaidOperationsEver } from './operations-empty-states';

const emptySummary = {
  incomeTotalKopecks: 0,
  expenseTotalKopecks: 0,
  categories: [],
};

const summaryWithExpenses = {
  incomeTotalKopecks: 0,
  expenseTotalKopecks: 200000,
  categories: [
    { slug: 'internet', label: 'Интернет', type: 'expense' as const, totalKopecks: 200000 },
  ],
};

describe('hasNoPaidOperationsEver', () => {
  it('пустая all-time сводка — оплаченных операций не было никогда', () => {
    expect(hasNoPaidOperationsEver(emptySummary)).toBe(true);
  });

  it('недогруженная сводка — «еще не было» не утверждаем (показ скелета)', () => {
    expect(hasNoPaidOperationsEver(undefined)).toBe(false);
  });

  it('есть операции за любой период — объект не пуст', () => {
    expect(hasNoPaidOperationsEver(summaryWithExpenses)).toBe(false);
  });
});
