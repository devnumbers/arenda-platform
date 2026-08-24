import type { Property } from '@/entities/property';

type PropertyStatus = Property['status'];

export type PropertyPageStatus =
  | 'free'
  | 'maintenance'
  | 'archived';

export function getPropertyPageStatus(
  propertyStatus: PropertyStatus,
): PropertyPageStatus {
  if (propertyStatus === 'archived') return 'archived';
  if (propertyStatus === 'maintenance') return 'maintenance';
  return 'free';
}
