import { pluralize } from '@/shared/lib/pluralize';

export function getLeaseMonthCount(start: string): number {
  const startDate = new Date(start);
  if (Number.isNaN(startDate.getTime())) return 0;
  const now = new Date();
  const months =
    (now.getFullYear() - startDate.getFullYear()) * 12 +
    (now.getMonth() - startDate.getMonth());
  return Math.max(0, months);
}

export function formatLeaseMonth(start: string): string {
  const normalized = getLeaseMonthCount(start);
  if (normalized === 0) return '';
  return `${normalized} ${pluralize(normalized, 'месяц', 'месяца', 'месяцев')}`;
}
