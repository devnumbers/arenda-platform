import { describe, expect, it } from 'vitest';
import type { OperationCreateRequest, OperationUpdateRequest } from '@/entities/operation';
import { toCreateWireRequest, toUpdateWireRequest } from './hooks';

describe('operations wire serializers', () => {
  it('maps create command fields to snake_case wire keys', () => {
    const command: OperationCreateRequest = {
      type: 'income',
      categoryId: 'category-1',
      name: 'Аренда за июнь',
      amountKopecks: 4500000,
      operationDate: '2026-06-10',
      comment: 'предоплата',
      leaseId: 'lease-1',
      reminderOffsetDays: 3,
    };

    expect(toCreateWireRequest(command)).toStrictEqual({
      type: 'income',
      category_id: 'category-1',
      name: 'Аренда за июнь',
      amount_kopecks: 4500000,
      operation_date: '2026-06-10',
      comment: 'предоплата',
      lease_id: 'lease-1',
      reminder_offset_days: 3,
    });
  });

  it('maps update command fields to snake_case wire keys', () => {
    const command: OperationUpdateRequest = {
      type: 'expense',
      categoryId: 'category-2',
      name: 'Электричество',
      amountKopecks: 150000,
      operationDate: '2026-07-01',
      comment: undefined,
      leaseId: undefined,
      reminderOffsetDays: 0,
    };

    expect(toUpdateWireRequest(command)).toStrictEqual({
      type: 'expense',
      category_id: 'category-2',
      name: 'Электричество',
      amount_kopecks: 150000,
      operation_date: '2026-07-01',
      comment: undefined,
      lease_id: undefined,
      reminder_offset_days: 0,
    });
  });
});
