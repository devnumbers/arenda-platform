import { describe, expect, it } from 'vitest';
import { paymentRowSubtitle } from './payment-row-subtitle';
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

describe('paymentRowSubtitle', () => {
  it('у активной бессрочной платежа — дата следующего вхождения', () => {
    const result = paymentRowSubtitle(payment({}), '2026-08-27');
    expect(result).toStrictEqual({ kind: 'date', iso: '2026-09-01' });
  });

  it('у платежа на активной бессрочной паузе — «пауза»', () => {
    const result = paymentRowSubtitle(
      payment({ pauses: [{ from: '2026-08-01' }] }),
      '2026-08-27',
    );
    expect(result).toStrictEqual({ kind: 'paused' });
  });

  it('день возобновления уже не в паузе — снова дата', () => {
    const result = paymentRowSubtitle(
      payment({ pauses: [{ from: '2026-08-01', to: '2026-08-27' }] }),
      '2026-08-27',
    );
    expect(result).toStrictEqual({ kind: 'date', iso: '2026-09-01' });
  });

  it('у завершённого платежа (endDate в прошлом) — нет подзаголовка', () => {
    const result = paymentRowSubtitle(
      payment({ endDate: '2026-08-01' }),
      '2026-08-27',
    );
    expect(result).toStrictEqual({ kind: 'none' });
  });

  it('серверный флаг isCompleted гасит календарную дату — правила уже нет', () => {
    // Сценарий бага короткого правила: endDate ещё впереди (01.09), но все
    // вхождения оплачены — флаг важнее календарной проекции.
    const result = paymentRowSubtitle(
      payment({ endDate: '2026-09-01', isCompleted: true }),
      '2026-08-31',
    );
    expect(result).toStrictEqual({ kind: 'none' });
  });
});
