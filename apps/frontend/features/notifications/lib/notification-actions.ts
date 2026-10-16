/**
 * Словарь кнопок страницы уведомления (#745, Действие — решение #737):
 * каждое живое действие бэка (вычислено при чтении) получает лейбл макета
 * и переход на экран сущности — кнопка никогда не мутирует. Переходы
 * строятся только из ссылок payload; живой ссылки нет — кнопки нет.
 * Продлить — secondary по макету 2316:143297 (пара аренды), остальные
 * primary (2316:143530/143588/143647, 2333:184060). Исключение —
 * open_tariffs: у №14 «Тариф изменён» макета нет, лейбл из решения
 * каталога #737 (см. комментарий у кейса). Лейбл open_property выбирается
 * по паре (event_type, action) — решение владельца 12.10.2026, #1359.
 */

import { ROUTES } from '@/shared/config/routes';
import type { NotificationActionKind, NotificationPayload } from '@/entities/notification';

export type NotificationActionView = {
  readonly label: string;
  readonly href: string;
  readonly variant: 'primary' | 'secondary';
};

/**
 * Лейблы open_property по паре (event_type, action) (#1359, решение
 * владельца 12.10.2026): приглашение — «В объект» (вместо «Принять» —
 * приглашение принимается по email-ссылке, лента лишь навигирует), смена
 * роли — «Перейти в объект» (было «Участники» — owner-only /participants,
 * мёртвая для участника-получателя кнопка). Каталог событий закрыт бэком,
 * других продуцентов open_property нет; неизвестное событие — «В объект»:
 * навигационная семантика действия.
 */
const OPEN_PROPERTY_LABELS: Readonly<Record<string, string>> = {
  property_invitation: 'В объект',
  access_role_changed: 'Перейти в объект',
};

export function notificationActionView(
  eventType: string,
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
        ? { label: OPEN_PROPERTY_LABELS[eventType] ?? 'В объект', href: ROUTES.property(propertyId), variant: 'primary' }
        : null;
    case 'open_property_members':
      return { label: 'Участники', href: ROUTES.participants, variant: 'primary' };
    case 'open_tariffs':
      // №14 «Тариф изменён» (#788): лейбл «Тариф» — по решению каталога
      // #737 («переход к тарифам»); макета у №14 нет, «Оплатить» из макета
      // grace-пары (2316:143647) не переносится — даунгрейд не оплата.
      return { label: 'Тариф', href: ROUTES.profileTariff, variant: 'primary' };
    case 'open_payment_methods':
      return { label: 'Способ оплаты', href: ROUTES.profilePaymentMethods, variant: 'primary' };
  }
}
