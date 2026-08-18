import type { components } from '@/shared/api/dto';
import type { Operation, OperationsPage, RecurringOperation } from './types';

type OperationResponse = components['schemas']['OperationResponse'];
type OperationsResponse = components['schemas']['OperationsResponse'];
type RecurringOperationResponse =
  components['schemas']['RecurringOperationResponse'];

export function mapOperationResponse(dto: OperationResponse): Operation {
  return {
    id: dto.id,
    ownerId: dto.owner_id,
    propertyId: dto.property_id ?? null,
    propertyStatus: dto.property_status,
    leaseId: dto.lease_id,
    recurringOperationId: dto.recurring_operation_id,
    type: dto.type,
    categoryId: dto.category_id,
    categoryName: dto.category_name,
    name: dto.name,
    amountKopecks: dto.amount_kopecks,
    operationDate: dto.operation_date,
    status: dto.status,
    comment: dto.comment ?? null,
    isException: dto.is_exception,
    reminderOffsetDays: dto.reminder_offset_days,
    createdAt: dto.created_at,
    updatedAt: dto.updated_at,
  };
}

export function mapOperationsResponse(dto: OperationsResponse): OperationsPage {
  return {
    items: dto.items.map(mapOperationResponse),
    limit: dto.limit,
    offset: dto.offset,
    hasMore: dto.has_more,
    nextOffset: dto.next_offset ?? undefined,
  };
}

export function mapRecurringOperationResponse(
  dto: RecurringOperationResponse,
): RecurringOperation {
  return {
    id: dto.id,
    ownerId: dto.owner_id,
    propertyId: dto.property_id ?? null,
    leaseId: dto.lease_id,
    type: dto.type,
    categoryId: dto.category_id,
    categoryName: dto.category_name,
    name: dto.name,
    amountKopecks: dto.amount_kopecks,
    startDate: dto.start_date,
    paymentDay: dto.payment_day,
    endDate: dto.end_date,
    periodicity: dto.periodicity,
    status: dto.status,
    comment: dto.comment ?? null,
    reminderOffsetDays: dto.reminder_offset_days,
    createdAt: dto.created_at,
    updatedAt: dto.updated_at,
  };
}
