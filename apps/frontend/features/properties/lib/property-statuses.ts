import type { components } from '@/shared/api/generated';

export type PropertyStatus = components['schemas']['PropertyResponse']['status'];
export type Occupancy = components['schemas']['PropertyResponse']['occupancy'];

export type DisplayStatus = 'rented' | 'free' | 'maintenance';

export const displayStatusConfig: Record<
  DisplayStatus,
  { label: string; color: 'success' | 'warning' | 'neutral' }
> = {
  rented: { label: 'Арендован', color: 'success' },
  free: { label: 'Не арендован', color: 'neutral' },
  maintenance: { label: 'На ремонте', color: 'warning' },
};

export function getDisplayStatus(
  status: PropertyStatus,
  occupancy: Occupancy,
): DisplayStatus | null {
  if (status === 'archived') return null;
  if (status === 'maintenance') return 'maintenance';
  return occupancy === 'occupied' ? 'rented' : 'free';
}

export type StatusFilterValue = DisplayStatus;

export const statusFilterOptions: { value: StatusFilterValue; label: string }[] = [
  { value: 'rented', label: 'Аренда' },
  { value: 'free', label: 'Без аренды' },
  { value: 'maintenance', label: 'На ремонте' },
];
