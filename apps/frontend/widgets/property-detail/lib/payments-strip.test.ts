import { describe, expect, it } from 'vitest';

import type { Payment } from '@/entities/payment';

import { propertyPaymentGroups } from './payments-strip';

function paymentFixture(overrides: Partial<Payment> = {}): Payment {
  return {
    id: 'payment-1',
    propertyId: 'property-1',
    type: 'expense',
    title: 'Интернет',
    amountKopecks: 50000,
    recurrence: { kind: 'monthly', daysOfMonth: [10], lastDay: false },
    since: '2026-01-10',
    autoPay: false,
    paymentForm: 'transfer',
    category: { source: 'default', slug: 'internet', label: 'Интернет' },
    isFavorite: false,
    isCompleted: false,
    pauses: [],
    createdAt: '2026-01-10T00:00:00Z',
    updatedAt: '2026-01-10T00:00:00Z',
    ...overrides,
  };
}

describe('propertyPaymentGroups', () => {
  it('делит на автоплатежи и обычные платежи (макет 1185:40820: автоплатежи первыми)', () => {
    const payments = [
      paymentFixture({ id: 'p1', autoPay: false }),
      paymentFixture({ id: 'p2', autoPay: true }),
      paymentFixture({ id: 'p3', autoPay: false }),
    ];
    const groups = propertyPaymentGroups(payments, new Set(), '2026-09-11');
    expect(groups.map((group) => group.label)).toEqual(['Автоплатежи', 'Платежи']);
    expect(groups[1]?.items.map((item) => item.payment.id)).toEqual(['p1', 'p3']);
    expect(groups[0]?.items.map((item) => item.payment.id)).toEqual(['p2']);
  });

  it('точка просрочки — по идентификаторам платёж с накопленной просрочкой', () => {
    const payments = [
      paymentFixture({ id: 'p1', autoPay: false }),
      paymentFixture({ id: 'p2', autoPay: false }),
    ];
    const groups = propertyPaymentGroups(payments, new Set(['p2']), '2026-09-11');
    expect(groups[0]?.items.map((item) => item.overdue)).toEqual([false, true]);
  });

  it('паузные правила не выводятся — их место на странице платежа', () => {
    const payments = [
      paymentFixture({
        id: 'p1',
        pauses: [{ from: '2026-09-01' }],
      }),
      paymentFixture({ id: 'p2' }),
    ];
    const groups = propertyPaymentGroups(payments, new Set(), '2026-09-11');
    expect(groups).toHaveLength(1);
    expect(groups[0]?.items.map((item) => item.payment.id)).toEqual(['p2']);
  });

  it('пустая группа не выводится: только автоплатежи — одна группа', () => {
    const payments = [paymentFixture({ id: 'p1', autoPay: true })];
    const groups = propertyPaymentGroups(payments, new Set(), '2026-09-11');
    expect(groups).toHaveLength(1);
    expect(groups[0]?.label).toBe('Автоплатежи');
  });
});
