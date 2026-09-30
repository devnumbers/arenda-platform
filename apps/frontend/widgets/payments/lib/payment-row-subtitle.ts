import { isDatePaused } from '@/entities/payment';
import type { IsoDate, Payment } from '@/entities/payment';

/**
 * Подзаголовок строки платежа в секциях «Платежи»/«Автоплатежи» (Figma:
 * дата ближайшего вхождения «11 августа»): активная пауза — «на паузе»
 * (даты у приостановленного правила нет), завершённое правило — без
 * подзаголовка. Дата — серверный `nearestDate` («Следующая дата оплаты»,
 * CONTEXT.md, #993): одна истина с глобальной лентой и страницей платежа,
 * после предоплаты строка переезжает на новую дату без пересборки проекции.
 * Проверка паузы остаётся клиентской — по ней выбирается текст состояния,
 * а не дата («сегодня» приходит параметром).
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
  // null сервера — открытая пауза или завершённое правило (после проверки
  // паузы остаётся второе) и страховка контракта: даты следующего
  // вхождения нет — подзаголовка тоже нет.
  return payment.nearestDate === null ? { kind: 'none' } : { kind: 'date', iso: payment.nearestDate };
}
