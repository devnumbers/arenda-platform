import type { PropertyStatus, Occupancy } from '@/entities/property/model/types';
import type { LeaseStatus } from '@/entities/lease/model/types';

export type DisplayStatus = 'rented' | 'free' | 'maintenance' | 'overdue' | 'finished';

export const displayStatusLabels: Record<DisplayStatus, string> = {
  rented: 'Арендован',
  free: 'Не арендован',
  maintenance: 'На ремонте',
  overdue: '1 просроченная операция',
  finished: 'Аренда завершена',
};

export function getDisplayStatus(
  status: PropertyStatus,
  occupancy: Occupancy,
  leaseStatus?: LeaseStatus,
): DisplayStatus | null {
  if (status === 'archived') return null;
  if (status === 'maintenance') return 'maintenance';
  if (leaseStatus === 'active') return 'rented';
  if (leaseStatus === 'completed') return 'finished';
  return occupancy === 'occupied' ? 'rented' : 'free';
}

export type StatusFilterValue = 'rented' | 'free' | 'maintenance' | 'finished';

export const statusFilterOptions: { value: StatusFilterValue; label: string }[] = [
  { value: 'rented', label: 'Аренда' },
  { value: 'free', label: 'Без аренды' },
  { value: 'maintenance', label: 'На ремонте' },
  { value: 'finished', label: 'Аренда завершена' },
];
