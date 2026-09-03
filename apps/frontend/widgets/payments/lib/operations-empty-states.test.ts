import { describe, expect, it } from 'vitest';

import { formatMoneyKopecks } from '@/shared/lib/format-money';

import { hasNoPaidOperationsEver, operationsTypeHeadline } from './operations-empty-states';

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

describe('operationsTypeHeadline', () => {
  it('пока не загружено — прочерк', () => {
    expect(operationsTypeHeadline('income', undefined)).toBe('—');
  });

  it('пустой период — «Нет доходов»/«Нет трат» вместо «0 ₽» (1510-76177, 1510-75650)', () => {
    expect(operationsTypeHeadline('income', 0)).toBe('Нет доходов');
    expect(operationsTypeHeadline('expense', 0)).toBe('Нет трат');
  });

  it('непустой период — сумма направления', () => {
    expect(operationsTypeHeadline('income', 5620000)).toBe(formatMoneyKopecks(5620000));
    expect(operationsTypeHeadline('expense', 200000)).toBe(formatMoneyKopecks(200000));
  });
});
