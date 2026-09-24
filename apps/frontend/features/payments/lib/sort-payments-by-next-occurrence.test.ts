import { describe, expect, it } from 'vitest';
import { sortPaymentsByNextOccurrence } from './sort-payments-by-next-occurrence';
import type { Payment } from '@/entities/payment';

function payment(overrides: Partial<Payment>): Payment {
  return {
    isCompleted: false,
    isRentalManaged: false,
    id: 'p1',
    propertyId: 'prop1',
    type: 'expense',
    title: 'Арендная плата',
    amountKopecks: 4500000,
    recurrence: { kind: 'monthly', daysOfMonth: [1], lastDay: false },
    since: '2026-01-01',
    autoPay: false,
    paymentForm: 'transfer',
    category: { source: 'default', slug: 'rent', label: 'Арендная плата' },
    isFavorite: false,
    pauses: [],
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

describe('sortPaymentsByNextOccurrence', () => {
  it('самое раннее вхождение — первым', () => {
    const monthly5 = payment({ id: 'monthly-5', recurrence: { kind: 'monthly', daysOfMonth: [5], lastDay: false } });
    const monthly1 = payment({ id: 'monthly-1' });
    const weekly = payment({ id: 'weekly', recurrence: { kind: 'weekly', weekdays: [6] } });

    const sorted = sortPaymentsByNextOccurrence([monthly5, weekly, monthly1], '2026-08-27');

    // 27.08.2026 — четверг: суббота (29.08), 1-е (01.09), 5-е (05.09).
    expect(sorted.map((p) => p.id)).toStrictEqual(['weekly', 'monthly-1', 'monthly-5']);
  });

  it('завершённые и паузные уходят в конец без дат', () => {
    const finished = payment({ id: 'finished', endDate: '2026-08-01' });
    const paused = payment({ id: 'paused', pauses: [{ from: '2026-08-01' }] });
    const active = payment({ id: 'active' });

    const sorted = sortPaymentsByNextOccurrence([finished, paused, active], '2026-08-27');

    expect(sorted.map((p) => p.id)).toStrictEqual(['active', 'finished', 'paused']);
  });

  it('при равной дате сохраняется серверный порядок (устойчивость)', () => {
    const first = payment({ id: 'first', recurrence: { kind: 'monthly', daysOfMonth: [1], lastDay: false } });
    const second = payment({ id: 'second', recurrence: { kind: 'monthly', daysOfMonth: [1], lastDay: false } });

    const sorted = sortPaymentsByNextOccurrence([first, second], '2026-08-27');

    expect(sorted.map((p) => p.id)).toStrictEqual(['first', 'second']);
  });
});
