import { describe, expect, it } from 'vitest';
import type { components } from '@/shared/api/dto';
import { mapNotification, mapNotificationDetail } from './mappers';

type NotificationItemDto = components['schemas']['NotificationItem'];
type NotificationDetailDto = components['schemas']['NotificationDetailResponse'];

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

function detailDto(
  overrides: Omit<Partial<NotificationDetailDto>, 'payload'> & { payload?: Record<string, unknown> },
): NotificationDetailDto {
  const { payload, ...rest } = overrides;
  return {
    id: '0194a3f8-7c1b-7d21-9a4e-3f2b8c5d6e70',
    event_type: 'rental_completed',
    category: 'rental',
    title: 'Аренда завершена',
    body: 'Договор аренды по объекту «2-комнатная на Ленина» завершён.',
    actions: [],
    created_at: '2026-09-17T14:40:00Z',
    ...rest,
    // Payload путешествует free-form объектом (контраст сгенерированному
    // Record<string, never>) — тест кормит сырьё, как его отдаёт бэк.
    payload: (payload ?? {}) as NotificationDetailDto['payload'],
  };
}

describe('mapNotificationDetail', () => {
  it('переносит поля строки и нормализует опциональные к null', () => {
    const mapped = mapNotificationDetail(
      detailDto({
        context_label: '2-комнатная на Ленина',
        read_at: null,
      }),
    );
    expect(mapped.id).toBe('0194a3f8-7c1b-7d21-9a4e-3f2b8c5d6e70');
    expect(mapped.eventType).toBe('rental_completed');
    expect(mapped.category).toBe('rental');
    expect(mapped.contextLabel).toBe('2-комнатная на Ленина');
    expect(mapped.readAt).toBeNull();
    expect(mapped.createdAt).toBe('2026-09-17T14:40:00Z');
  });

  it('переносит payload-снимки в camelCase', () => {
    const mapped = mapNotificationDetail(
      detailDto({
        payload: {
          property: { id: '0194a3f8-0000-7000-8000-000000000001', name: '2-комнатная на Ленина' },
          actor: { id: '0194a3f8-0000-7000-8000-000000000002', name: 'Иван Петров' },
          rental_id: '0194a3f8-0000-7000-8000-000000000003',
          payment_id: '0194a3f8-0000-7000-8000-000000000004',
          task_id: '0194a3f8-0000-7000-8000-000000000005',
          membership_id: '0194a3f8-0000-7000-8000-000000000006',
          invitation_id: '0194a3f8-0000-7000-8000-000000000007',
          tariff: { slug: 'pro', period: 'month', amount_kopecks: 49000, active_until: '2026-10-01T00:00:00Z' },
        },
      }),
    );
    expect(mapped.payload).toStrictEqual({
      property: { id: '0194a3f8-0000-7000-8000-000000000001', name: '2-комнатная на Ленина' },
      actor: { id: '0194a3f8-0000-7000-8000-000000000002', name: 'Иван Петров' },
      rentalId: '0194a3f8-0000-7000-8000-000000000003',
      paymentId: '0194a3f8-0000-7000-8000-000000000004',
      taskId: '0194a3f8-0000-7000-8000-000000000005',
      membershipId: '0194a3f8-0000-7000-8000-000000000006',
      invitationId: '0194a3f8-0000-7000-8000-000000000007',
      tariff: { slug: 'pro', period: 'month', amountKopecks: 49000, activeUntil: '2026-10-01T00:00:00Z' },
    });
  });

  it('урезает payload до известного словаря — мусор не проходит', () => {
    const mapped = mapNotificationDetail(
      detailDto({
        payload: {
          property: { id: '0194a3f8-0000-7000-8000-000000000001', name: 'Объект', address: 'неизвестное поле' },
          unknown_key: 42,
        },
      }),
    );
    expect(mapped.payload).toStrictEqual({
      property: { id: '0194a3f8-0000-7000-8000-000000000001', name: 'Объект' },
    });
  });

  it('фильтрует неизвестные действия из живого состояния', () => {
    const mapped = mapNotificationDetail(
      detailDto({ actions: ['rental_extend', 'rental_complete', 'open_tariffs_v2'] as NotificationDetailDto['actions'] }),
    );
    expect(mapped.actions).toStrictEqual(['rental_extend', 'rental_complete']);
  });
});
