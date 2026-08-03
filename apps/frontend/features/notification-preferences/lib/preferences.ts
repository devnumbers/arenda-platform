import type {
  NotificationEventType,
  NotificationPreference,
} from '@/entities/user/model/types';

export const NOTIFICATION_OPTIONS: {
  readonly eventType: NotificationEventType;
  readonly label: string;
  readonly description: string;
}[] = [
  {
    eventType: 'operation_due',
    label: 'Предстоящие операции',
    description: 'За 1, 3 или 7 дней до операции; срок выбирается в самой операции',
  },
  {
    eventType: 'operation_overdue',
    label: 'Просроченные операции',
    description: 'Если операция не отмечена вовремя',
  },
  {
    eventType: 'lease_expiring',
    label: 'Окончание аренды',
    description: 'За 30 дней до конца аренды',
  },
  {
    eventType: 'lease_requires_action',
    label: 'Аренда требует действия',
    description: 'Аренда закончилась и ждёт решения',
  },
  {
    eventType: 'free_reminder',
    label: 'Свои напоминания',
    description: 'Созданные вами напоминания о любых датах по объектам',
  },
];

export type NotificationPreferencesState = Record<NotificationEventType, boolean>;

export function buildInitialPreferences(
  preferences: NotificationPreference[],
): NotificationPreferencesState {
  const state: NotificationPreferencesState = {
    operation_due: true,
    operation_overdue: true,
    lease_expiring: true,
    lease_requires_action: true,
    free_reminder: true,
  };
  for (const preference of preferences) {
    state[preference.eventType] = preference.allowed;
  }
  return state;
}
