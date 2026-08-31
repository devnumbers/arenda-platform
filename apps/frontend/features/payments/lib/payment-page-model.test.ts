import { describe, expect, it } from 'vitest';
import type { Payment, PaymentOperation } from '@/entities/payment';
import {
  isPaymentCompleted,
  nearestOccurrence,
  oldestUnpaidOperation,
  paymentTypeLabel,
} from './payment-page-model';

function operation(overrides: Partial<PaymentOperation>): PaymentOperation {
  return {
    id: 'op-1',
    propertyId: 'p-1',
    paymentId: 'pay-1',
    date: '2026-08-01',
    status: 'planned',
    type: 'expense',
    title: 'Арендная плата',
    amountKopecks: 4500000,
    categoryLabel: 'Аренда',
    ...overrides,
  };
}

function payment(overrides: Partial<Payment>): Payment {
  return {
    isCompleted: false,
    id: 'pay-1',
    propertyId: 'p-1',
    type: 'expense',
    title: 'Арендная плата',
    amountKopecks: 4500000,
    recurrence: { kind: 'monthly', daysOfMonth: [1], lastDay: false },
    since: '2026-01-01',
    autoPay: false,
    paymentForm: 'transfer',
    category: { source: 'default', slug: 'rent', label: 'Аренда' },
    isFavorite: false,
    pauses: [],
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

describe('oldestUnpaidOperation', () => {
  it('берёт старейшую просрочку, даже если плановая дата раньше по списку', () => {
    const overdue = [operation({ id: 'overdue', status: 'overdue', date: '2026-08-01' })];
    const planned = [operation({ id: 'planned', status: 'planned', date: '2026-09-01' })];

    expect(oldestUnpaidOperation(overdue, planned)?.id).toBe('overdue');
  });

  it('без просрочек гасит первую плановую (asc)', () => {
    const planned = [
      operation({ id: 'planned-1', date: '2026-09-01' }),
      operation({ id: 'planned-2', date: '2026-10-01' }),
    ];

    expect(oldestUnpaidOperation([], planned)?.id).toBe('planned-1');
  });

  it('возвращает null, когда всё оплачено', () => {
    expect(oldestUnpaidOperation([], [])).toBeNull();
  });
});

describe('isPaymentCompleted', () => {
  it('бессрочный платеж не завершён никогда', () => {
    expect(isPaymentCompleted(payment({ endDate: undefined }), '2026-08-27')).toBe(false);
  });

  it('день окончания включён: завершён только после него', () => {
    const ended = payment({ endDate: '2026-08-31' });

    expect(isPaymentCompleted(ended, '2026-08-31')).toBe(false);
    expect(isPaymentCompleted(ended, '2026-09-01')).toBe(true);
  });
});

describe('paymentTypeLabel', () => {
  it('подписывает направление — «Расход» / «Доход»', () => {
    expect(paymentTypeLabel('expense')).toBe('Расход');
    expect(paymentTypeLabel('income')).toBe('Доход');
  });
});

describe('nearestOccurrence', () => {
  const dailySinceToday: Payment = {
    ...payment({}),
    recurrence: { kind: 'daily' },
    since: '2026-08-28',
  };

  it('материализованное плановое — источник истины, проекция не перекрывает его', () => {
    const planned = [operation({ id: 'op30', date: '2026-08-30' })];
    const nearest = nearestOccurrence(dailySinceToday, planned, '2026-08-28');
    expect(nearest).toStrictEqual({
      kind: 'operation',
      operation: operation({ id: 'op30', date: '2026-08-30' }),
    });
  });

  it('без материализованных — вхождение «сегодня» по проекции', () => {
    const nearest = nearestOccurrence(dailySinceToday, [], '2026-08-28');
    expect(nearest).toStrictEqual({ kind: 'projected', date: '2026-08-28' });
  });

  it('правило стартует в будущем — первое вхождение после сегодня', () => {
    const future = { ...dailySinceToday, since: '2026-09-02' };
    const nearest = nearestOccurrence(future, [], '2026-08-28');
    expect(nearest).toStrictEqual({ kind: 'projected', date: '2026-09-02' });
  });

  it('исчерпанное правило (endDate позади) — undefined', () => {
    const finished = { ...dailySinceToday, endDate: '2026-08-20' };
    expect(nearestOccurrence(finished, [], '2026-08-28')).toBeUndefined();
  });
});
