import type { PaymentReminderOffset } from '../model/types';

/**
 * Опции напоминания о платеже (карта #822, контракт reminderOffsetDays
 * 1|3|7): «за N дней» до даты вхождения. Кортеж — единственный источник
 * контракта на фронте: отсюда берёт метки пикер, валидатор черновика и
 * paymentReminderOptionLabel; порядок — по макету 1084-24863.
 */
export const PAYMENT_REMINDER_OPTIONS = [
  { offset: 1, label: 'За 1 день' },
  { offset: 3, label: 'За 3 дня' },
  { offset: 7, label: 'За 7 дней' },
] as const satisfies ReadonlyArray<{
  readonly offset: PaymentReminderOffset;
  readonly label: string;
}>;

/** Метка оффсета («За 3 дня») — из того же кортежа, что и пикер. */
export function paymentReminderOptionLabel(offset: PaymentReminderOffset): string {
  const option = PAYMENT_REMINDER_OPTIONS.find(
    (candidate) => candidate.offset === offset,
  );
  return option?.label ?? '';
}
