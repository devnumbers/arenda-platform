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

/** Шаг 2 флоу смены email (#721/#722, протокол #1202): код с текущего
 * адреса — сервер проверяет его, сжигает и выпускает одноразовый грант;
 * нового адреса в запросе ещё нет. */
export type VerifyCurrentEmailCommand = {
  code: string;
};

/** Шаг 3 флоу смены email (протокол #1202): новый адрес привязывается
 * к живому гранту, код уходит на новый адрес (204). */
export type RequestNewEmailCodeCommand = {
  grant: string;
  newEmail: string;
};

/** Финальный шаг флоу смены почты: код с нового адреса + грант. */
export type ChangeEmailCommand = {
  grant: string;
  code: string;
};

/** Повторная отправка кода на новый адрес по живому гранту (#732/#733):
 * код шага 1 уже сожжён verify-current, resend-плитка — единственный
 * путь повторной доставки. */
export type ResendEmailCodeCommand = {
  grant: string;
};

export type { TariffName } from '@/shared/model/tariff';

