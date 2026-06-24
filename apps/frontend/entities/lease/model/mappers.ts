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
    tenantName: dto.tenant_contact?.name ?? 'Арендатор',
    rentKopecks: dto.rent_amount_kopecks,
    startDate: dto.start_date,
    endDate: dto.end_date ?? undefined,
    status: leaseStatusMap[dto.status],
  };
}
