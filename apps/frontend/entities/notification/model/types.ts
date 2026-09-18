/**
 * Доменная модель уведомлений на фронте (словарь — notifications/CONTEXT.md,
 * ADR 0056, решение #737): хранимая лента пишется всем получателям всегда;
 * настройки каналов ленту не глушат. Моменты — UTC-строки (created_at),
 * группировка по дням — дело экрана (клиентский день смотрящего, DESIGN.md §9).
 */

/** Категория уведомлений (решение #737): группа настроек и иконка строки.
 * Каталог v1 фиксирован бэком; Тариф и Системные — сервисные, вне экрана
 * настроек. */
export type NotificationCategory =
  | 'rental'
  | 'payments_operations'
  | 'tasks'
  | 'shared_access'
  | 'tariff'
  | 'system';

/**
 * Строка ленты одного получателя (GET /notifications): текст — снимок
 * момента публикации, живое состояние действий — на странице уведомления
 * (#745). Прочитанность — readAt: null значит непрочитано.
 */
export type Notification = {
  readonly id: string;
  /** Тип события каталога v1 — ключ страницы уведомления (#745). */
  readonly eventType: string;
  readonly category: NotificationCategory;
  readonly title: string;
  readonly body: string;
  /** Строка над заголовком — имя объекта/тарифа или «Системные уведомления». */
  readonly contextLabel: string | null;
  /** UTC-момент события. */
  readonly createdAt: string;
  /** UTC-момент прочтения; null — непрочитано (красная точка строки). */
  readonly readAt: string | null;
};

/** Непрочитанное уведомление — точка на иконке и полная насыщенность. */
export function isNotificationUnread(notification: Notification): boolean {
  return notification.readAt === null;
}
