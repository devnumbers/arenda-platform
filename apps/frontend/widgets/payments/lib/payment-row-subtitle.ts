import { isDatePaused, nextOccurrenceAfter } from '@/entities/payment';
import type { IsoDate, Payment } from '@/entities/payment';

/**
 * Подзаголовок строки платежа в секциях «Платежи»/«Автоплатежи» (Figma:
 * дата ближайшего вхождения «11 августа»): активная пауза — «на паузе»
 * (даты у приостановленного правила нет), завершённое правило — без
 * подзаголовка. Чистая функция над клиентским портом прототипа: «сегодня»
 * приходит параметром (клиентская проекция — см. client-today.ts).
 */
export type PaymentRowSubtitle =
  | { readonly kind: 'date'; readonly iso: IsoDate }
  | { readonly kind: 'paused' }
  | { readonly kind: 'none' };

export function paymentRowSubtitle(
  payment: Payment,
  today: IsoDate,
): PaymentRowSubtitle {
  if (isDatePaused(payment.pauses, today)) {
    return { kind: 'paused' };
  }
  const next = nextOccurrenceAfter(payment, today);
  return next === null ? { kind: 'none' } : { kind: 'date', iso: next };
}
