import { describe, expect, it } from 'vitest';
import type { Property } from '@/entities/property';
import { cardOwnerName } from './property-owner-name';

// Ряд владельца на карточке объекта (решение владельца по итогам обхода
// #756): ряд носят только ЧУЖИЕ объекты — карточка показывает, чей это
// объект и кто выдал доступ. Свои объекты ряда не ведут, как и строки без
// контекста доступа или без обогащения (write-пути, stale-кэш).

const base = {
  id: '33333333-3333-4333-8333-333333333333',
  name: 'Квартира на Ленина',
  type: 'apartment' as const,
  address: 'Москва, ул. Ленина, 1',
  attributes: {},
  status: 'active' as const,
  members_count: 2,
  created_at: '2026-09-19T00:00:00Z',
  pinned_at: null,
};

describe('cardOwnerName', () => {
  it('чужой объект с обогащением — имя владельца', () => {
    const property: Property = {
      ...base,
      access: { role: 'viewer', ownerName: 'Иван Иванов' },
    };
    expect(cardOwnerName(property)).toBe('Иван Иванов');
  });

  it('владелец безымянный — «Пользователь» проходит как есть (карта #1105, аменд #1123, #1109)', () => {
    // Канон строит бекенд (Имя Фамилия, иначе «Пользователь» — телефон
    // больше не фолбэк); фронт не
    // пере-маскирует — ряд карточки несёт строку шва как есть.
    const property: Property = {
      ...base,
      access: { role: 'viewer', ownerName: 'Пользователь' },
    };
    expect(cardOwnerName(property)).toBe('Пользователь');
  });

  it('чужой объект без обогащения — ряда нет', () => {
    const property: Property = {
      ...base,
      access: { role: 'full_access', ownerName: undefined },
    };
    expect(cardOwnerName(property)).toBeNull();
  });

  it('свой объект — ряда нет: владелец сам читающий', () => {
    const property: Property = {
      ...base,
      access: { role: 'owner', ownerName: 'утечка' },
    };
    expect(cardOwnerName(property)).toBeNull();
  });

  it('строка без контекста доступа — ряда нет', () => {
    const property: Property = { ...base };
    expect(cardOwnerName(property)).toBeNull();
  });
});
