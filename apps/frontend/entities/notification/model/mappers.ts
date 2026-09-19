/**
 * DTO → entity модели слайса уведомлений (#743, карта #734). Контракт
 * snake_case: event_type, context_label, read_at; опциональные поля
 * нормализуются к null.
 */

import type { components } from '@/shared/api/dto';
import type {
  Notification,
  NotificationActionKind,
  NotificationCategory,
  NotificationDetail,
  NotificationEntityRef,
  NotificationPayload,
  NotificationTariffRef,
} from './types';
import { NOTIFICATION_ACTION_KINDS } from './types';

type NotificationItemDto = components['schemas']['NotificationItem'];
type NotificationDetailDto = components['schemas']['NotificationDetailResponse'];

/** Категория — строка в openapi, но каталог v1 (#737) фиксирован бэком. */
function toCategory(value: string): NotificationCategory {
  return value as NotificationCategory;
}

export function mapNotification(dto: NotificationItemDto): Notification {
  return {
    id: dto.id,
    eventType: dto.event_type,
    category: toCategory(dto.category),
    title: dto.title,
    body: dto.body,
    contextLabel: dto.context_label ?? null,
    createdAt: dto.created_at,
    readAt: dto.read_at ?? null,
  };
}

/** Payload путешествует free-form объектом (контракт #743) — урезаем до
 * известного словаря и переводим в camelCase; чужие ключи и кривые
 * структуры не проходят. */
function toEntityRef(value: unknown): NotificationEntityRef | undefined {
  if (typeof value !== 'object' || value === null) return undefined;
  const { id, name, address, email } = value as Record<string, unknown>;
  if (typeof id !== 'string' || typeof name !== 'string') return undefined;
  return withoutUndefinedSlots({
    id,
    name,
    address: typeof address === 'string' ? address : undefined,
    email: typeof email === 'string' ? email : undefined,
  });
}

function toTariffRef(value: unknown): NotificationTariffRef | undefined {
  if (typeof value !== 'object' || value === null) return undefined;
  const { slug, period, amount_kopecks, active_until } = value as Record<string, unknown>;
  if (typeof slug !== 'string' || typeof period !== 'string' || typeof amount_kopecks !== 'number') {
    return undefined;
  }
  return {
    slug,
    period,
    amountKopecks: amount_kopecks,
    activeUntil: typeof active_until === 'string' ? active_until : null,
  };
}

function toId(value: unknown): string | undefined {
  return typeof value === 'string' ? value : undefined;
}

/** Опциональные поля, не пришедшие с бэка, не остаются явными
 * undefined-ключами — в карточках и действиях работает факт отсутствия. */
function withoutUndefinedSlots<T extends object>(value: T): T {
  return Object.fromEntries(
    Object.entries(value).filter(([, slot]) => slot !== undefined),
  ) as T;
}

function toPayload(raw: Record<string, unknown>): NotificationPayload {
  return withoutUndefinedSlots({
    property: toEntityRef(raw.property),
    actor: toEntityRef(raw.actor),
    rentalId: toId(raw.rental_id),
    paymentId: toId(raw.payment_id),
    taskId: toId(raw.task_id),
    membershipId: toId(raw.membership_id),
    invitationId: toId(raw.invitation_id),
    tariff: toTariffRef(raw.tariff),
  });
}

/** Живые действия читателя — enum закрыт бэком; неизвестное значение
 * (каталог v2 при старом фронте) не рендерится. Источник каталога —
 * NOTIFICATION_ACTION_KINDS (types.ts), расхождение исключено. */
const ACTION_KINDS: ReadonlySet<string> = new Set(NOTIFICATION_ACTION_KINDS);
function toActions(values: NotificationDetailDto['actions']): NotificationActionKind[] {
  return values.filter((value): value is NotificationActionKind => ACTION_KINDS.has(value));
}

export function mapNotificationDetail(dto: NotificationDetailDto): NotificationDetail {
  return {
    ...mapNotification(dto),
    payload: toPayload(dto.payload),
    actions: toActions(dto.actions),
  };
}
