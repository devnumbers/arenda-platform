import { mapLeaseResponse } from '@/entities/lease/model/mappers';
import type { components } from '@/shared/api/generated';
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
    activeLease: dto.active_lease ? mapLeaseResponse(dto.active_lease) : null,
    overdue_rent_count: dto.overdue_rent_count,
  };
}
