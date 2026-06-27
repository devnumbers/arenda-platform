import type { Property } from '@/entities/property/model/types';
import type { components } from '@/shared/api/generated';

type PropertyStatus = Property['status'];
type Lease = components['schemas']['LeaseResponse'];

export type PropertyPageStatus =
  | 'rented'
  | 'requires_action'
  | 'awaiting_start'
  | 'finished'
  | 'free'
  | 'maintenance'
  | 'archived';

export function getPropertyPageStatus(
  propertyStatus: PropertyStatus,
  leases: Lease[],
): PropertyPageStatus {
  if (propertyStatus === 'archived') return 'archived';
  if (propertyStatus === 'maintenance') return 'maintenance';

  const openLease = leases.find(
    (l) => l.status !== 'completed' && l.status !== 'archived',
  );
  if (openLease) {
    if (openLease.status === 'requires_action') return 'requires_action';
    if (openLease.status === 'awaiting_start') return 'awaiting_start';
    return 'rented';
  }

  const finished = leases.find((l) => l.status === 'completed');
  if (finished) return 'finished';

  return 'free';
}

export function findCurrentLease(leases: Lease[]): Lease | undefined {
  return leases.find(
    (l) => l.status !== 'completed' && l.status !== 'archived',
  );
}

export function findLastLease(leases: Lease[]): Lease | undefined {
  return leases.find((l) => l.status === 'completed');
}
