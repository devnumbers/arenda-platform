import { isDatePaused } from '@/entities/payment';
import type { IsoDate, Payment } from '@/entities/payment';

/**
 * Подзаголовок строки платежа в секциях «Платежи»/«Автоплатежи» (Figma:
 * дата ближайшего вхождения «11 августа»): активная пауза — «на паузе»
 * (даты у приостановленного правила нет), вычисляемая завершённость —
 * «Полностью оплачен» (#1341: серверный isCompleted — глоссарий
 * «Завершённый платёж»; пауза проверяется первой — за вырезанным курсором
 * открытой паузы флаг отвечает «завершён», а строка честно показывает
 * паузу). Дата — серверный `nearestDate` («Следующая дата оплаты»,
 * CONTEXT.md, #993): одна истина с глобальной лентой и страницей платежа,
 * после предоплаты строка переезжает на новую дату без пересборки
 * проекции. Проверка паузы остаётся клиентской — по ней выбирается текст
 * состояния, а не дата («сегодня» приходит параметром).
 */
/** Подпись вычисляемой завершённости (#1341) — одна на все поверхности
 * строк: объектные списки (PaymentRow) и карточки фида (nearestDateLine). */
export const PAYMENT_COMPLETED_LABEL = 'Полностью оплачен';

export type PaymentRowSubtitle =
  | { readonly kind: 'date'; readonly iso: IsoDate }
  | { readonly kind: 'paused' }
  | { readonly kind: 'completed' }
  | { readonly kind: 'none' };

export function paymentRowSubtitle(
  payment: Payment,
  today: IsoDate,
): PaymentRowSubtitle {
  if (isDatePaused(payment.pauses, today)) {
    return { kind: 'paused' };
  }
  if (payment.isCompleted) {
    return { kind: 'completed' };
  }
  // null сервера — открытая пауза или завершённое правило (после проверки
  // паузы и флага остаётся страховка контракта): даты следующего
  // вхождения нет — подзаголовка тоже нет.
  return payment.nearestDate === null ? { kind: 'none' } : { kind: 'date', iso: payment.nearestDate };
}
