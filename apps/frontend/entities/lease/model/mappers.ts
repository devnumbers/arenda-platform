import type { components } from '@/shared/api/generated';
import type { Lease, LeaseStatus } from './types';

const leaseStatusMap: Record<
  components['schemas']['LeaseResponse']['status'],
  LeaseStatus
> = {
  awaiting_start: 'awaiting_start',
  active: 'active',
  requires_action: 'requires_action',
  completed: 'completed',
  archived: 'archived',
};

export function mapLeaseResponse(
  dto: components['schemas']['LeaseResponse'],
): Lease {
  return {
    id: dto.id,
    propertyId: dto.property_id,
    tenantContactId: dto.tenant_contact?.id,
    tenantName: dto.tenant_contact?.name ?? 'Арендатор',
    tenantContact: dto.tenant_contact
      ? {
          id: dto.tenant_contact.id,
          name: dto.tenant_contact.name,
          surname: dto.tenant_contact.surname,
          patronymic: dto.tenant_contact.patronymic,
          phone: dto.tenant_contact.phone,
          email: dto.tenant_contact.email,
        }
      : null,
    rentKopecks: dto.rent_amount_kopecks,
    startDate: dto.start_date,
    endDate: dto.end_date ?? undefined,
    status: leaseStatusMap[dto.status],
  };
}
