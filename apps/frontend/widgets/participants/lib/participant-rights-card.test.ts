import { describe, expect, it } from 'vitest';
import type { PropertyAccessMember } from '@/entities/access';
import type { Participant } from '@/entities/participants';
import { pendingMemberRightsCard, pendingParticipantRightsCard } from './participant-rights-card';

const pendingParticipant: Participant = {
  id: 'maksim%40yandex.ru',
  userId: undefined,
  email: 'maksim@yandex.ru',
  displayName: undefined,
  aggregateStatus: 'all_properties',
  photoUrl: null,
  photoBlur: null,
  accessiblePropertiesCount: 1,
  properties: [
    {
      propertyId: '33333333-3333-4333-8333-333333333333',
      title: 'Квартира на Ленина',
      role: 'viewer',
      status: 'pending',
      type: 'apartment',
      photoUrl: null,
      photoBlur: null,
    },
  ],
};

const registeredParticipant: Participant = {
  ...pendingParticipant,
  id: '12111111-1111-4111-8111-111111111121',
  userId: '12111111-1111-4111-8111-111111111121',
  email: 'maria@example.com',
  displayName: 'Мария Петрова',
};

function memberOf(
  overrides: Partial<PropertyAccessMember>,
): PropertyAccessMember {
  return {
    id: '99999999-9999-4999-8999-999999999971',
    userId: null,
    email: 'maksim@yandex.ru',
    displayName: '',
    photoUrl: null,
    photoBlur: null,
    role: 'viewer',
    status: 'pending',
    isOwner: false,
    ...overrides,
  };
}

describe('pendingParticipantRightsCard', () => {
  it('pending-участник с pending-строкой — карточка D7: анонимный титул, почта, иконка роли, бейдж', () => {
    expect(pendingParticipantRightsCard(pendingParticipant, memberOf({ role: 'viewer' }))).toStrictEqual({
      title: 'Пользователь',
      email: 'maksim@yandex.ru',
      emailIcon: 'eye',
      badge: { tone: 'info', label: 'Аккаунт не создан', icon: 'lock' },
      photoUrl: undefined,
      photoBlur: undefined,
    });
  });

  it('pending-строка с ролью full_access — иконка карандаша («Управление», #1389)', () => {
    const card = pendingParticipantRightsCard(
      pendingParticipant,
      memberOf({ role: 'full_access' }),
    );

    expect(card?.emailIcon).toBe('edit');
  });

  it('зарегистрированный участник — не D7-кейс (макет 2177-59620 остаётся)', () => {
    expect(
      pendingParticipantRightsCard(
        registeredParticipant,
        memberOf({ userId: registeredParticipant.userId ?? null, status: 'active' }),
      ),
    ).toBeUndefined();
  });

  it('строка не найдена (доступ уже снят) — не D7-кейс: честный 404 выше', () => {
    expect(
      pendingParticipantRightsCard(pendingParticipant, undefined),
    ).toBeUndefined();
  });

  it('pending-участник с активной строкой — не D7-кейс (ветка по статусу строки)', () => {
    expect(
      pendingParticipantRightsCard(
        pendingParticipant,
        memberOf({ status: 'active' }),
      ),
    ).toBeUndefined();
  });
});

describe('pendingMemberRightsCard — карточка pending в member-режиме прав (#1362)', () => {
  it('pending-строка — та же анатомия карточки D7: анонимный титул, почта, иконка роли, бейдж; фото у pending не бывает', () => {
    expect(pendingMemberRightsCard(memberOf({ role: 'viewer' }))).toStrictEqual({
      title: 'Пользователь',
      email: 'maksim@yandex.ru',
      emailIcon: 'eye',
      badge: { tone: 'info', label: 'Аккаунт не создан', icon: 'lock' },
      photoUrl: undefined,
      photoBlur: undefined,
    });
  });

  it('pending-строка с ролью full_access — иконка карандаша («Управление», #1389)', () => {
    expect(pendingMemberRightsCard(memberOf({ role: 'full_access' }))?.emailIcon).toBe('edit');
  });

  it('не-pending строка — не карточка (зарегистрированный остаётся на макете объекта)', () => {
    expect(
      pendingMemberRightsCard(
        memberOf({ status: 'active', userId: '12111111-1111-4111-8111-111111111121' }),
      ),
    ).toBeUndefined();
  });
});
