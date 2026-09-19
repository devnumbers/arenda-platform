/**
 * Доменная модель уведомлений на фронте (словарь — notifications/CONTEXT.md,
 * ADR 0056, решение #737): хранимая лента пишется всем получателям всегда;
 * настройки каналов ленту не глушат. Моменты — UTC-строки (created_at),
 * группировка по дням — дело экрана (клиентский день смотрящего, DESIGN.md §9).
 */

/** Категория уведомлений (решение #737): группа настроек и иконка строки.
 * Каталог v1 фиксирован бэком; Тариф и Системные — сервисные, вне экрана
 * настроек. Кортеж — единственный источник каталога: юнион ниже выводится
 * из него (канон NOTIFICATION_ACTION_KINDS), дрейфа списков не бывает. */
export const NOTIFICATION_CATEGORIES = [
  'rental',
  'payments_operations',
  'tasks',
  'shared_access',
  'tariff',
  'system',
] as const;

export type NotificationCategory = (typeof NOTIFICATION_CATEGORIES)[number];

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

/**
 * Кнопка-переход на экран сущности (Действие, решение #737): enum закрыт
 * бэком, вычисляется при чтении по живому состоянию и правам читателя;
 * экран рендерит только то, что пришло (никогда не мутация). Кортеж —
 * единственный источник каталога: union ниже, Set маппера и тест полного
 * покрытия выводятся из него, дрейфа между списками не бывает.
 */
export const NOTIFICATION_ACTION_KINDS = [
  'rental_extend',
  'rental_complete',
  'open_payment',
  'open_task',
  'open_property',
  'open_property_members',
  'open_tariffs',
  'open_payment_methods',
] as const;

export type NotificationActionKind = (typeof NOTIFICATION_ACTION_KINDS)[number];

/** Payload-ссылка со снимком имени (EntityRef, решение #737): id для
 * перехода, имя для карточки — переживает переименование и удаление
 * сущности. Строки карточек сверх имени — тоже снимки момента публикации
 * (решение владельца 19.09.2026, #745): объект несёт адрес, приглашающий —
 * email; строки, которых в снимке нет, карточка не рисует. */
export type NotificationEntityRef = {
  readonly id: string;
  readonly name: string;
  readonly address?: string;
  readonly email?: string;
};

/** Тарифный снимок биллинг-событий (TariffRef); сумма — BIGINT копейки. */
export type NotificationTariffRef = {
  readonly slug: string;
  readonly period: string;
  readonly amountKopecks: number;
  readonly activeUntil: string | null;
};

/**
 * Payload страницы уведомления (решение #737): ссылки и снимки имён,
 * все поля опциональны — текст-снимок title/body самодостаточен, payload
 * добавляет навигацию и карточки.
 */
export type NotificationPayload = {
  readonly property?: NotificationEntityRef;
  readonly actor?: NotificationEntityRef;
  readonly rentalId?: string;
  /** Id правила (Платёж); переход кнопки «Оплатить» ведёт на его страницу. */
  readonly paymentId?: string;
  /** Дата операции (решение #737: payment = правило + дата операции, #749). */
  readonly paymentDate?: string;
  readonly taskId?: string;
  readonly membershipId?: string;
  readonly invitationId?: string;
  readonly tariff?: NotificationTariffRef;
};

/**
 * Страница уведомления (GET /notifications/{id}, #745): строка ленты плюс
 * payload-ссылки и живые действия этого читателя (вычислены при чтении —
 * после выполнения действия кнопки пропадают, решение #737).
 */
export type NotificationDetail = Notification & {
  readonly payload: NotificationPayload;
  readonly actions: readonly NotificationActionKind[];
};
