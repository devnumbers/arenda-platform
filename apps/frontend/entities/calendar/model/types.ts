// Entity-модель календаря напоминаний. Мэппится из DTO эндпоинта
// GET /reminders/calendar на границе API, чтобы не тащить generated-типы в widgets.

export type CalendarEntryType = 'operation' | 'system';

// Статус показывается у operation/system — события дня без акцента на время.
export type CalendarEntryStatus = 'pending' | 'sent';

export type CalendarEntryEventType =
  | 'operation_due'
  | 'operation_overdue'
  | 'lease_expiring'
  | 'lease_requires_action';

export type CalendarEntry = {
  readonly id: string;
  readonly type: CalendarEntryType;
  /** UTC date-time из API; локальное время интерпретируется в поясе браузера (равно поясу владельца). */
  readonly scheduledAt: string;
  readonly title: string;
  readonly hasProperty: boolean;
  readonly propertyName: string | null;
  readonly status: CalendarEntryStatus | null;
  readonly eventType: CalendarEntryEventType | null;
  readonly operationId: string | null;
  readonly leaseId: string | null;
};
