import type { components } from '@/shared/api/generated';
import type { Property } from './types';

export function mapPropertyResponse(
  dto: components['schemas']['PropertyResponse'],
): Property {
  return {
    id: dto.id,
    name: dto.name,
    type: dto.type,
    address: dto.address,
    description: dto.description,
    status: dto.status,
    occupancy: dto.occupancy,
  };
}
