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
    member_names: dto.member_names,
    created_at: dto.created_at,
    pinned_at: dto.pinned_at,
    occupancy: dto.occupancy
      ? {
          status: dto.occupancy.status,
          start_date: dto.occupancy.start_date,
          planned_end_date: dto.occupancy.planned_end_date,
        }
      : undefined,
    has_overdue_operations: dto.has_overdue_operations,
  };
}
