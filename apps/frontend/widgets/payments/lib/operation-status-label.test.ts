import { describe, expect, it } from 'vitest';
import { operationStatusLabel } from './operation-status-label';
import type { PaymentOperation } from '@/entities/payment';

function operation(overrides: Partial<PaymentOperation>): PaymentOperation {
  return {
    id: 'op1',
    propertyId: 'prop1',
    paymentId: 'p1',
    date: '2026-08-20',
    paidDate: '2026-08-20',
    status: 'paid',
    type: 'expense',
    title: 'Интернет',
    amountKopecks: 100000,
    paymentForm: 'transfer',
    categoryLabel: 'Интернет',
    categorySlug: 'internet',
    ...overrides,
  };
}

describe('operationStatusLabel', () => {
  it('оплачена раньше срока — «Заранее на N дней»', () => {
    const label = operationStatusLabel(operation({ paidDate: '2026-08-15' }));
    expect(label).toStrictEqual({ kind: 'early', text: 'Заранее на 5 дней' });
  });

  it('оплачена позже срока — «Задержан на N дней»', () => {
    const label = operationStatusLabel(operation({ paidDate: '2026-08-28' }));
    expect(label).toStrictEqual({ kind: 'late', text: 'Задержан на 8 дней' });
  });

  it('оплачена ровно в срок — дата оплаты', () => {
    const label = operationStatusLabel(operation({}));
    expect(label).toStrictEqual({ kind: 'ontime', text: '20 августа' });
  });

  it('просрочка и плановые — подписи lib не касаются', () => {
    expect(
      operationStatusLabel(operation({ status: 'overdue', paidDate: undefined })),
    ).toBeNull();
    expect(operationStatusLabel(operation({ status: 'planned', paidDate: undefined }))).toBeNull();
  });

  it('смена месяца и года считается по календарю', () => {
    const label = operationStatusLabel(operation({ date: '2026-09-02', paidDate: '2026-08-28' }));
    expect(label).toStrictEqual({ kind: 'early', text: 'Заранее на 5 дней' });
  });
});
