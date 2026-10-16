import { describe, expect, it } from 'vitest';
import type { Notification } from '@/entities/notification';
import { groupNotificationsByDay } from './feed-groups';

function notification(overrides: Partial<Notification>): Notification {
  return {
    id: 'n1',
    eventType: 'rental_completed',
    category: 'rental',
    title: 'Аренда завершена',
    body: 'Договор аренды завершён.',
    contextLabel: '2-комнатная на Ленина',
    createdAt: '2026-09-18T10:00:00',
    readAt: null,
    ...overrides,
  };
}

const TODAY = '2026-09-18';

describe('groupNotificationsByDay', () => {
  it('«Сегодня» и «Вчера» — клиентский день смотрящего, время показывается', () => {
    const groups = groupNotificationsByDay(
      [
        notification({ id: 'a', createdAt: '2026-09-18T14:40:00' }),
        notification({ id: 'b', createdAt: '2026-09-17T09:15:00' }),
      ],
      TODAY,
    );
    expect(groups.map((group) => [group.label, group.showTime])).toStrictEqual([
      ['Сегодня', true],
      ['Вчера', true],
    ]);
  });

  it('дни старше вчера — «день месяц» с годом вне текущего, время скрыто', () => {
    const groups = groupNotificationsByDay(
      [
        notification({ id: 'a', createdAt: '2026-08-10T14:40:00' }),
        notification({ id: 'b', createdAt: '2025-12-30T14:40:00' }),
        notification({ id: 'c', createdAt: '2024-09-01T14:40:00' }),
      ],
      TODAY,
    );
    expect(groups.map((group) => [group.label, group.showTime])).toStrictEqual([
      ['10 августа', false],
      ['30 декабря, 2025', false],
      ['1 сентября, 2024', false],
    ]);
  });

  it('подряд идущие уведомления одного дня — одна группа (лента newest-first)', () => {
    const groups = groupNotificationsByDay(
      [
        notification({ id: 'a', createdAt: '2026-09-18T20:00:00' }),
        notification({ id: 'b', createdAt: '2026-09-18T08:00:00' }),
        notification({ id: 'c', createdAt: '2026-09-18T07:00:00' }),
        notification({ id: 'd', createdAt: '2026-09-17T12:00:00' }),
      ],
      TODAY,
    );
    expect(groups).toHaveLength(2);
    expect(groups[0]?.notifications.map((item) => item.id)).toStrictEqual(['a', 'b', 'c']);
    expect(groups[1]?.notifications.map((item) => item.id)).toStrictEqual(['d']);
  });

  it('пустая лента — без групп', () => {
    expect(groupNotificationsByDay([], TODAY)).toStrictEqual([]);
  });
});
