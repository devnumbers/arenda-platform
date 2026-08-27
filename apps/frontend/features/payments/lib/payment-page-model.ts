import type { IsoDate, Payment, PaymentOperation, PaymentType } from '@/entities/payment';

/**
 * Чистая модель страницы платежа (#465): выбор цели кнопки «Оплатить»,
 * признак завершённости правила и подпись направления. Даты — date-строки,
 * «сегодня» приходит параметром (ADR 0048): в этих функциях нет часов.
 */

/**
 * Старейшее неоплаченное вхождение для «Оплатить» (история 25 спеки #453):
 * просроченные в приоритете, затем плановые. Оба списка — asc по плановой
 * дате (порядок закреплён за API), поэтому цель — head первого непустого.
 */
export function oldestUnpaidOperation(
  overdue: ReadonlyArray<PaymentOperation>,
  planned: ReadonlyArray<PaymentOperation>,
): PaymentOperation | null {
  return overdue[0] ?? planned[0] ?? null;
}

/**
 * Завершённое правило: `endDate` в прошлом. День окончания включён в
 * расписание (домен-порт entities/payment/lib/occurrences), поэтому
 * завершённость наступает только на следующий день после него; у бессрочного
 * endDate нет — не завершён никогда. ISO-строки сравниваются лексикографически.
 */
export function isPaymentCompleted(payment: Payment, today: IsoDate): boolean {
  return payment.endDate !== undefined && payment.endDate < today;
}

/** Подпись направления карточки («Расход» / «Доход»). */
export function paymentTypeLabel(type: PaymentType): string {
  return type === 'income' ? 'Доход' : 'Расход';
}
