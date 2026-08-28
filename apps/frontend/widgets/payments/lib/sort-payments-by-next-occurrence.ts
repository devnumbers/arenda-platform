import { isDatePaused, nextOccurrenceAfter } from '@/entities/payment';
import type { IsoDate, Payment } from '@/entities/payment';

/**
 * Порядок секций «Платежи»/«Автоплатежи» (Figma 1043:57610, решение
 * владельца): по дате ближайшего вхождения, самое раннее сверху. Правила без
 * вхождений (завершённые — endDate в прошлом) и приостановленные (дат нет по
 * определению паузы) уходят в конец списка. Сортировка стабильна: при равной
 * дате сохраняется серверный порядок заведения. Чистая функция — «сегодня»
 * приходит параметром (клиентская проекция, entities/payment/lib).
 */
export function sortPaymentsByNextOccurrence(
  payments: ReadonlyArray<Payment>,
  today: IsoDate,
): Payment[] {
  const withKey = payments.map((payment) => {
    const next = isDatePaused(payment.pauses, today)
      ? null
      : nextOccurrenceAfter(payment, today);
    return { payment, next };
  });
  withKey.sort((a, b) => {
    if (a.next === null && b.next === null) return 0;
    if (a.next === null) return 1;
    if (b.next === null) return -1;
    return a.next < b.next ? -1 : a.next > b.next ? 1 : 0;
  });
  return withKey.map((entry) => entry.payment);
}
