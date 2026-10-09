import { describe, expect, it } from 'vitest';
import type { PropertyAccessMember } from '@/entities/access';
import type { Participant } from '@/entities/participants';
import { resolveParticipantMemberRow } from './participant-member-lookup';

function member(overrides: Partial<PropertyAccessMember> = {}): PropertyAccessMember {
  return {
    id: '99999999-9999-4999-8999-999999999931',
    userId: '12111111-1111-4111-8111-111111111121',
    email: null,
    role: 'full_access',
    isOwner: false,
    displayName: 'Мария Петрова',
    photoUrl: null,
    status: 'active',
    suspendedAt: null,
    lastSentAt: null,
    ...overrides,
  };
}

function participant(overrides: Partial<Participant> = {}): Participant {
  return {
    id: '12111111-1111-4111-8111-111111111121',
    userId: '12111111-1111-4111-8111-111111111121',
    email: 'maria@example.com',
    displayName: 'Мария Петрова',
    aggregateStatus: 'all_properties',
    photoUrl: null,
    accessiblePropertiesCount: 1,
    properties: [],
    ...overrides,
  };
}

describe('resolveParticipantMemberRow — строка доступа участника на объекте (#698)', () => {
  it('зарегистрированный — по user_id, владелец объекта не участник', () => {
    const rows = [
      member({ isOwner: true, userId: '11111111-1111-4111-8111-111111111111' }),
      member(),
      member({ userId: '13111111-1111-4111-8111-111111111131', id: '2' }),
    ];

    expect(resolveParticipantMemberRow(rows, participant())?.id).toBe(
      '99999999-9999-4999-8999-999999999931',
    );
  });

  it('pending-участник (id — почта) — по email среди pending-строк', () => {
    const rows = [
      member({ userId: null, email: 'other@example.com', status: 'pending', id: 'inv-1' }),
      member({ userId: null, email: 'invitee@example.com', status: 'pending', id: 'inv-2' }),
    ];

    expect(
      resolveParticipantMemberRow(
        rows,
        participant({
          id: 'invitee@example.com',
          userId: undefined,
          displayName: undefined,
          email: 'invitee@example.com',
        }),
      )?.id,
    ).toBe('inv-2');
  });

  it('строки нет — undefined (участник уже отозван с объекта)', () => {
    expect(resolveParticipantMemberRow([], participant())).toBeUndefined();
  });
});
