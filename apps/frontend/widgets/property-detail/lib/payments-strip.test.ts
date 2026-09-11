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
  it('обе группы — максимум 4 платежа в каждой (решение владельца 11.09)', () => {
    const payments = [
      paymentFixture({ id: 'a1', autoPay: true }),
      paymentFixture({ id: 'a2', autoPay: true }),
      paymentFixture({ id: 'a3', autoPay: true }),
      paymentFixture({ id: 'a4', autoPay: true }),
      paymentFixture({ id: 'a5', autoPay: true }),
      paymentFixture({ id: 'p1' }),
      paymentFixture({ id: 'p2' }),
      paymentFixture({ id: 'p3' }),
      paymentFixture({ id: 'p4' }),
      paymentFixture({ id: 'p5' }),
    ];
    const groups = propertyPaymentGroups(payments, new Set(), '2026-09-11');
    expect(groups.map((group) => group.label)).toEqual(['Автоплатежи', 'Платежи']);
    expect(groups[0]?.items).toHaveLength(4);
    expect(groups[1]?.items).toHaveLength(4);
  });

  it('одна группа — максимум 7 платежей', () => {
    const payments = Array.from({ length: 9 }, (_, index) =>
      paymentFixture({ id: `p${index}`, autoPay: false }),
    );
    const groups = propertyPaymentGroups(payments, new Set(), '2026-09-11');
    expect(groups).toHaveLength(1);
    expect(groups[0]?.items).toHaveLength(7);
  });

  it('внутри группы просрочки первыми, затем по ближайшему вхождению', () => {
    const payments = [
      paymentFixture({ id: 'next-month', recurrence: { kind: 'monthly', daysOfMonth: [20], lastDay: false } }),
      paymentFixture({ id: 'plain', recurrence: { kind: 'monthly', daysOfMonth: [15], lastDay: false } }),
      paymentFixture({ id: 'overdue-1', recurrence: { kind: 'monthly', daysOfMonth: [25], lastDay: false } }),
      paymentFixture({ id: 'overdue-2', recurrence: { kind: 'monthly', daysOfMonth: [28], lastDay: false } }),
    ];
    const groups = propertyPaymentGroups(payments, new Set(['overdue-1', 'overdue-2']), '2026-09-11');
    expect(groups[0]?.items.map((item) => item.payment.id)).toEqual([
      'overdue-1',
      'overdue-2',
      'plain',
      'next-month',
    ]);
    expect(groups[0]?.items.map((item) => item.overdue)).toEqual([true, true, false, false]);
  });

  it('точка просрочки — по идентификаторам платёж с накопленной просрочкой', () => {
    const payments = [
      paymentFixture({ id: 'p1', autoPay: false }),
      paymentFixture({ id: 'p2', autoPay: false }),
    ];
    const groups = propertyPaymentGroups(payments, new Set(['p2']), '2026-09-11');
    // p2 с просрочкой выходит вперёд
    expect(groups[0]?.items.map((item) => item.payment.id)).toEqual(['p2', 'p1']);
    expect(groups[0]?.items.map((item) => item.overdue)).toEqual([true, false]);
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
