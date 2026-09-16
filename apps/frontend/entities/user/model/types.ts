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

/** Единственное событие доставочных уведомлений после удаления домена аренд (спека #434). */
export type NotificationEventType = 'subscription_grace';

export type NotificationPreference = {
  readonly eventType: NotificationEventType;
  readonly emailAllowed: boolean;
  readonly pushAllowed: boolean;
};

