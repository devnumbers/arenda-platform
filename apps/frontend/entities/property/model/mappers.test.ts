import { describe, expect, it } from 'vitest';
import type { components } from '@/shared/api/dto';
import { mapPropertyResponse } from './mappers';

function makeDto(
  overrides: Partial<components['schemas']['PropertyResponse']> = {},
): components['schemas']['PropertyResponse'] {
  return {
    id: 'property-1',
    name: 'Квартира на Ленина',
    type: 'apartment',
    address: 'Москва, ул. Ленина, 1',
    attributes: {},
    status: 'active',
    members_count: 0,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

describe('mapPropertyResponse access', () => {
  it('maps access role and owner_name to camelCase ownerName', () => {
    const property = mapPropertyResponse(
      makeDto({ access: { role: 'viewer', owner_name: 'Иван Петров' } }),
    );

    expect(property.access).toEqual({ role: 'viewer', ownerName: 'Иван Петров' });
  });

  it('maps access without owner_name', () => {
    const property = mapPropertyResponse(makeDto({ access: { role: 'owner' } }));

    expect(property.access).toEqual({ role: 'owner', ownerName: undefined });
  });

  it('leaves access undefined when the DTO has no access', () => {
    const property = mapPropertyResponse(makeDto());

    expect(property.access).toBeUndefined();
  });
});
