import { describe, expect, it } from 'vitest';

import type { HistoryEntry } from '@/entities/history';

import { groupHistoryByDay } from './feed-groups';

const TODAY = '2026-09-22';

function entry(overrides: {
  id: string;
  createdAt: string;
  propertyId?: string;
  propertyName?: string;
  actorId?: string | null;
  actorName?: string;
}): HistoryEntry {
  return {
    propertyId: 'property-1',
    propertyName: 'Моя квартира',
    actorId: 'user-1',
    actorName: 'Иван Иванов',
    actorRole: 'owner',
    baseAction: 'added',
    action: 'payment.created',
    segments: [],
    ...overrides,
  };
}

describe('groupHistoryByDay', () => {
  it('пустая лента — пустой список дней', () => {
    expect(groupHistoryByDay([], TODAY)).toEqual([]);
  });

  it('группирует подряд идущие записи одного дня, чип «Сегодня»', () => {
    const days = groupHistoryByDay(
      [
        entry({ id: 'a', createdAt: '2026-09-22T09:00:00Z' }),
        entry({ id: 'b', createdAt: '2026-09-22T18:00:00Z' }),
      ],
      TODAY,
    );
    expect(days).toHaveLength(1);
    expect(days[0]?.label).toBe('Сегодня');
    expect(days[0]?.day).toBe(TODAY);
    expect(days[0]?.objects[0]?.actors[0]?.actorId).toBe('user-1');
    expect(days[0]?.objects[0]?.actors[0]?.entries.map((e) => e.id)).toEqual(['a', 'b']);
  });

  it('дни чередуются: «Сегодня», «Вчера», дальняя дата «10 сентября»; порядок объектов — первое появление', () => {
    const days = groupHistoryByDay(
      [
        entry({ id: 'old-1', createdAt: '2026-09-10T12:00:00Z' }),
        entry({ id: 'yest-1', createdAt: '2026-09-21T12:00:00Z' }),
        entry({ id: 'today-1', createdAt: '2026-09-22T12:00:00Z' }),
      ],
      TODAY,
    );
    expect(days.map((day) => day.label)).toEqual(['10 сентября', 'Вчера', 'Сегодня']);
  });

  it('дата вне текущего года подписывается годом', () => {
    const days = groupHistoryByDay([entry({ id: 'a', createdAt: '2025-12-30T12:00:00Z' })], TODAY);
    expect(days[0]?.label).toBe('30 декабря, 2025');
  });

  it('внутри дня записи группируются по объектам, внутри объекта — по актёрам; серии идут подряд (канон мессенджера)', () => {
    const days = groupHistoryByDay(
      [
        entry({ id: 'a', createdAt: '2026-09-22T09:00:00Z', propertyId: 'p1', propertyName: 'Квартира' }),
        entry({
          id: 'b',
          createdAt: '2026-09-22T10:00:00Z',
          propertyId: 'p1',
          actorId: 'user-2',
          actorName: 'Анна Сигрейва',
        }),
        entry({ id: 'c', createdAt: '2026-09-22T11:00:00Z', propertyId: 'p1' }),
        entry({ id: 'd', createdAt: '2026-09-22T12:00:00Z', propertyId: 'p2', propertyName: 'Гараж' }),
      ],
      TODAY,
    );
    const objects = days[0]?.objects ?? [];
    expect(objects.map((object_) => object_.propertyId)).toEqual(['p1', 'p2']);
    expect(objects[0]?.propertyName).toBe('Квартира');
    expect(objects[0]?.actors.map((actor) => actor.name)).toEqual(['Иван Иванов', 'Анна Сигрейва', 'Иван Иванов']);
    expect(objects[0]?.actors[0]?.entries.map((e) => e.id)).toEqual(['a']);
    expect(objects[0]?.actors[1]?.entries.map((e) => e.id)).toEqual(['b']);
    expect(objects[0]?.actors[2]?.entries.map((e) => e.id)).toEqual(['c']);
    expect(objects[1]?.actors[0]?.entries.map((e) => e.id)).toEqual(['d']);
  });

  it('обезличенные записи (actor_id null) группируются по снимку имени', () => {
    const days = groupHistoryByDay(
      [
        entry({ id: 'a', createdAt: '2026-09-22T09:00:00Z', actorId: null, actorName: 'Бывший участник' }),
        entry({ id: 'b', createdAt: '2026-09-22T10:00:00Z', actorId: null, actorName: 'Бывший участник' }),
        entry({ id: 'c', createdAt: '2026-09-22T11:00:00Z', actorId: null, actorName: 'Другой удалённый' }),
      ],
      TODAY,
    );
    expect(days[0]?.objects[0]?.actors.map((actor) => actor.name)).toEqual([
      'Бывший участник',
      'Другой удалённый',
    ]);
    expect(days[0]?.objects[0]?.actors.map((actor) => actor.actorId)).toEqual([null, null]);
  });
});
