export type FreeReminderPeriodicity = 'once' | 'daily' | 'weekly' | 'monthly' | 'yearly';

export type PeriodicityOption = {
  readonly value: FreeReminderPeriodicity;
  readonly label: string;
};

export const PERIODICITY_OPTIONS: readonly PeriodicityOption[] = [
  { value: 'once', label: 'Разовое' },
  { value: 'daily', label: 'Каждый день' },
  { value: 'weekly', label: 'Каждую неделю' },
  { value: 'monthly', label: 'Каждый месяц' },
  { value: 'yearly', label: 'Каждый год' },
] as const;

export const PERIODICITY_LABELS: Readonly<Record<FreeReminderPeriodicity, string>> = {
  once: 'Разовое',
  daily: 'Каждый день',
  weekly: 'Каждую неделю',
  monthly: 'Каждый месяц',
  yearly: 'Каждый год',
};

/** Свободное напоминание (entity-модель, camelCase). */
export type FreeReminder = {
  readonly id: string;
  readonly ownerId: string;
  readonly propertyId: string;
  readonly title: string;
  readonly triggerAt: string;
  readonly periodicity: FreeReminderPeriodicity;
  readonly createdAt: string;
  readonly updatedAt: string;
};

/** Ближайшее срабатывание периодического напоминания (entity-модель, camelCase). */
export type UpcomingFreeReminder = {
  readonly freeReminderId: string;
  readonly title: string;
  readonly propertyId: string;
  readonly triggerAt: string;
  readonly periodicity: FreeReminderPeriodicity;
};

/** Команда создания напоминания (camelCase; wire-формат сериализуется в api/hooks). */
export type FreeReminderCreateRequest = {
  readonly title: string;
  readonly triggerAt: string;
  readonly periodicity: FreeReminderPeriodicity;
};

/** Команда обновления напоминания (camelCase; wire-формат сериализуется в api/hooks). */
export type FreeReminderUpdateRequest = {
  readonly title?: string;
  readonly triggerAt?: string;
  readonly periodicity?: FreeReminderPeriodicity;
};
