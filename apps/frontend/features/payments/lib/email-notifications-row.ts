import type { PaymentReminderOffset } from '@/entities/payment';
import type { PaymentDraftType } from './use-payment-wizard-draft';

/**
 * Гейт ряда «Уведомления на почту» (EmailNotificationsRow) на экранах
 * создания и правки платежа/автоплатежа (решение владельца 22.10.2026):
 * пер-платёжный выбор «не надо» гасит весь ряд — уведомлений и напоминаний
 * по платежу нет, шоткату глобальной email-настройки нечего включать.
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
