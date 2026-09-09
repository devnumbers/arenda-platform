import { describe, expect, it } from 'vitest';
import type { GlobalPayment } from '@/entities/payment';
import { globalOverdueList, parseOverdueSortParams } from './overdue-global-model';

function item(overrides: Partial<GlobalPayment>): GlobalPayment {
  return {
    id: 'payment-1',
    propertyId: 'property-1',
    propertyName: 'Моя квартира',
    title: 'Страхование',
    amountKopecks: 3_200_000,
    type: 'expense',
    category: { source: 'default', slug: 'insurance', label: 'Страхование' },
    autoPay: false,
    isFavorite: false,
    favoriteOrder: null,
    today: '2026-09-08',
    nearestDate: '2026-09-10',
    overdueOperationCount: 0,
    overdueDays: null,
    oldestOverdueOperationId: null,
    ...overrides,
  };
}

describe('parseOverdueSortParams', () => {
  it('отсутствующее, пустое и неизвестное значение — дефолт «Старые» (решение владельца 09.09)', () => {
    expect(parseOverdueSortParams(undefined)).toBe('old');
    expect(parseOverdueSortParams('')).toBe('old');
    expect(parseOverdueSortParams('garbage')).toBe('old');
  });

  it('читает ?sort=new|old строки запроса', () => {
    expect(parseOverdueSortParams('new')).toBe('new');
    expect(parseOverdueSortParams('old')).toBe('old');
  });
});

describe('globalOverdueList', () => {
  it('оставляет только правила с накопленной просрочкой', () => {
    const overdue = item({ id: 'overdue', overdueOperationCount: 2, overdueDays: 3 });
    const fresh = item({ id: 'fresh' });

    expect(globalOverdueList([overdue, fresh], 'new')).toStrictEqual([overdue]);
  });

  it('«Новые» — по возрастанию возраста просрочки: недавно просроченные первыми', () => {
    const freshOverdue = item({ id: 'fresh-overdue', overdueOperationCount: 1, overdueDays: 2 });
    const oldest = item({ id: 'oldest', overdueOperationCount: 5, overdueDays: 60 });
    const middle = item({ id: 'middle', overdueOperationCount: 3, overdueDays: 5 });

    const result = globalOverdueList([oldest, middle, freshOverdue], 'new');

    expect(result.map((row) => row.id)).toStrictEqual(['fresh-overdue', 'middle', 'oldest']);
  });

  it('«Старые» — по убыванию возраста: старейшие первыми (как секция главного экрана)', () => {
    const freshOverdue = item({ id: 'fresh-overdue', overdueOperationCount: 1, overdueDays: 2 });
    const oldest = item({ id: 'oldest', overdueOperationCount: 5, overdueDays: 60 });
    const middle = item({ id: 'middle', overdueOperationCount: 3, overdueDays: 5 });

    const result = globalOverdueList([freshOverdue, oldest, middle], 'old');

    expect(result.map((row) => row.id)).toStrictEqual(['oldest', 'middle', 'fresh-overdue']);
  });

  it('равный возраст сохраняет порядок фида (сортировка стабильна)', () => {
    const a = item({ id: 'a', overdueOperationCount: 1, overdueDays: 5 });
    const b = item({ id: 'b', overdueOperationCount: 4, overdueDays: 5 });
    const c = item({ id: 'c', overdueOperationCount: 2, overdueDays: 5 });

    const result = globalOverdueList([a, b, c], 'new');

    expect(result.map((row) => row.id)).toStrictEqual(['a', 'b', 'c']);
  });

  it('аномальный null возраста уходит в конец в обоих направлениях', () => {
    const fresh = item({ id: 'fresh', overdueOperationCount: 1, overdueDays: 2 });
    const anomalous = item({ id: 'anomalous', overdueOperationCount: 3, overdueDays: null });

    expect(globalOverdueList([anomalous, fresh], 'new').map((row) => row.id)).toStrictEqual([
      'fresh',
      'anomalous',
    ]);
    expect(globalOverdueList([anomalous, fresh], 'old').map((row) => row.id)).toStrictEqual([
      'fresh',
      'anomalous',
    ]);
  });
});
