import { describe, expect, it } from 'vitest';
import type { GlobalPayment, GlobalPaymentObject } from '@/entities/payment';
import {
  globalFavoritePayments,
  globalOverduePayments,
  globalPaymentObjectHasOverdue,
  nearestDateLine,
  overdueDaysLine,
  overdueOperationsCountLabel,
  paymentsCountLabel,
} from './payments-global-model';

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
    ...overrides,
  };
}

describe('globalFavoritePayments', () => {
  it('оставляет только избранные', () => {
    const favorite = item({ id: 'f', isFavorite: true });
    const plain = item({ id: 'p' });

    expect(globalFavoritePayments([favorite, plain])).toStrictEqual([favorite]);
  });

  it('сортирует по favoriteOrder, правила без позиции — в конец (№576)', () => {
    const unordered = item({ id: 'new', isFavorite: true, favoriteOrder: null });
    const second = item({ id: 'second', isFavorite: true, favoriteOrder: 2 });
    const first = item({ id: 'first', isFavorite: true, favoriteOrder: 1 });

    const result = globalFavoritePayments([unordered, second, first]);

    expect(result.map((row) => row.id)).toEqual(['first', 'second', 'new']);
  });
});

describe('globalOverduePayments', () => {
  it('оставляет правила с накопленной просрочкой', () => {
    const overdue = item({ id: 'overdue', overdueOperationCount: 2, overdueDays: 3 });
    const fresh = item({ id: 'fresh' });

    expect(globalOverduePayments([overdue, fresh])).toStrictEqual([overdue]);
  });
});

describe('counters labels', () => {
  it.each([
    [1, '1 платёж'],
    [2, '2 платежа'],
    [17, '17 платежей'],
    [21, '21 платёж'],
  ])('paymentsCountLabel(%i) — «%s»', (count, label) => {
    expect(paymentsCountLabel(count)).toBe(label);
  });

  it.each([
    [1, '1 просроченный'],
    [3, '3 просроченных'],
    [7, '7 просроченных'],
    [21, '21 просроченный'],
  ])('overdueOperationsCountLabel(%i) — «%s»', (count, label) => {
    expect(overdueOperationsCountLabel(count)).toBe(label);
  });
});

describe('nearestDateLine — дата «Ближайший» без статуса', () => {
  it('рисует дату графика', () => {
    expect(nearestDateLine(item({ nearestDate: '2026-08-17' }))).toBe('17 августа');
  });

  it('нет следующего вхождения (пауза/завершён) — строки нет', () => {
    expect(nearestDateLine(item({ nearestDate: null }))).toBeUndefined();
  });
});

describe('overdueDaysLine', () => {
  it('возраст старейшей просрочки — «N дней»', () => {
    expect(overdueDaysLine(item({ overdueDays: 2 }))).toBe('2 дня');
  });

  it('без просрочки строки нет', () => {
    expect(overdueDaysLine(item({ overdueDays: null }))).toBeUndefined();
  });
});

describe('globalPaymentObjectHasOverdue', () => {
  const object = (overrides: Partial<GlobalPaymentObject>): GlobalPaymentObject => ({
    propertyId: 'property-1',
    name: 'Моя квартира',
    address: 'Новатарова, 8',
    pinnedAt: null,
    autoPayRules: [],
    otherRules: [],
    ...overrides,
  });

  it('просрочка в любой стопке красит точкой', () => {
    expect(
      globalPaymentObjectHasOverdue(
        object({ autoPayRules: [{ paymentId: 'p', hasOverdue: false }], otherRules: [{ paymentId: 'q', hasOverdue: true }] }),
      ),
    ).toBe(true);
  });

  it('без просрочек точки нет', () => {
    expect(
      globalPaymentObjectHasOverdue(
        object({ autoPayRules: [{ paymentId: 'p', hasOverdue: false }] }),
      ),
    ).toBe(false);
  });
});
