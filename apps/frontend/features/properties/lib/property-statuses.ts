import type { PropertyStatus } from '@/entities/property';

/**
 * Экранное представление статуса объекта. После удаления домена аренд
 * (спека #434) занятости больше нет: активный объект либо «в работе»
 * (active), либо «на ремонте» (maintenance); архив скрывает бейдж списка.
 */
export type DisplayStatus =
  | 'active'
  | 'maintenance';

export function getDisplayStatus(
  status: PropertyStatus,
): DisplayStatus | null {
  if (status === 'archived') return null;
  if (status === 'maintenance') return 'maintenance';
  return 'active';
}

export type StatusFilterValue = DisplayStatus;

export const statusFilterOptions: { value: StatusFilterValue; label: string }[] = [
  { value: 'active', label: 'Активные' },
  { value: 'maintenance', label: 'На ремонте' },
];
