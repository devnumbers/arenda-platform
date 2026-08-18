import { describe, expect, it } from 'vitest';
import type { components } from '@/shared/api/dto';
import {
  mapOperationResponse,
  mapOperationsResponse,
  mapRecurringOperationResponse,
} from './mappers';

type OperationResponse = components['schemas']['OperationResponse'];
type OperationsResponse = components['schemas']['OperationsResponse'];
type RecurringOperationResponse =
  components['schemas']['RecurringOperationResponse'];

function makeOperationDto(
  overrides: Partial<OperationResponse> = {},
): OperationResponse {
  return {
    id: 'operation-1',
    owner_id: 'owner-1',
    property_id: 'property-1',
    property_status: 'active',
    lease_id: null,
    recurring_operation_id: null,
    type: 'income',
    category_id: 'category-1',
    category_name: 'Аренда',
    name: 'Аренда за июнь',
    amount_kopecks: 4500000,
    operation_date: '2026-06-10',
    status: 'pending',
    comment: null,
    is_exception: false,
    reminder_offset_days: 3,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

function makeRecurringDto(
  overrides: Partial<RecurringOperationResponse> = {},
): RecurringOperationResponse {
  return {
    id: 'recurring-1',
    owner_id: 'owner-1',
    property_id: 'property-1',
    lease_id: null,
    type: 'expense',
    category_id: 'category-2',
    category_name: 'Коммунальные',
    name: 'Электричество',
    amount_kopecks: 150000,
    start_date: '2026-01-01',
    payment_day: 5,
    end_date: null,
    periodicity: 'monthly',
    status: 'active',
    comment: null,
    reminder_offset_days: null,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

describe('mapOperationResponse', () => {
  it('maps snake_case DTO to camelCase entity with all fields', () => {
    const operation = mapOperationResponse(makeOperationDto());

    expect(operation).toEqual({
      id: 'operation-1',
      ownerId: 'owner-1',
      propertyId: 'property-1',
      propertyStatus: 'active',
      leaseId: null,
      recurringOperationId: null,
      type: 'income',
      categoryId: 'category-1',
      categoryName: 'Аренда',
      name: 'Аренда за июнь',
      amountKopecks: 4500000,
      operationDate: '2026-06-10',
      status: 'pending',
      comment: null,
      isException: false,
      reminderOffsetDays: 3,
      createdAt: '2026-01-01T00:00:00Z',
      updatedAt: '2026-01-01T00:00:00Z',
    });
  });

  it('maps an orphan operation (property deleted in detach mode)', () => {
    const operation = mapOperationResponse(
      makeOperationDto({ property_id: null, property_status: undefined }),
    );

    expect(operation.propertyId).toBeNull();
    expect(operation.propertyStatus).toBeUndefined();
  });
});

describe('mapOperationsResponse', () => {
  it('maps items and paging fields', () => {
    const dto: OperationsResponse = {
      items: [makeOperationDto(), makeOperationDto({ id: 'operation-2' })],
      limit: 50,
      offset: 50,
      has_more: true,
      next_offset: 100,
    };

    const page = mapOperationsResponse(dto);

    expect(page.items).toHaveLength(2);
    expect(page.items[1]?.id).toBe('operation-2');
    expect(page.limit).toBe(50);
    expect(page.offset).toBe(50);
    expect(page.hasMore).toBe(true);
    expect(page.nextOffset).toBe(100);
  });

  it('maps the last page without next_offset', () => {
    const page = mapOperationsResponse({
      items: [],
      limit: 50,
      offset: 100,
      has_more: false,
      next_offset: null,
    });

    expect(page.hasMore).toBe(false);
    expect(page.nextOffset).toBeUndefined();
  });
});

describe('mapRecurringOperationResponse', () => {
  it('maps snake_case DTO to camelCase entity', () => {
    const recurring = mapRecurringOperationResponse(
      makeRecurringDto({ end_date: '2026-12-31', status: 'paused', lease_id: 'lease-1' }),
    );

    expect(recurring).toEqual({
      id: 'recurring-1',
      ownerId: 'owner-1',
      propertyId: 'property-1',
      leaseId: 'lease-1',
      type: 'expense',
      categoryId: 'category-2',
      categoryName: 'Коммунальные',
      name: 'Электричество',
      amountKopecks: 150000,
      startDate: '2026-01-01',
      paymentDay: 5,
      endDate: '2026-12-31',
      periodicity: 'monthly',
      status: 'paused',
      comment: null,
      reminderOffsetDays: null,
      createdAt: '2026-01-01T00:00:00Z',
      updatedAt: '2026-01-01T00:00:00Z',
    });
  });
});
