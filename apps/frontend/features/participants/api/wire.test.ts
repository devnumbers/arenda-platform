import { describe, expect, it } from 'vitest';
import {
  toAddParticipantPropertiesWireRequest,
  type AddParticipantPropertiesCommand,
} from './wire';

describe('toAddParticipantPropertiesWireRequest — entity → wire (#694)', () => {
  it('переводит в snake_case и сохраняет порядок идентификаторов', () => {
    const command: AddParticipantPropertiesCommand = {
      role: 'viewer',
      propertyIds: [
        '44444444-4444-4444-8444-444444444444',
        '33333333-3333-4333-8333-333333333333',
      ],
    };

    expect(toAddParticipantPropertiesWireRequest(command)).toStrictEqual({
      role: 'viewer',
      property_ids: [
        '44444444-4444-4444-8444-444444444444',
        '33333333-3333-4333-8333-333333333333',
      ],
    });
  });

  it('сохраняет пустой список — гейт непустого выбора на экране', () => {
    expect(
      toAddParticipantPropertiesWireRequest({
        role: 'full_access',
        propertyIds: [],
      }),
    ).toStrictEqual({ role: 'full_access', property_ids: [] });
  });
});
