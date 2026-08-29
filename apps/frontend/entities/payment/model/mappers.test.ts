import { describe, expect, it } from 'vitest';
import type { components } from '@/shared/api/dto';
import { mapPayment, mapPaymentOperation } from './mappers';

type PaymentDto = components['schemas']['PaymentResponse'];
type OperationDto = components['schemas']['OperationResponse'];

const paymentDto: PaymentDto = {
  id: '0198b6a7-1000-7000-8000-000000000001',
  propertyId: '0198b6a7-1000-7000-8000-00000000prop',
  type: 'expense',
  title: 'Арендная плата',
  amountKopecks: 5_600_000,
  recurrence: { kind: 'monthly', daysOfMonth: [], lastDay: true },
  since: '2026-01-31',
  endDate: null,
  autoPay: false,
  paymentForm: 'transfer',
  category: { source: 'default', slug: 'rent', label: 'Арендная плата' },
  isFavorite: false,
  pauses: [
    { fromDate: '2026-03-01', toDate: '2026-04-01' },
    { fromDate: '2026-05-01', toDate: null },
  ],
  createdAt: '2026-01-31T09:00:00Z',
  updatedAt: '2026-05-01T10:30:00Z',
};

describe('mapPayment — DTO → entity', () => {
  const payment = mapPayment(paymentDto);

  it('скалярные поля переносятся один в один (контракт camelCase)', () => {
    expect(payment.id).toBe(paymentDto.id);
    expect(payment.title).toBe('Арендная плата');
    expect(payment.amountKopecks).toBe(5_600_000);
    expect(payment.recurrence).toStrictEqual({ kind: 'monthly', daysOfMonth: [], lastDay: true });
    expect(payment.since).toBe('2026-01-31');
    expect(payment.isFavorite).toBe(false);
    expect(payment.category).toStrictEqual({
      source: 'default',
      slug: 'rent',
      id: undefined,
      label: 'Арендная плата',
    });
  });

  it('nullables нормализуются к опциональности', () => {
    expect(payment.endDate).toBeUndefined();
  });

  it('интервалы пауз переименовываются в [from, to), открытая бессрочная без to', () => {
    expect(payment.pauses).toStrictEqual([
      { from: '2026-03-01', to: '2026-04-01' },
      { from: '2026-05-01', to: undefined },
    ]);
  });

  it('endDate датой приходит как есть, категория custom несёт id вместо слага', () => {
    const ended: PaymentDto = {
      ...paymentDto,
      endDate: '2026-12-31',
      category: { source: 'custom', id: '0198b6a7-user', label: 'Своя' },
    };
    const mapped = mapPayment(ended);
    expect(mapped.endDate).toBe('2026-12-31');
    expect(mapped.category.slug).toBeUndefined();
    expect(mapped.category.id).toBe('0198b6a7-user');
  });
});

const operationDto: OperationDto = {
  id: '0198b6a7-op00-7000-8000-000000000001',
  propertyId: '0198b6a7-1000-7000-8000-00000000prop',
  paymentId: null,
  date: '2026-08-27',
  paidDate: null,
  status: 'overdue',
  type: 'expense',
  title: 'ЖКУ',
  amountKopecks: 320_000,
  paymentForm: null,
  categoryLabel: 'Коммунальные услуги',
  categorySlug: null,
};

describe('mapPaymentOperation — DTO → entity', () => {
  const operation = mapPaymentOperation(operationDto);

  it('просрочка и ручные поля нормализуются: nulls → опциональность', () => {
    expect(operation.status).toBe('overdue');
    expect(operation.paymentId).toBeNull();
    expect(operation.paidDate).toBeUndefined();
    expect(operation.paymentForm).toBeUndefined();
    expect(operation.categorySlug).toBeUndefined();
    expect(operation.amountKopecks).toBe(320_000);
  });

  it('оплаченное вхождение переносит фактическую дату и снапшот категории', () => {
    const paid: OperationDto = {
      ...operationDto,
      status: 'paid',
      paidDate: '2026-08-25',
      paymentId: '0198b6a7-rule',
      paymentForm: 'cash',
      categorySlug: 'utilities',
    };
    const mapped = mapPaymentOperation(paid);
    expect(mapped.paidDate).toBe('2026-08-25');
    expect(mapped.paymentForm).toBe('cash');
    expect(mapped.categorySlug).toBe('utilities');
  });
});
