import type { IsoDate, Payment, PaymentOperation, PaymentType } from '@/entities/payment';
import { nextOccurrenceAfter, occurrencesBetween } from '@/entities/payment';
import { TYPE_LABELS } from './payment-labels';

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
  return TYPE_LABELS[type];
}

/**
 * Ближайшее вхождение для секции «Ближайшая операция» (1096:37792):
 * материализованное плановое — источник истины (после оплаты тик
 * материализует следующее, и секция переезжает на него, как «График»);
 * проекция портом вхождений — только пока сервер ничего не материализовал,
 * день «сегодня» включается (честный вид до прогона тика). undefined —
 * правило исчерпано (завершённое без planned). Активная пауза разрешается
 * вызывающим (секция показывает «На паузе»).
 */
export type NearestOccurrence =
  | { readonly kind: 'operation'; readonly operation: PaymentOperation }
  | { readonly kind: 'projected'; readonly date: IsoDate };

export function nearestOccurrence(
  payment: Payment,
  planned: ReadonlyArray<PaymentOperation>,
  today: IsoDate,
): NearestOccurrence | undefined {
  const firstPlanned = planned[0];
  if (firstPlanned !== undefined) {
    return { kind: 'operation', operation: firstPlanned };
  }
  const todayOccurrence = occurrencesBetween(payment, today, today)[0];
  const projected = todayOccurrence ?? nextOccurrenceAfter(payment, today);
  // nextOccurrenceAfter сигналит исчерпание null'ом.
  return projected === null ? undefined : { kind: 'projected', date: projected };
}
