import { mapLeaseResponse } from '@/shared/api/mappers/lease';
import type { components } from '@/shared/api/dto';
import type { Property } from './types';
import { coerceAttributes } from './attributes';

export function mapPropertyResponse(
  dto: components['schemas']['PropertyResponse'],
): Property {
  return {
    id: dto.id,
    name: dto.name,
    type: dto.type,
    address: dto.address,
    description: dto.description,
    attributes: coerceAttributes(dto.attributes),
    status: dto.status,
    occupancy: dto.occupancy,
    photos: dto.photos?.map((photo) => ({ id: photo.id, url: photo.url })),
    access: dto.access
      ? { role: dto.access.role, ownerName: dto.access.owner_name }
      : undefined,
    activeLease: dto.active_lease ? mapLeaseResponse(dto.active_lease) : null,
    overdue_rent_count: dto.overdue_rent_count,
    members_count: dto.members_count,
  };
}
