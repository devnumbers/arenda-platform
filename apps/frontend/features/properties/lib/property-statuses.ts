import type { PropertyStatus, Occupancy } from '@/entities/property/model/types';
import type { LeaseStatus } from '@/entities/lease/model/types';

export type DisplayStatus =
  | 'rented'
  | 'requires_action'
  | 'awaiting_start'
  | 'free'
  | 'maintenance'
  | 'overdue';

export const displayStatusLabels: Record<DisplayStatus, string> = {
  rented: 'Арендован',
  requires_action: 'Требует действия',
  awaiting_start: 'Скоро начнётся',
  free: 'Не арендован',
  maintenance: 'На ремонте',
  overdue: '1 просроченная операция',
};

export function getDisplayStatus(
  status: PropertyStatus,
  occupancy: Occupancy,
  leaseStatus?: LeaseStatus,
): DisplayStatus | null {
  if (status === 'archived') return null;
  if (status === 'maintenance') return 'maintenance';
  if (leaseStatus === 'active') return 'rented';
  if (leaseStatus === 'requires_action') return 'requires_action';
  if (leaseStatus === 'awaiting_start') return 'awaiting_start';
  return occupancy === 'occupied' ? 'rented' : 'free';
}

export type StatusFilterValue = 'rented' | 'free' | 'maintenance';

export const statusFilterOptions: { value: StatusFilterValue; label: string }[] = [
  { value: 'rented', label: 'Аренда' },
  { value: 'free', label: 'Без аренды' },
  { value: 'maintenance', label: 'На ремонте' },
];
