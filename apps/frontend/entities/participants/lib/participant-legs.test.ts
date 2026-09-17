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

  it('suspended — «Превышен лимит объектов», warning с замком (канон агрегат-чипа #697)', () => {
    expect(participantLegBadge(leg({ status: 'suspended' }))).toEqual({
      tone: 'warning',
      label: 'Превышен лимит объектов',
      icon: 'lock',
    });
  });
});
