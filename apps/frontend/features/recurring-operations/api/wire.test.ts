import { describe, expect, it } from 'vitest';
import type {
  RecurringOperationCreateRequest,
  RecurringOperationUpdateRequest,
} from '@/entities/operation';
import { toCreateWireRequest, toUpdateWireRequest } from './hooks';

describe('recurring-operations wire serializers', () => {
  it('maps create command fields to snake_case wire keys', () => {
    const command: RecurringOperationCreateRequest = {
      type: 'expense',
      categoryId: 'category-2',
      name: 'Электричество',
      amountKopecks: 150000,
      startDate: '2026-01-01',
      paymentDay: 5,
      endDate: '2026-12-31',
      comment: 'по счётчику',
      periodicity: 'monthly',
      reminderOffsetDays: 1,
    };

    expect(toCreateWireRequest(command)).toStrictEqual({
      type: 'expense',
      category_id: 'category-2',
      name: 'Электричество',
      amount_kopecks: 150000,
      start_date: '2026-01-01',
      payment_day: 5,
      end_date: '2026-12-31',
      comment: 'по счётчику',
      periodicity: 'monthly',
      reminder_offset_days: 1,
    });
  });

  it('maps update command fields to snake_case wire keys incl. apply_from_date', () => {
    const command: RecurringOperationUpdateRequest = {
      type: 'expense',
      categoryId: 'category-3',
      name: 'Вода',
      amountKopecks: 90000,
      startDate: '2026-03-01',
      paymentDay: 7,
      endDate: undefined,
      comment: undefined,
      periodicity: 'yearly',
      applyFromDate: '2026-09-01',
      reminderOffsetDays: 0,
    };

    expect(toUpdateWireRequest(command)).toStrictEqual({
      type: 'expense',
      category_id: 'category-3',
      name: 'Вода',
      amount_kopecks: 90000,
      start_date: '2026-03-01',
      payment_day: 7,
      end_date: undefined,
      comment: undefined,
      periodicity: 'yearly',
      apply_from_date: '2026-09-01',
      reminder_offset_days: 0,
    });
  });
});
