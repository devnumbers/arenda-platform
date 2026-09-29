import type { components } from '@/shared/api/dto';
import type { Property, PropertyPhoto } from './types';
import { coerceAttributes } from './attributes';

/** Фото объекта (wire-схема уже канон): переносим один в один, чтобы
 * потребители зависели от entity, а не от сгенерированного DTO. */
export function mapPropertyPhoto(
  dto: components['schemas']['PropertyPhoto'],
): PropertyPhoto {
  return { id: dto.id, url: dto.url };
}

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
    photos: dto.photos?.map(mapPropertyPhoto),
    access: dto.access
      ? {
          role: dto.access.role,
          ownerName: dto.access.owner_name,
          ownerEmail: dto.access.owner_email,
        }
      : undefined,
    members_count: dto.members_count,
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
