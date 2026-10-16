import { isDatePaused } from '@/entities/payment';
import type { IsoDate, Payment } from '@/entities/payment';

/**
 * Порядок секций «Платежи»/«Автоплатежи» (Figma 1043:57610, решение
 * владельца): по дате ближайшего вхождения, самое раннее сверху. Ключ —
 * серверный `nearestDate` («Следующая дата оплаты», CONTEXT.md, #993):
 * одна истина с глобальной лентой, после предоплаты правило встаёт по
 * новой дате без пересборки проекции. Правила без даты (завершённые —
 * сервер дал null) и приостановленные (дат нет по определению паузы,
 * проверка по «сегодня» параметром) уходят в конец списка. Сортировка
 * стабильна: при равной дате сохраняется серверный порядок заведения.
 */
export function sortPaymentsByNextOccurrence(
  payments: ReadonlyArray<Payment>,
  today: IsoDate,
): Payment[] {
  const withKey = payments.map((payment) => {
    const nearest = isDatePaused(payment.pauses, today) ? null : payment.nearestDate;
    return { payment, nearest };
  });
  withKey.sort((a, b) => {
    if (a.nearest === null && b.nearest === null) return 0;
    if (a.nearest === null) return 1;
    if (b.nearest === null) return -1;
    return a.nearest < b.nearest ? -1 : a.nearest > b.nearest ? 1 : 0;
  });
  return withKey.map((entry) => entry.payment);
}
