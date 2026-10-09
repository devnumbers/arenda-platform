import { describe, expect, it } from 'vitest';
import type { Property } from '@/entities/property';
import {
  parseParticipantsPropertyOrderParams,
  serializeParticipantsPropertyOrderToParams,
  sortUserPropertyRows,
  userPropertyRows,
  type UserPropertyRow,
} from './participants-properties-list';

function makeProperty(overrides: Partial<Property> & Pick<Property, 'id' | 'name'>): Property {
  return {
    type: 'apartment',
    address: 'ул. Ленина, 1',
    attributes: {},
    status: 'active',
    members_count: 0,
    created_at: '2026-09-01T10:00:00Z',
    pinned_at: null,
    ...overrides,
  };
}

describe('userPropertyRows', () => {
  it('оставляет только чужие объекты — свои (owner) и строки без access уходят', () => {
    const rows = userPropertyRows([
      makeProperty({ id: 'own', name: 'Моя квартира', access: { role: 'owner' } }),
      makeProperty({ id: 'shared-edit', name: 'Кофейня', access: { role: 'full_access' } }),
      makeProperty({ id: 'shared-view', name: 'Склад', access: { role: 'viewer' } }),
      makeProperty({ id: 'no-access', name: 'Без контекста' }),
    ]);
    expect(rows.map((row) => row.id)).toEqual(['shared-edit', 'shared-view']);
  });

  it('ряд несёт титул, адрес, фото, тип, роль и бейдж: full_access → «Редактирование»', () => {
    const rows = userPropertyRows([
      makeProperty({
        id: 'p1',
        name: 'Кофейня',
        address: 'ул. Мира, 15',
        photoUrl: '/photo.jpg',
        type: 'commercial',
        access: { role: 'full_access' },
      }),
    ]);
    expect(rows[0]).toMatchObject({
      id: 'p1',
      title: 'Кофейня',
      address: 'ул. Мира, 15',
      photoUrl: '/photo.jpg',
      // Тип travels в ряд — ключ глифа-плейсхолдера аватара (#1244).
      type: 'commercial',
      role: 'full_access',
      badge: { tone: 'neutral', label: 'Редактирование', icon: 'edit' },
    });
  });

  it('viewer → роль viewer и бейдж «Просмотр» с глазом', () => {
    const rows = userPropertyRows([
      makeProperty({ id: 'p2', name: 'Склад', access: { role: 'viewer' } }),
    ]);
    expect(rows[0]).toMatchObject({
      role: 'viewer',
      badge: { tone: 'neutral', label: 'Просмотр', icon: 'eye' },
    });
  });
});

describe('sortUserPropertyRows', () => {
  const rows: UserPropertyRow[] = (
    [
      ['b', 'Гараж на Садовой'],
      ['a', 'Квартира на Ленина'],
      ['c', 'Апартаменты у моря'],
    ] as const
  ).map(([id, name]) => ({
    id,
    title: name,
    address: '',
    photoUrl: undefined,
    type: 'apartment' as const,
    role: 'viewer',
    badge: { tone: 'neutral', label: 'Просмотр', icon: 'eye' },
  }));

  it('asc — русская коллация по названию', () => {
    expect(sortUserPropertyRows(rows, 'asc').map((row) => row.id)).toEqual(['c', 'b', 'a']);
  });

  it('desc — обратный порядок, вход не мутируется', () => {
    sortUserPropertyRows(rows, 'desc');
    expect(rows.map((row) => row.id)).toEqual(['b', 'a', 'c']);
    expect(sortUserPropertyRows(rows, 'desc').map((row) => row.id)).toEqual(['a', 'b', 'c']);
  });
});

describe('parseParticipantsPropertyOrderParams — разбор ?order= «Объектов пользователей» (#785)', () => {
  it('отсутствие, пустое, неизвестное и массивное — дефолт «А→Я»', () => {
    expect(parseParticipantsPropertyOrderParams(undefined)).toBe('asc');
    expect(parseParticipantsPropertyOrderParams('')).toBe('asc');
    expect(parseParticipantsPropertyOrderParams('по дате')).toBe('asc');
    expect(parseParticipantsPropertyOrderParams(['desc'])).toBe('asc');
  });

  it('читает «Я→А»', () => {
    expect(parseParticipantsPropertyOrderParams('desc')).toBe('desc');
  });
});

describe('serializeParticipantsPropertyOrderToParams — патч ?order= для адреса (#785)', () => {
  it('дефолт «А→Я» параметров не создаёт — пустой query даёт голый адрес', () => {
    expect(serializeParticipantsPropertyOrderToParams('asc')).toStrictEqual({});
  });

  it('«Я→А» пишет order=desc', () => {
    expect(serializeParticipantsPropertyOrderToParams('desc')).toStrictEqual({ order: 'desc' });
  });
});
