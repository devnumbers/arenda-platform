export type OperationType = 'income' | 'expense';

export type OperationStatus = 'pending' | 'overdue' | 'paid' | 'received' | 'unconfirmed';

export type OperationFrequency = 'once' | 'monthly' | 'yearly';

export type RecurringOperationPeriodicity = 'monthly' | 'yearly';

export type RecurringOperationStatus = 'active' | 'paused';

/** Статус объекта, в котором лежит операция (null-объект после detach-удаления объекта). */
export type OperationPropertyStatus = 'active' | 'maintenance' | 'archived';

/**
 * Операция — запись о доходе/расходе (entity-модель, camelCase).
 * Деньги — целые копейки; DTO-поля маппятся в model/mappers.ts.
 */
export type Operation = {
  readonly id: string;
  readonly ownerId: string;
  readonly propertyId: string | null;
  readonly propertyStatus?: OperationPropertyStatus;
  readonly leaseId?: string | null;
  readonly recurringOperationId?: string | null;
  readonly type: OperationType;
  readonly categoryId: string;
  readonly categoryName: string;
  readonly name: string;
  readonly amountKopecks: number;
  readonly operationDate: string;
  readonly status: OperationStatus;
  readonly comment?: string | null;
  readonly isException: boolean;
  readonly reminderOffsetDays?: 1 | 3 | 7 | null;
  readonly createdAt: string;
  readonly updatedAt: string;
};

/** Постатаничная выдача операций (entity-аналог OperationsResponse). */
export type OperationsPage = {
  readonly items: ReadonlyArray<Operation>;
  readonly limit: number;
  readonly offset: number;
  readonly hasMore: boolean;
  readonly nextOffset?: number;
};

/** Команда создания операции (camelCase; wire-формат сериализуется в features/operations). */
export type OperationCreateRequest = {
  readonly type: OperationType;
  readonly categoryId: string;
  readonly name: string;
  readonly amountKopecks: number;
  readonly operationDate: string;
  readonly comment?: string;
  readonly leaseId?: string;
  readonly reminderOffsetDays?: 0 | 1 | 3 | 7 | null;
};

/** Команда обновления операции (camelCase; wire-формат сериализуется в features/operations). */
export type OperationUpdateRequest = {
  readonly type?: OperationType;
  readonly categoryId?: string;
  readonly name?: string;
  readonly amountKopecks?: number;
  readonly operationDate?: string;
  readonly comment?: string;
  readonly leaseId?: string;
  readonly reminderOffsetDays?: 0 | 1 | 3 | 7 | null;
};

/** Серия периодических операций (entity-модель, camelCase). */
export type RecurringOperation = {
  readonly id: string;
  readonly ownerId: string;
  readonly propertyId: string | null;
  readonly leaseId?: string | null;
  readonly type: OperationType;
  readonly categoryId: string;
  readonly categoryName: string;
  readonly name: string;
  readonly amountKopecks: number;
  readonly startDate: string;
  readonly paymentDay: number;
  readonly endDate?: string | null;
  readonly periodicity: RecurringOperationPeriodicity;
  readonly status: RecurringOperationStatus;
  readonly comment?: string | null;
  readonly reminderOffsetDays?: 1 | 3 | 7 | null;
  readonly createdAt: string;
  readonly updatedAt: string;
};

/** Команда создания серии (camelCase; wire-формат сериализуется в features/recurring-operations). */
export type RecurringOperationCreateRequest = {
  readonly type: OperationType;
  readonly categoryId: string;
  readonly name: string;
  readonly amountKopecks: number;
  readonly startDate: string;
  readonly paymentDay?: number;
  readonly endDate?: string;
  readonly comment?: string;
  readonly periodicity?: RecurringOperationPeriodicity;
  readonly reminderOffsetDays?: 0 | 1 | 3 | 7 | null;
};

/** Команда обновления серии (camelCase; wire-формат сериализуется в features/recurring-operations). */
export type RecurringOperationUpdateRequest = {
  readonly type?: OperationType;
  readonly categoryId?: string;
  readonly name?: string;
  readonly amountKopecks?: number;
  readonly startDate?: string;
  readonly paymentDay?: number;
  readonly endDate?: string;
  readonly comment?: string;
  readonly periodicity?: RecurringOperationPeriodicity;
  readonly applyFromDate?: string;
  readonly reminderOffsetDays?: 0 | 1 | 3 | 7 | null;
};
