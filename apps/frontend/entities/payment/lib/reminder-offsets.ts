import type { PaymentReminderOffset } from '../model/types';

/**
 * Опции напоминания о платеже (карта #822, контракт reminderOffsetDays
 * 1|3|7): «за N дней» до даты вхождения. Метки — родительный падеж по
 * числу (день/дня/дней); кортеж — источник порядка строк пикера
 * (макет 1084-24863), метки агрегирует paymentReminderOptionLabel.
 */
export const PAYMENT_REMINDER_OPTIONS = [
  { offset: 1, label: 'За 1 день' },
  { offset: 3, label: 'За 3 дня' },
  { offset: 7, label: 'За 7 дней' },
] as const satisfies ReadonlyArray<{
  readonly offset: PaymentReminderOffset;
  readonly label: string;
}>;

/** Метка оффала («За 3 дня»); switch исчерпывающий — новый оффал
 * контракта потребует метку компилятором. */
export function paymentReminderOptionLabel(offset: PaymentReminderOffset): string {
  switch (offset) {
    case 1:
      return 'За 1 день';
    case 3:
      return 'За 3 дня';
    case 7:
      return 'За 7 дней';
  }
}
