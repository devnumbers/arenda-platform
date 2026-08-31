import type { IsoDate, Payment, PaymentOperation } from '@/entities/payment';
import { occurrencesBetween } from '@/entities/payment';

/**
 * Проекционная «операция» для просмотра (решение владельца — чисто фронт):
 * будущее вхождение правила ещё не материализовано сервером (в БД лежит
 * ровно одна будущая planned), страницу просмотра строим из полей правила
 * на дату из клиентской проекции. Вне расписания, в паузе или в прошлом
 * проекции нет — вызывающий показывает «вхождение не найдено».
 *
 * Идентификатора у проекции нет: id кодирует дату (`projected:<date>`),
 * оплачивать её нельзя — платится только материализованная операция.
 */
export function projectedOperation(
  payment: Payment,
  date: IsoDate,
  today: IsoDate,
): PaymentOperation | undefined {
  if (date < today) {
    return undefined;
  }
  const occurs = occurrencesBetween(payment, date, date);
  if (occurs.length === 0) {
    return undefined;
  }
  return {
    id: `projected:${date}`,
    propertyId: payment.propertyId,
    paymentId: payment.id,
    date,
    status: 'planned',
    type: payment.type,
    title: payment.title,
    amountKopecks: payment.amountKopecks,
    paymentForm: payment.paymentForm,
    categoryLabel: payment.category.label,
    categorySlug: payment.category.slug,
  };
}
