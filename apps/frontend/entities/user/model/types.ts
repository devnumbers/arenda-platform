export type User = {
  readonly id: string;
  readonly phone: string;
  readonly role: 'owner' | 'admin';
  readonly name: string | null;
  readonly surname: string | null;
  readonly patronymic: string | null;
  readonly email: string | null;
  readonly timezone: string | null;
  /** Путь приватного фото профиля (ADR 0065): same-origin стриминг через
   * бэкенд; null — фото нет. */
  readonly photoUrl: string | null;
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

/** Шаг 2 флоу смены почты (#721/#722): подтверждение кода с текущего
 * адреса + сам новый адрес одним запросом — в ответ приходит одноразовый
 * грант и код уходит на новый адрес. */
export type ConfirmCurrentEmailCommand = {
  newEmail: string;
  code: string;
};

/** Шаг 3 флоу смены почты: код с нового адреса + грант из шага 2. */
export type ChangeEmailCommand = {
  grant: string;
  code: string;
};

/** Повторная отправка кода на новый адрес по живому гранту (#732/#733):
 * код шага 1 уже сожжён confirm-current, resend-плитка — единственный
 * путь повторной доставки. */
export type ResendEmailCodeCommand = {
  grant: string;
};

export type { TariffName } from '@/shared/model/tariff';

