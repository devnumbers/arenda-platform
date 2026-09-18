import { describe, expect, it } from 'vitest';
import type { Property } from '@/entities/property';
import { cardParticipantNames } from './property-participants';

function makeProperty(
  overrides: Partial<Property> & Pick<Property, 'id' | 'name'>,
): Property {
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

describe('cardParticipantNames', () => {
  it('свой объект — имена активных участников в порядке списка', () => {
    const property = makeProperty({
      id: 'own',
      name: 'Квартира',
      access: { role: 'owner' },
      member_names: ['Мария Иванова', 'Пётр Селезнёв'],
    });
    expect(cardParticipantNames(property)).toEqual([
      'Мария Иванова',
      'Пётр Селезнёв',
    ]);
  });

  it('объект без access (деталь владельца) — ряд участников рисуется', () => {
    const property = makeProperty({
      id: 'own',
      name: 'Квартира',
      member_names: ['Мария Иванова'],
    });
    expect(cardParticipantNames(property)).toEqual(['Мария Иванова']);
  });

  it('чужой объект — ряд не носят ни при какой роли', () => {
    const fullAccess = makeProperty({
      id: 'shared',
      name: 'Чужая',
      access: { role: 'full_access', ownerName: 'Мария Иванова' },
      member_names: ['утечка'],
    });
    const viewer = makeProperty({
      id: 'shared',
      name: 'Чужая',
      access: { role: 'viewer' },
      member_names: ['утечка'],
    });
    expect(cardParticipantNames(fullAccess)).toEqual([]);
    expect(cardParticipantNames(viewer)).toEqual([]);
  });

  it('без участников ряд пуст — блок не рисуется', () => {
    expect(
      cardParticipantNames(makeProperty({ id: 'own', name: 'Квартира' })),
    ).toEqual([]);
  });
});
