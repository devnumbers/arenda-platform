import type { Property } from '@/entities/property/model/types';
import type { components } from '@/shared/api/generated';
import {
  getEffectiveLeaseStatus,
  isOpenLeaseStatus,
} from '@/entities/lease/lib/status';

type PropertyStatus = Property['status'];
type Lease = components['schemas']['LeaseResponse'];

export type PropertyPageStatus =
  | 'rented'
  | 'requires_action'
  | 'awaiting_start'
  | 'free'
  | 'maintenance'
  | 'archived';

export function getPropertyPageStatus(
  propertyStatus: PropertyStatus,
  leases: Lease[],
): PropertyPageStatus {
  if (propertyStatus === 'archived') return 'archived';
  if (propertyStatus === 'maintenance') return 'maintenance';

  const openLease = findCurrentLease(leases);
  if (openLease) {
    const effectiveStatus = getEffectiveLeaseStatus({
      status: openLease.status,
      startDate: openLease.start_date,
      endDate: openLease.end_date ?? undefined,
    });
    if (effectiveStatus === 'requires_action') return 'requires_action';
    if (effectiveStatus === 'awaiting_start') return 'awaiting_start';
    return 'rented';
  }

  return 'free';
}

export function findCurrentLease(leases: Lease[]): Lease | undefined {
  return leases.find((lease) => isOpenLeaseStatus(lease.status));
}
