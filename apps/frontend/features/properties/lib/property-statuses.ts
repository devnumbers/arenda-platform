import type { PropertyStatus, Occupancy } from '@/entities/property';
import type { LeaseStatus } from '@/entities/lease';

export type DisplayStatus =
  | 'rented'
  | 'requires_action'
  | 'awaiting_start'
  | 'free'
  | 'maintenance'
  | 'overdue';

export const displayStatusLabels: Record<Exclude<DisplayStatus, 'overdue'>, string> = {
  rented: 'Арендован',
  requires_action: 'Требует действия',
  awaiting_start: 'Скоро начнётся',
  free: 'Не арендован',
  maintenance: 'На ремонте',
};

export function getDisplayStatus(
  status: PropertyStatus,
  occupancy: Occupancy,
  leaseStatus?: LeaseStatus,
  overdueRentCount: number = 0,
): DisplayStatus | null {
  if (status === 'archived') return null;
  if (status === 'maintenance') return 'maintenance';
  if (overdueRentCount > 0) return 'overdue';
  if (leaseStatus === 'active') return 'rented';
  if (leaseStatus === 'requires_action') return 'requires_action';
  if (leaseStatus === 'awaiting_start') return 'awaiting_start';
  return occupancy === 'occupied' ? 'rented' : 'free';
}

export function formatOverduePaymentsCount(n: number): string {
  const mod10 = n % 10;
  const mod100 = n % 100;
  const word =
    mod10 === 1 && mod100 !== 11 ? 'платёж'
    : mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14) ? 'платежа'
    : 'платежей';
  return `${n} просроченный ${word}`;
}

export type StatusFilterValue = 'rented' | 'free' | 'maintenance';

export const statusFilterOptions: { value: StatusFilterValue; label: string }[] = [
  { value: 'rented', label: 'Аренда' },
  { value: 'free', label: 'Без аренды' },
  { value: 'maintenance', label: 'На ремонте' },
];
