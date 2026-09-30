import { describe, expect, it } from 'vitest';
import type { Property } from '@/entities/property';
import { resolvePropertiesNavItem } from './properties-nav-item';

function makeProperty(overrides: Partial<Property> & { id: string }): Property {
  return {
    name: 'Квартира',
    type: 'apartment',
    address: 'Москва, ул. Ленина, 1',
    attributes: {},
    status: 'active',
    members_count: 0,
    created_at: '2026-01-01T00:00:00Z',
    pinned_at: null,
    ...overrides,
  };
}

describe('resolvePropertiesNavItem (пункт «Объекты» хрома, карта #984)', () => {
  it('данные списка не загружены — «Объекты» на список', () => {
    expect(resolvePropertiesNavItem('basic', undefined, undefined)).toStrictEqual({
      label: 'Объекты',
      href: '/properties',
    });
  });

  it('список загружен, архив ещё нет — «Объекты» (про архив неизвестно)', () => {
    expect(
      resolvePropertiesNavItem('basic', [makeProperty({ id: 'only', access: { role: 'owner' } })], undefined),
    ).toStrictEqual({ label: 'Объекты', href: '/properties' });
  });

  it('пустая книга и пустой архив — «Объекты» на список', () => {
    expect(resolvePropertiesNavItem('basic', [], [])).toStrictEqual({
      label: 'Объекты',
      href: '/properties',
    });
  });

  it('basic и ровно один активный свой без архива — «Объект» на его страницу', () => {
    expect(
      resolvePropertiesNavItem('basic', [makeProperty({ id: 'only', access: { role: 'owner' } })], []),
    ).toStrictEqual({ label: 'Объект', href: '/properties/only' });
  });

  it('basic и единственный в ремонте — тоже «Объект» на его страницу', () => {
    expect(
      resolvePropertiesNavItem(
        'basic',
        [makeProperty({ id: 'only', status: 'maintenance', access: { role: 'owner' } })],
        [],
      ),
    ).toStrictEqual({ label: 'Объект', href: '/properties/only' });
  });

  it('basic и один активный свой плюс архивный — «Объекты» (архивные считаются)', () => {
    const active = makeProperty({ id: 'only', access: { role: 'owner' } });
    const archived = makeProperty({ id: 'old', status: 'archived', access: { role: 'owner' } });
    expect(resolvePropertiesNavItem('basic', [active], [archived])).toStrictEqual({
      label: 'Объекты',
      href: '/properties',
    });
  });

  it('basic и один активный свой плюс чужой активный — «Объекты» (поделились)', () => {
    const properties = [
      makeProperty({ id: 'own', access: { role: 'owner' } }),
      makeProperty({ id: 'shared', access: { role: 'full_access', ownerName: 'Иван Иванов' } }),
    ];
    expect(resolvePropertiesNavItem('basic', properties, [])).toStrictEqual({
      label: 'Объекты',
      href: '/properties',
    });
  });

  it('basic и единственный объект — чужой — «Объекты»', () => {
    expect(
      resolvePropertiesNavItem(
        'basic',
        [makeProperty({ id: 'shared', access: { role: 'viewer', ownerName: 'Иван Иванов' } })],
        [],
      ),
    ).toStrictEqual({ label: 'Объекты', href: '/properties' });
  });

  it('basic и единственный объект без access — «Объекты» (чей — неизвестно)', () => {
    expect(resolvePropertiesNavItem('basic', [makeProperty({ id: 'only' })], [])).toStrictEqual({
      label: 'Объекты',
      href: '/properties',
    });
  });

  it('basic и только архивный — «Объекты» на список', () => {
    expect(
      resolvePropertiesNavItem('basic', [], [makeProperty({ id: 'old', status: 'archived' })]),
    ).toStrictEqual({ label: 'Объекты', href: '/properties' });
  });

  it('basic и два активных своих — «Объекты» на список', () => {
    const properties = [
      makeProperty({ id: 'a', access: { role: 'owner' } }),
      makeProperty({ id: 'b', access: { role: 'owner' } }),
    ];
    expect(resolvePropertiesNavItem('basic', properties, [])).toStrictEqual({
      label: 'Объекты',
      href: '/properties',
    });
  });

  it('платный тариф — всегда «Объекты», закрепление умерло (карта #984)', () => {
    const pinned = [makeProperty({ id: 'pinned', pinned_at: '2026-09-01T00:00:00Z', access: { role: 'owner' } })];
    expect(resolvePropertiesNavItem('pro', pinned, [])).toStrictEqual({
      label: 'Объекты',
      href: '/properties',
    });
    expect(resolvePropertiesNavItem('business', pinned, [])).toStrictEqual({
      label: 'Объекты',
      href: '/properties',
    });
  });

  it('тариф неизвестен (me не загружен) — «Объекты» на список', () => {
    expect(
      resolvePropertiesNavItem(undefined, [makeProperty({ id: 'only', access: { role: 'owner' } })], []),
    ).toStrictEqual({ label: 'Объекты', href: '/properties' });
  });
});
