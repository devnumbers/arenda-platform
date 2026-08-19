import type { components } from '@/shared/api/dto';

export type User = {
  readonly id: string;
  readonly phone: string;
  readonly role: 'owner' | 'admin';
  readonly name: string | null;
  readonly surname: string | null;
  readonly patronymic: string | null;
  readonly email: string | null;
  readonly timezone: string | null;
  readonly subscription: {
    readonly tariff: {
      readonly name: string;
    };
  } | null;
};

export type UserUpdateCommand = {
  name?: string | null;
  surname?: string | null;
  patronymic?: string | null;
  email?: string | null;
  timezone?: string | null;
};

export type SendPhoneChangeCodeCommand = {
  phone: string;
};

export type ChangePhoneCommand = {
  phone: string;
  code: string;
};

export type { TariffName } from '@/shared/model/tariff';

// Свободные напоминания выведены из продукта (тикет #381), но enum в
// сгенерированном API-клиенте ещё содержит 'free_reminder' — регенерация
// контрактного клиента едет следующим тикетом набора. Рантайм-список —
// источник истины для разделения живых и выведенных строк в маппере.
export const NOTIFICATION_EVENT_TYPES = [
  'operation_due',
  'operation_overdue',
  'lease_expiring',
  'lease_requires_action',
  'subscription_grace',
] as const;

export type NotificationEventType = (typeof NOTIFICATION_EVENT_TYPES)[number];

export type NotificationPreference = {
  readonly eventType: NotificationEventType;
  readonly emailAllowed: boolean;
  readonly pushAllowed: boolean;
};

type ApiNotificationPreference = components['schemas']['NotificationPreference'];

/**
 * Строка настроек события, выведенного из продукта: UI её не показывает, но
 * PUT /notification-preferences сервер валидирует полным набором событий,
 * пока его домен не почищен (тикет #381) — поэтому такие строки переносятся
 * в payload как есть, без изменений значений.
 */
export type CarriedNotificationPreference = {
  readonly eventType: Exclude<ApiNotificationPreference['event_type'], NotificationEventType>;
  readonly emailAllowed: boolean;
  readonly pushAllowed: boolean;
};
