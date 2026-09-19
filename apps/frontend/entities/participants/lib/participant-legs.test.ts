import { describe, expect, it } from 'vitest';
import type { ParticipantPropertyLeg } from '../model/types';
import { participantLegBadge } from './participant-legs';

function leg(overrides: Partial<ParticipantPropertyLeg> = {}): ParticipantPropertyLeg {
  return {
    propertyId: '33333333-3333-4333-8333-333333333333',
    title: 'Квартира на Ленина',
    role: 'full_access',
    status: 'active',
    ...overrides,
  };
}

describe('participantLegBadge — чип ноги доступа (макеты 2008-81468, 2177-59620)', () => {
  it('active full_access — «Редактирование», серый, иконка Edit (решение чарта: роли — только отображение)', () => {
    expect(participantLegBadge(leg())).toEqual({
      tone: 'neutral',
      label: 'Редактирование',
      icon: 'edit',
    });
  });

  it('active viewer — «Просмотр», серый, иконка Eye', () => {
    expect(participantLegBadge(leg({ role: 'viewer' }))).toEqual({
      tone: 'neutral',
      label: 'Просмотр',
      icon: 'eye',
    });
  });

  it('pending — «Приглашён», серый без иконки (роли у приглашения ещё нет)', () => {
    expect(participantLegBadge(leg({ status: 'pending', role: 'viewer' }))).toEqual({
      tone: 'neutral',
      label: 'Приглашён',
      icon: undefined,
    });
  });

  it('suspended full_access — роль «Редактирование»: статус ноги коммуницируется на уровне участника (макет 2036-84861, правка приёмки #756)', () => {
    expect(participantLegBadge(leg({ status: 'suspended' }))).toEqual({
      tone: 'neutral',
      label: 'Редактирование',
      icon: 'edit',
    });
  });

  it('suspended viewer — роль «Просмотр», серый, иконка Eye', () => {
    expect(participantLegBadge(leg({ status: 'suspended', role: 'viewer' }))).toEqual({
      tone: 'neutral',
      label: 'Просмотр',
      icon: 'eye',
    });
  });
});
