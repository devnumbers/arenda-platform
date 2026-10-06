/**
 * Warning-бейдж строки ленты и страницы уведомления (#1164): красный
 * StatusIconDanger — только на тарифных событиях «что-то не так». Ключ —
 * тип события, не категория: категория «Тариф» несёт и хорошие новости
 * («Оплата прошла», «Тариф изменён», напоминание), бейдж на них вешать
 * нельзя. Новым тарифным событиям бейдж не достаётся по умолчанию —
 * матрица расширяется явно.
 */

/** Тарифные события-проблемы — единственные носители бейджа (#1164). */
export const NOTIFICATION_WARNING_EVENT_TYPES = [
  'subscription_payment_failed',
  'subscription_grace_entered',
  'subscription_grace_expiring',
] as const;

export type NotificationWarningEventType = (typeof NOTIFICATION_WARNING_EVENT_TYPES)[number];

const WARNING_EVENT_TYPES: ReadonlySet<string> = new Set<string>(NOTIFICATION_WARNING_EVENT_TYPES);

/** Есть ли у события каталога Warning-бейдж (eventType — строка модели
 * уведомления; неизвестный тип бейдж не получает). */
export function notificationHasWarningBadge(eventType: string): boolean {
  return WARNING_EVENT_TYPES.has(eventType);
}
