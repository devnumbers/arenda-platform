import { describe, expect, it } from 'vitest';

import { categoryStyle } from '@/features/payment-categories';

import { summaryBarSegments } from './summary-bar';

const summary = {
  incomeTotalKopecks: 4620000,
  expenseTotalKopecks: 350000,
  categories: [
    { slug: 'rent', label: 'Арендная плата', type: 'income' as const, totalKopecks: 4500000 },
    { slug: 'security', label: 'Охрана', type: 'expense' as const, totalKopecks: 250000 },
    {
      slug: 'utilities-compensation',
      label: 'Компенсация коммунальных услуг',
      type: 'income' as const,
      totalKopecks: 120000,
    },
    { slug: 'internet', label: 'Интернет', type: 'expense' as const, totalKopecks: 100000 },
  ],
};

describe('summaryBarSegments', () => {
  it('отдаёт все категории своего направления в порядке сводки', () => {
    const expenses = summaryBarSegments(summary, 'expense');
    expect(expenses.map((s) => s.weight)).toEqual([250000, 100000]);
    const incomes = summaryBarSegments(summary, 'income');
    expect(incomes.map((s) => s.weight)).toEqual([4500000, 120000]);
  });

  it('каждой категории — цвет каталога, даже если цвет совпадает с соседней', () => {
    const expenses = summaryBarSegments(summary, 'expense');
    expect(expenses[0]?.color).toBe(categoryStyle('default', 'security').color);
    expect(expenses[1]?.color).toBe(categoryStyle('default', 'internet').color);
  });

  it('категории без операций и нулевые не попадают в полосу', () => {
    const withZero = {
      ...summary,
      categories: [
        ...summary.categories,
        { slug: 'repairs', label: 'Ремонт', type: 'expense' as const, totalKopecks: 0 },
      ],
    };
    expect(summaryBarSegments(withZero, 'expense').map((s) => s.weight)).toEqual([
      250000, 100000,
    ]);
  });

  it('без сводки или при нулевом итоге полоса пуста — контейнер рисует серую пилюлю', () => {
    expect(summaryBarSegments(undefined, 'expense')).toEqual([]);
    expect(summaryBarSegments({ ...summary, expenseTotalKopecks: 0, categories: [] }, 'expense')).toEqual([]);
  });
});
