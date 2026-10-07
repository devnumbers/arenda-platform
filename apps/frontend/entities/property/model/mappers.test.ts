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
    pinned_at: null,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

describe('mapPropertyResponse photoUrl', () => {
  it('maps photo_url onto the entity (the same-origin streaming path, ADR 0065)', () => {
    const property = mapPropertyResponse(
      makeDto({ photo_url: '/api/v1/properties/property-1/photo' }),
    );

    expect(property.photoUrl).toBe('/api/v1/properties/property-1/photo');
  });

  it('null without a photo', () => {
    const property = mapPropertyResponse(makeDto({ photo_url: null }));

    expect(property.photoUrl).toBeNull();
  });
});

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

describe('mapPropertyResponse list projections (#586)', () => {
  it('maps created_at and pinned_at as-is', () => {
    const property = mapPropertyResponse(
      makeDto({
        created_at: '2025-06-15T10:30:00Z',
        pinned_at: '2026-09-01T08:00:00Z',
      }),
    );

    expect(property.created_at).toBe('2025-06-15T10:30:00Z');
    expect(property.pinned_at).toBe('2026-09-01T08:00:00Z');
  });

  it('keeps pinned_at null for a regular property', () => {
    expect(mapPropertyResponse(makeDto()).pinned_at).toBeNull();
  });
});

describe('mapPropertyResponse occupancy', () => {
  it('maps occupancy and has_overdue_operations', () => {
    const property = mapPropertyResponse(
      makeDto({
        occupancy: {
          status: 'active',
          start_date: '2026-01-01',
          planned_end_date: '2027-03-01',
        },
        has_overdue_operations: true,
      }),
    );

    expect(property.occupancy).toEqual({
      status: 'active',
      start_date: '2026-01-01',
      planned_end_date: '2027-03-01',
    });
    expect(property.has_overdue_operations).toBe(true);
  });

  it('keeps the occupancy dates nullable (open-ended rental, no rental)', () => {
    const openEnded = mapPropertyResponse(
      makeDto({ occupancy: { status: 'active', start_date: '2026-01-01', planned_end_date: null } }),
    );
    expect(openEnded.occupancy?.planned_end_date).toBeNull();

    const none = mapPropertyResponse(
      makeDto({ occupancy: { status: 'none', start_date: null, planned_end_date: null } }),
    );
    expect(none.occupancy?.status).toBe('none');
    expect(none.occupancy?.start_date).toBeNull();
  });

  it('leaves occupancy and has_overdue_operations undefined when not enriched', () => {
    const property = mapPropertyResponse(makeDto());

    expect(property.occupancy).toBeUndefined();
    expect(property.has_overdue_operations).toBeUndefined();
  });
});
