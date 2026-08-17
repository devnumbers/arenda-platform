import type { components } from '@/shared/api/dto';
import type { PropertyContact } from './types';

type PropertyContactResponse = components['schemas']['PropertyContactResponse'];

export function mapPropertyContactResponse(
  dto: PropertyContactResponse,
): PropertyContact {
  return {
    id: dto.id,
    propertyId: dto.property_id,
    name: dto.name,
    phone: dto.phone,
    createdAt: dto.created_at,
    updatedAt: dto.updated_at,
  };
}
