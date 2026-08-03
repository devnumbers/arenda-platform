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
