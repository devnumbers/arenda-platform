import type { components } from '@/shared/api/generated';
import type { Lease, LeaseStatus } from './types';

const leaseStatusMap: Record<string, LeaseStatus> = {
  active: 'active',
  completed: 'completed',
  cancelled: 'cancelled',
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
    status: leaseStatusMap[dto.status] ?? 'active',
  };
}
