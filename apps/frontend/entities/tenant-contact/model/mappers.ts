import type { components } from '@/shared/api/dto';
import { mapLeaseResponse } from '@/shared/api/mappers/lease';
import type { TenantContact } from './types';

type TenantContactResponse = components['schemas']['TenantContactResponse'];

export function mapTenantContactResponse(dto: TenantContactResponse): TenantContact {
  return {
    id: dto.id,
    ownerId: dto.owner_id,
    name: dto.name,
    surname: dto.surname,
    patronymic: dto.patronymic,
    phone: dto.phone,
    email: dto.email,
    comment: dto.comment,
    isActive: dto.is_active,
    activeLease: dto.active_lease ? mapLeaseResponse(dto.active_lease) : undefined,
    lastLease: dto.last_lease ? mapLeaseResponse(dto.last_lease) : undefined,
    createdAt: dto.created_at,
    updatedAt: dto.updated_at,
  };
}
