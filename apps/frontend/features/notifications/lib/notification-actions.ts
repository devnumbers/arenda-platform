/**
 * Словарь кнопок страницы уведомления (#745, Действие — решение #737):
 * каждое живое действие бэка (вычислено при чтении) получает лейбл макета
 * и переход на экран сущности — кнопка никогда не мутирует. Переходы
 * строятся только из ссылок payload; живой ссылки нет — кнопки нет.
 * Продлить — secondary по макету 2316:143297 (пара аренды), остальные
 * primary (2316:143530/143588/143647, 2333:184060).
 */

import { ROUTES } from '@/shared/config/routes';
import type { NotificationActionKind, NotificationPayload } from '@/entities/notification';

export type NotificationActionView = {
  readonly label: string;
  readonly href: string;
  readonly variant: 'primary' | 'secondary';
};

export function notificationActionView(
  action: NotificationActionKind,
  payload: NotificationPayload,
): NotificationActionView | null {
  const propertyId = payload.property?.id;
  switch (action) {
    case 'rental_extend':
      return propertyId
        ? { label: 'Продлить', href: ROUTES.propertyRentalExtend(propertyId), variant: 'secondary' }
        : null;
    case 'rental_complete':
      return propertyId
        ? { label: 'Завершить', href: ROUTES.propertyRentalComplete(propertyId), variant: 'primary' }
        : null;
    case 'open_payment':
      return propertyId && payload.paymentId
        ? { label: 'Оплатить', href: ROUTES.propertyPayment(propertyId, payload.paymentId), variant: 'primary' }
        : null;
    case 'open_task':
      // Экран задачи — экран её правила (#750): объектная задача ведёт
      // через объект, безобъектная — по плоскому маршруту (ADR 0052).
      return payload.taskRuleId
        ? {
            label: 'Выполнить',
            href: propertyId
              ? ROUTES.propertyTaskEdit(propertyId, payload.taskRuleId)
              : ROUTES.taskEdit(payload.taskRuleId),
            variant: 'primary',
          }
        : null;
    case 'open_property':
      return propertyId
        ? { label: 'Принять', href: ROUTES.property(propertyId), variant: 'primary' }
        : null;
    case 'open_property_members':
      return { label: 'Участники', href: ROUTES.participants, variant: 'primary' };
    case 'open_tariffs':
      return { label: 'Оплатить', href: ROUTES.profileTariff, variant: 'primary' };
    case 'open_payment_methods':
      return { label: 'Способ оплаты', href: ROUTES.profilePaymentMethods, variant: 'primary' };
  }
}
