/**
 * DTO → entity модели слайса уведомлений (#743, карта #734). Контракт
 * snake_case: event_type, context_label, read_at; опциональные поля
 * нормализуются к null.
 */

import type { components } from '@/shared/api/dto';
import type { Notification, NotificationCategory } from './types';

type NotificationItemDto = components['schemas']['NotificationItem'];

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
