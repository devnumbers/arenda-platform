import { describe, expect, it } from 'vitest';
import type { Participant, ParticipantPropertyLeg } from '../model/types';
import { availableInviteProperties } from './participant-invite-options';

const APARTMENT = '33333333-3333-4333-8333-333333333333';
const GARAGE = '44444444-4444-4444-8444-444444444444';
const STUDIO = '46464646-4646-4646-8646-464646464646';

function property(id: string, name: string) {
  return { id, name, address: 'Адрес', type: 'apartment' as const };
}

function participant(properties: ReadonlyArray<ParticipantPropertyLeg>): Participant {
  return {
    id: '12111111-1111-4111-8111-111111111121',
    userId: '12111111-1111-4111-8111-111111111121',
    email: 'maria@example.com',
    displayName: 'Мария Петрова',
    aggregateStatus: 'partial',
    photoUrl: null,
    accessiblePropertiesCount: properties.length,
    properties,
  };
}

function leg(propertyId: string, overrides: Partial<ParticipantPropertyLeg> = {}): ParticipantPropertyLeg {
  return { propertyId, title: 'Объект', role: 'viewer', status: 'active', type: 'apartment', photoUrl: null, ...overrides };
}

describe('availableInviteProperties — объекты для «Пригласить в объект» (макет 2010-131329)', () => {
  it('показывает объекты читающего, к которым у участника ещё нет доступа', () => {
    const result = availableInviteProperties(
      [property(APARTMENT, 'Квартира на Ленина'), property(GARAGE, 'Гараж на Садовой')],
      participant([leg(APARTMENT)]),
    );

    expect(result.map((p) => p.id)).toEqual([GARAGE]);
    // Тип travels в опцию — ключ глифа-плейсхолдера аватара (#1244).
    expect(result[0]?.type).toBe('apartment');
  });

  it('pending-нога тоже исключает объект — повторное приглашение даёт skipped_duplicate', () => {
    const result = availableInviteProperties(
      [property(APARTMENT, 'Квартира'), property(STUDIO, 'Студия')],
      participant([leg(APARTMENT, { status: 'pending' })]),
    );

    expect(result.map((p) => p.id)).toEqual([STUDIO]);
  });

  it('suspended-нога исключает объект — доступ уже есть, его не выдаёт повторно', () => {
    const result = availableInviteProperties(
      [property(GARAGE, 'Гараж')],
      participant([leg(GARAGE, { status: 'suspended' })]),
    );

    expect(result).toEqual([]);
  });

  it('без ног все объекты доступны для приглашения', () => {
    const result = availableInviteProperties(
      [property(APARTMENT, 'Квартира'), property(GARAGE, 'Гараж')],
      participant([]),
    );

    expect(result).toHaveLength(2);
  });
});
