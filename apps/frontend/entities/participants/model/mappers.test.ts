import { describe, expect, it } from 'vitest';
import type { components } from '@/shared/api/dto';
import { mapParticipant } from './mappers';

type ParticipantDto = components['schemas']['ParticipantResponse'];

const registeredDto: ParticipantDto = {
  id: '12111111-1111-4111-8111-111111111121',
  user_id: '12111111-1111-4111-8111-111111111121',
  email: 'e2e-member@example.com',
  display_name: 'Мария Петрова',
  photo_url: '/api/v1/users/12111111-1111-4111-8111-111111111121/photo',
  aggregate_status: 'all_properties',
  accessible_properties_count: 1,
  properties: [
    {
      property_id: '33333333-3333-4333-8333-333333333333',
      title: 'Квартиры на Ленина',
      role: 'full_access',
      status: 'active',
      type: 'apartment',
      photo_url: '/api/v1/properties/33333333-3333-4333-8333-333333333333/photo',
    },
  ],
};

describe('mapParticipant — DTO → entity', () => {
  it('зарегистрированный участник переносится 1:1 (snake → camel)', () => {
    const participant = mapParticipant(registeredDto);

    expect(participant.id).toBe(registeredDto.id);
    expect(participant.userId).toBe('12111111-1111-4111-8111-111111111121');
    expect(participant.email).toBe('e2e-member@example.com');
    expect(participant.displayName).toBe('Мария Петрова');
    expect(participant.photoUrl).toBe(
      '/api/v1/users/12111111-1111-4111-8111-111111111121/photo',
    );
    expect(participant.properties[0]).toMatchObject({
      type: 'apartment',
      photoUrl: '/api/v1/properties/33333333-3333-4333-8333-333333333333/photo',
    });
    expect(participant.aggregateStatus).toBe('all_properties');
    expect(participant.accessiblePropertiesCount).toBe(1);
    expect(participant.properties).toHaveLength(1);
    expect(participant.properties[0]).toEqual({
      propertyId: '33333333-3333-4333-8333-333333333333',
      title: 'Квартиры на Ленина',
      role: 'full_access',
      status: 'active',
      type: 'apartment',
      photoUrl: '/api/v1/properties/33333333-3333-4333-8333-333333333333/photo',
    });
  });

  it('pending-строка (wire null): ни юзера, ни имени — лейбл строки почта', () => {
    const pending = mapParticipant({
      id: 'invitee@example.com',
      user_id: null,
      email: 'invitee@example.com',
      display_name: null,
      aggregate_status: 'partial',
      accessible_properties_count: 2,
      properties: [],
    });

    expect(pending.userId).toBeUndefined();
    expect(pending.displayName).toBeUndefined();
    expect(pending.email).toBe('invitee@example.com');
  });

  it('зарегистрированный без почты — email undefined', () => {
    const noEmail = mapParticipant({ ...registeredDto, email: null });

    expect(noEmail.email).toBeUndefined();
  });
});
