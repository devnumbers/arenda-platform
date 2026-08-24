import type { PropertyStatus } from '@/entities/property';

export type DisplayStatus =
  | 'free'
  | 'maintenance';

export const displayStatusLabels: Record<DisplayStatus, string> = {
  free: 'Не арендован',
  maintenance: 'На ремонте',
};

export function getDisplayStatus(
  status: PropertyStatus,
): DisplayStatus | null {
  if (status === 'archived') return null;
  if (status === 'maintenance') return 'maintenance';
  return 'free';
}

export type StatusFilterValue = 'free' | 'maintenance';

export const statusFilterOptions: { value: StatusFilterValue; label: string }[] = [
  { value: 'free', label: 'Без аренды' },
  { value: 'maintenance', label: 'На ремонте' },
];
