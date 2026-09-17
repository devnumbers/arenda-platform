import { describe, expect, it } from 'vitest';
import {
  toInviteParticipantWireRequest,
  type InviteParticipantCommand,
} from './wire';

describe('toInviteParticipantWireRequest — entity → wire (#694)', () => {
  it('переводит в snake_case: email, роль и порядок идентификаторов', () => {
    const command: InviteParticipantCommand = {
      email: 'maksim@yandex.ru',
      role: 'viewer',
      propertyIds: [
        '33333333-3333-4333-8333-333333333333',
        '44444444-4444-4444-8444-444444444444',
      ],
    };

    expect(toInviteParticipantWireRequest(command)).toStrictEqual({
      email: 'maksim@yandex.ru',
      role: 'viewer',
      property_ids: [
        '33333333-3333-4333-8333-333333333333',
        '44444444-4444-4444-8444-444444444444',
      ],
    });
  });
});
