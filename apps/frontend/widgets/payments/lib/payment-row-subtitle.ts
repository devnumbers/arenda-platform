import { isDatePaused, nextOccurrenceAfter } from '@/entities/payment';
import type { IsoDate, Payment } from '@/entities/payment';

/**
 * Подзаголовок строки платежа в секциях «Платежи»/«Автоплатежи» (Figma:
 * дата ближайшего вхождения «11 августа»): активная пауза — «на паузе»
 * (даты у приостановленного правила нет), завершённое правило — без
 * подзаголовка. Чистая функция над клиентским портом прототипа: «сегодня»
 * приходит параметром (клиентская проекция — см. entities/payment/lib/client-today).
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
  // Завершённое правило (CONTEXT.md «Завершённый платёж»): серверный
  // вычисляемый флаг — дат следующего вхождения у него нет и не будет.
  if (payment.isCompleted) {
    return { kind: 'none' };
  }
  const next = nextOccurrenceAfter(payment, today);
  return next === null ? { kind: 'none' } : { kind: 'date', iso: next };
}
