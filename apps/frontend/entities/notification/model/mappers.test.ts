import { describe, expect, it } from 'vitest';
import type { components } from '@/shared/api/dto';
import { mapCategoryPreferences, mapNotification, mapNotificationDetail } from './mappers';

type NotificationItemDto = components['schemas']['NotificationItem'];
type NotificationDetailDto = components['schemas']['NotificationDetailResponse'];

function itemDto(
  overrides: Omit<Partial<NotificationItemDto>, 'payload'> & { payload?: Record<string, unknown> },
): NotificationItemDto {
  const { payload, ...rest } = overrides;
  return {
    id: '0194a3f8-7c1b-7d21-9a4e-3f2b8c5d6e70',
    event_type: 'rental_completed',
    category: 'rental',
    title: 'Аренда завершена',
    body: 'Договор аренды по объекту «2-комнатная на Ленина» завершён.',
    created_at: '2026-09-17T14:40:00Z',
    ...rest,
    // Payload путешествует free-form объектом (контраст сгенерированному
    // Record<string, never>) — тест кормит сырьё, как его отдаёт бэк.
    payload: payload as NotificationItemDto['payload'],
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

  it('переносит payload-снимки в camelCase, включая строки карточек', () => {
    const mapped = mapNotificationDetail(
      detailDto({
        payload: {
          property: {
            id: '0194a3f8-0000-7000-8000-000000000001',
            name: '2-комнатная на Ленина',
            address: 'Проспект Ленина, 31',
          },
          actor: {
            id: '0194a3f8-0000-7000-8000-000000000002',
            name: 'Иван Петров',
            email: 'ivan@yandex.ru',
          },
          rental_id: '0194a3f8-0000-7000-8000-000000000003',
          payment_id: '0194a3f8-0000-7000-8000-000000000004',
          task_id: '0194a3f8-0000-7000-8000-000000000005',
          membership_id: '0194a3f8-0000-7000-8000-000000000006',
          tariff: { slug: 'pro', period: 'month', amount_kopecks: 49000, active_until: '2026-10-01T00:00:00Z' },
        },
      }),
    );
    expect(mapped.payload).toStrictEqual({
      property: { id: '0194a3f8-0000-7000-8000-000000000001', name: '2-комнатная на Ленина', address: 'Проспект Ленина, 31' },
      actor: { id: '0194a3f8-0000-7000-8000-000000000002', name: 'Иван Петров', email: 'ivan@yandex.ru' },
      rentalId: '0194a3f8-0000-7000-8000-000000000003',
      paymentId: '0194a3f8-0000-7000-8000-000000000004',
      taskId: '0194a3f8-0000-7000-8000-000000000005',
      membershipId: '0194a3f8-0000-7000-8000-000000000006',
      tariff: { slug: 'pro', period: 'month', amountKopecks: 49000, activeUntil: '2026-10-01T00:00:00Z' },
    });
  });

  it('строки карточек без адреса и email остаются чистыми снимками {id, name}', () => {
    const mapped = mapNotificationDetail(
      detailDto({
        payload: {
          property: { id: '0194a3f8-0000-7000-8000-000000000001', name: 'Объект' },
          actor: { id: '0194a3f8-0000-7000-8000-000000000002', name: 'Иван Петров' },
        },
      }),
    );
    expect(mapped.payload.property).toStrictEqual({ id: '0194a3f8-0000-7000-8000-000000000001', name: 'Объект' });
    expect(mapped.payload.actor).toStrictEqual({ id: '0194a3f8-0000-7000-8000-000000000002', name: 'Иван Петров' });
  });

  it('type объекта — тот же снимок payload (карта #1217, #1244): строка проходит, не-строка нет', () => {
    const mapped = mapNotificationDetail(
      detailDto({
        payload: {
          property: {
            id: '0194a3f8-0000-7000-8000-000000000001',
            name: 'Гараж',
            type: 'garage',
          },
        },
      }),
    );
    expect(mapped.payload.property).toStrictEqual({
      id: '0194a3f8-0000-7000-8000-000000000001',
      name: 'Гараж',
      type: 'garage',
    });

    const mappedGarbage = mapNotificationDetail(
      detailDto({
        payload: {
          property: {
            id: '0194a3f8-0000-7000-8000-000000000001',
            name: 'Объект',
            type: 42,
          },
        },
      }),
    );
    expect(mappedGarbage.payload.property).toStrictEqual({
      id: '0194a3f8-0000-7000-8000-000000000001',
      name: 'Объект',
    });
  });

  it('photo объекта — снимок пути приватного фото (#1275): строка проходит, не-строка нет', () => {
    const mapped = mapNotificationDetail(
      detailDto({
        payload: {
          property: {
            id: '0194a3f8-0000-7000-8000-000000000001',
            name: 'Гараж',
            type: 'garage',
            photo: '/api/v1/properties/0194a3f8-0000-7000-8000-000000000001/photo',
          },
        },
      }),
    );
    expect(mapped.payload.property?.photo).toBe(
      '/api/v1/properties/0194a3f8-0000-7000-8000-000000000001/photo',
    );

    const mappedGarbage = mapNotificationDetail(
      detailDto({
        payload: {
          property: { id: '0194a3f8-0000-7000-8000-000000000001', name: 'Объект', photo: 42 },
        },
      }),
    );
    expect(mappedGarbage.payload.property).toStrictEqual({
      id: '0194a3f8-0000-7000-8000-000000000001',
      name: 'Объект',
    });
  });

  it('строка ленты несёт payload-снимки (#1275) — лента читает фото объекта без хода на страницу', () => {
    const mapped = mapNotification(
      itemDto({
        payload: {
          property: {
            id: '0194a3f8-0000-7000-8000-000000000001',
            name: 'Гараж',
            photo: '/api/v1/properties/0194a3f8-0000-7000-8000-000000000001/photo',
          },
        },
      }),
    );
    expect(mapped.payload?.property?.photo).toBe(
      '/api/v1/properties/0194a3f8-0000-7000-8000-000000000001/photo',
    );
  });

  it('строка ленты без payload остаётся без ключа — факт отсутствия, не пустой объект', () => {
    const mapped = mapNotification(itemDto({}));
    expect('payload' in mapped).toBe(false);
  });

  it('урезает payload до известного словаря — мусор не проходит', () => {
    const mapped = mapNotificationDetail(
      detailDto({
        payload: {
          property: { id: '0194a3f8-0000-7000-8000-000000000001', name: 'Объект', address: 42 },
          unknown_key: 42,
        },
      }),
    );
    // address не строка — строка карточки не проходит; имя и id остаются.
    expect(mapped.payload.property).toStrictEqual({ id: '0194a3f8-0000-7000-8000-000000000001', name: 'Объект' });
  });

  it('фильтрует неизвестные действия из живого состояния', () => {
    const mapped = mapNotificationDetail(
      detailDto({ actions: ['rental_extend', 'rental_complete', 'open_tariffs_v2'] as NotificationDetailDto['actions'] }),
    );
    expect(mapped.actions).toStrictEqual(['rental_extend', 'rental_complete']);
  });
});

describe('mapCategoryPreferences', () => {
  it('переводит snake_case-матрицу канала в доменные флаги (#746)', () => {
    expect(
      mapCategoryPreferences({
        rental: true,
        payments_operations: false,
        tasks: true,
        shared_access: false,
      }),
    ).toStrictEqual({
      rental: true,
      payments_operations: false,
      tasks: true,
      shared_access: false,
    });
  });
});
