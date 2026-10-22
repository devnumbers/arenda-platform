import type { PaymentReminderOffset } from '@/entities/payment';
import type { PaymentDraftType } from './use-payment-wizard-draft';

/**
 * Гейт ряда «Уведомления на почту» на экранах создания и правки
 * платежа/автоплатежа (решение владельца 22.10.2026): пер-платёжный выбор
 * «не надо» гасит весь ряд — уведомлений и напоминаний по платежу нет.
 * «Не надо» — это «Не напоминать» у ручного платежа (reminderOffsetDays
 * пуст) и «Не уведомлять» у автоплатежа (notifyAutoPaid — дефолт черновика
 * undefined и явное false состояния правки).
 */
export function emailNotificationsRowDisabled(
  draftType: PaymentDraftType,
  flags: {
    readonly reminderOffsetDays: PaymentReminderOffset | undefined;
    readonly notifyAutoPaid: boolean | undefined;
  },
): boolean {
  return draftType === 'payment'
    ? flags.reminderOffsetDays === undefined
    : flags.notifyAutoPaid !== true;
}

/**
 * Действующее значение ряда в двухуровневой модели (карта #1551, канон
 * #1552, решение владельца «надо запоминать»): свитч ряда — персональная
 * галочка почты платежа, её значение живёт в форме независимо от радио.
 * Гейт («не надо») показывает и отправляет off, но запомненное значение
 * не стирает — возврат радио в «да» оживляет ряд с ним же. Дифф правки
 * и ряд показывают одно и то же — функция одна.
 */
export function emailNotificationsRowValue(
  disabled: boolean,
  remembered: boolean,
): boolean {
  return disabled ? false : remembered;
}

/** Пара (заглушен, значение) одним вызовом — единственная форма, в которой
 * ряд потребляют экраны (disabled на обёртку, value в свитч и команду),
 * а модели команд — для действующего значения. */
export function emailNotificationsRowState(
  draftType: PaymentDraftType,
  flags: {
    readonly reminderOffsetDays: PaymentReminderOffset | undefined;
    readonly notifyAutoPaid: boolean | undefined;
  },
  remembered: boolean,
): { readonly disabled: boolean; readonly value: boolean } {
  const disabled = emailNotificationsRowDisabled(draftType, flags);
  return { disabled, value: emailNotificationsRowValue(disabled, remembered) };
}
