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
    photos: dto.photos?.map((photo) => ({ id: photo.id, url: photo.url })),
    access: dto.access
      ? { role: dto.access.role, ownerName: dto.access.owner_name }
      : undefined,
    members_count: dto.members_count,
  };
}
