import { describe, expect, it } from 'vitest';
import { makePayment } from '@/entities/payment';
import { sortPaymentsByNextOccurrence } from './sort-payments-by-next-occurrence';

describe('sortPaymentsByNextOccurrence', () => {
  it('самое раннее вхождение — первым', () => {
    const monthly5 = makePayment({ id: 'monthly-5', recurrence: { kind: 'monthly', daysOfMonth: [5], lastDay: false } });
    const monthly1 = makePayment({ id: 'monthly-1' });
    const weekly = makePayment({ id: 'weekly', recurrence: { kind: 'weekly', weekdays: [6] } });

    const sorted = sortPaymentsByNextOccurrence([monthly5, weekly, monthly1], '2026-08-27');

    // 27.08.2026 — четверг: суббота (29.08), 1-е (01.09), 5-е (05.09).
    expect(sorted.map((p) => p.id)).toStrictEqual(['weekly', 'monthly-1', 'monthly-5']);
  });

  it('завершённые и паузные уходят в конец без дат', () => {
    const finished = makePayment({ id: 'finished', endDate: '2026-08-01' });
    const paused = makePayment({ id: 'paused', pauses: [{ from: '2026-08-01' }] });
    const active = makePayment({ id: 'active' });

    const sorted = sortPaymentsByNextOccurrence([finished, paused, active], '2026-08-27');

    expect(sorted.map((p) => p.id)).toStrictEqual(['active', 'finished', 'paused']);
  });

  it('при равной дате сохраняется серверный порядок (устойчивость)', () => {
    const first = makePayment({ id: 'first', recurrence: { kind: 'monthly', daysOfMonth: [1], lastDay: false } });
    const second = makePayment({ id: 'second', recurrence: { kind: 'monthly', daysOfMonth: [1], lastDay: false } });

    const sorted = sortPaymentsByNextOccurrence([first, second], '2026-08-27');

    expect(sorted.map((p) => p.id)).toStrictEqual(['first', 'second']);
  });
});
