import { describe, expect, it } from 'vitest';

import { categoryStyle } from '@/features/payment-categories';

import { summaryBarSegments } from './summary-bar';

const summary = {
  incomeTotalKopecks: 4620000,
  expenseTotalKopecks: 480000,
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
    { slug: 'utilities', label: 'Коммунальные услуги', type: 'expense' as const, totalKopecks: 90000 },
    { slug: 'electricity', label: 'Электроэнергия', type: 'expense' as const, totalKopecks: 40000 },
  ],
};

describe('summaryBarSegments', () => {
  it('категории одного цвета каталога сливаются в одну пилюлю с суммарным весом', () => {
    const expenses = summaryBarSegments(summary, 'expense');
    const teal = expenses.find((s) => s.color === categoryStyle('default', 'utilities').color);
    expect(teal).toBeDefined();
    expect(teal?.weight).toBe(130000);
    expect(expenses.filter((s) => s.color === teal?.color)).toHaveLength(1);
  });

  it('пилюли одного направления — по одной на цвет, сортировка по сумме убывание', () => {
    const expenses = summaryBarSegments(summary, 'expense');
    expect(expenses.map((s) => s.color)).toEqual([
      categoryStyle('default', 'security').color,
      categoryStyle('default', 'utilities').color,
      categoryStyle('default', 'internet').color,
    ]);
    expect(expenses.map((s) => s.weight)).toEqual([250000, 130000, 100000]);
  });

  it('направления не смешиваются: доходы сливаются только внутри доходов', () => {
    const incomes = summaryBarSegments(summary, 'income');
    expect(incomes.map((s) => s.color)).toEqual([
      categoryStyle('default', 'rent').color,
      categoryStyle('default', 'utilities-compensation').color,
    ]);
    expect(incomes.map((s) => s.weight)).toEqual([4500000, 120000]);
  });

  it('категории без операций и нулевые не попадают в полосу', () => {
    const withZero = {
      ...summary,
      categories: [
        ...summary.categories,
        { slug: 'repairs', label: 'Ремонт', type: 'expense' as const, totalKopecks: 0 },
      ],
    };
    const expenses = summaryBarSegments(withZero, 'expense');
    expect(expenses.map((s) => s.weight)).toEqual([250000, 130000, 100000]);
    expect(expenses.map((s) => s.color)).not.toContain(categoryStyle('default', 'repairs').color);
  });

  it('без сводки или без категорий с операциями полоса пуста — контейнер рисует серую пилюлю', () => {
    expect(summaryBarSegments(undefined, 'expense')).toEqual([]);
    expect(summaryBarSegments({ ...summary, expenseTotalKopecks: 0, categories: [] }, 'expense')).toEqual([]);
  });
});
