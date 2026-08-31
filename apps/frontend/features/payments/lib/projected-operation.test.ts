import { describe, expect, it } from 'vitest';
import type { Payment, PaymentOperation } from '@/entities/payment';
import { projectedOperation } from './projected-operation';

/**
 * Проекционная «операция» для просмотра (решение владельца: чисто фронт):
 * строится из правила на дату вхождения; вне расписания, в паузе или в
 * прошлом проекции нет — страница показывает «вхождение не найдено».
 */

function payment(overrides: Partial<Payment> = {}): Payment {
  return {
    id: 'pay-1',
    propertyId: 'prop-1',
    type: 'expense',
    title: 'Страхование',
    amountKopecks: 320_000,
    recurrence: { kind: 'monthly', daysOfMonth: [15], lastDay: false },
    since: '2026-01-15',
    autoPay: false,
    paymentForm: 'transfer',
    category: { source: 'default', slug: 'insurance', label: 'Страхование' },
    isFavorite: false,
    isCompleted: false,
    pauses: [],
    createdAt: '2026-01-15T00:00:00Z',
    updatedAt: '2026-01-15T00:00:00Z',
    ...overrides,
  };
}

describe('projectedOperation', () => {
  it('будущее вхождение по расписанию — проекция из полей правила', () => {
    const op: PaymentOperation | undefined = projectedOperation(
      payment(),
      '2026-10-15',
      '2026-08-31',
    );
    expect(op).toBeDefined();
    expect(op).toMatchObject({
      id: 'projected:2026-10-15',
      propertyId: 'prop-1',
      paymentId: 'pay-1',
      date: '2026-10-15',
      status: 'planned',
      type: 'expense',
      title: 'Страхование',
      amountKopecks: 320_000,
      categoryLabel: 'Страхование',
      categorySlug: 'insurance',
    });
  });

  it('вне расписания проекции нет', () => {
    expect(projectedOperation(payment(), '2026-10-14', '2026-08-31')).toBeUndefined();
  });

  it('прошлая дата — не проекция', () => {
    expect(projectedOperation(payment(), '2026-07-15', '2026-08-31')).toBeUndefined();
  });

  it('день в паузе — проекции нет', () => {
    const paused = payment({
      pauses: [{ from: '2026-10-01', to: '2026-11-01' }],
    });
    expect(projectedOperation(paused, '2026-10-15', '2026-08-31')).toBeUndefined();
  });
});
