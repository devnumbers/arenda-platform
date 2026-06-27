import { getLeaseMonthCount } from '@/shared/lib/format-lease-month';
import { pluralize } from '@/shared/lib/pluralize';

const DAY_MS = 24 * 60 * 60 * 1000;

export function formatRemainingDuration(start: string, end?: string | null): string {
  if (!end) {
    return 'Бессрочно';
  }

  const startDate = new Date(start);
  const endDate = new Date(end);

  if (Number.isNaN(startDate.getTime()) || Number.isNaN(endDate.getTime())) {
    return '';
  }

  const now = Date.now();

  if (endDate.getTime() <= now) {
    return 'Срок истёк';
  }

  const totalDays = Math.max(1, Math.ceil((endDate.getTime() - Math.max(startDate.getTime(), now)) / DAY_MS));

  if (totalDays < 7) {
    return `${totalDays} ${pluralize(totalDays, 'день', 'дня', 'дней')}`;
  }

  const weeks = Math.floor(totalDays / 7);
  const days = totalDays % 7;
  const weeksText = `${weeks} ${pluralize(weeks, 'неделя', 'недели', 'недель')}`;

  if (days === 0) {
    return weeksText;
  }

  return `${weeksText}, ${days} ${pluralize(days, 'день', 'дня', 'дней')}`;
}

export function formatCurrentLeaseMonth(start: string): string {
  const startDate = new Date(start);
  if (Number.isNaN(startDate.getTime())) return '';

  const months = getLeaseMonthCount(start) + 1;
  return `${months} ${pluralize(months, 'месяц', 'месяца', 'месяцев')}`;
}
