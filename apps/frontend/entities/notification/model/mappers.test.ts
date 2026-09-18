import { describe, expect, it } from 'vitest';
import type { components } from '@/shared/api/dto';
import { mapNotification } from './mappers';

type NotificationItemDto = components['schemas']['NotificationItem'];

function itemDto(overrides: Partial<NotificationItemDto>): NotificationItemDto {
  return {
    id: '0194a3f8-7c1b-7d21-9a4e-3f2b8c5d6e70',
    event_type: 'rental_completed',
    category: 'rental',
    title: 'Аренда завершена',
    body: 'Договор аренды по объекту «2-комнатная на Ленина» завершён.',
    created_at: '2026-09-17T14:40:00Z',
    ...overrides,
  };
}

describe('mapNotification', () => {
  it('переносит поля и нормализует опциональные к null', () => {
    expect(
      mapNotification(
        itemDto({ context_label: '2-комнатная на Ленина', read_at: '2026-09-17T15:00:00Z' }),
      ),
    ).toStrictEqual({
      id: '0194a3f8-7c1b-7d21-9a4e-3f2b8c5d6e70',
      eventType: 'rental_completed',
      category: 'rental',
      title: 'Аренда завершена',
      body: 'Договор аренды по объекту «2-комнатная на Ленина» завершён.',
      contextLabel: '2-комнатная на Ленина',
      createdAt: '2026-09-17T14:40:00Z',
      readAt: '2026-09-17T15:00:00Z',
    });
  });

  it('непрочитанное без read_at и без контекста', () => {
    const mapped = mapNotification(itemDto({ category: 'system' }));
    expect(mapped.readAt).toBeNull();
    expect(mapped.contextLabel).toBeNull();
    expect(mapped.category).toBe('system');
  });
});
