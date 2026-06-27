import { pluralize } from '@/shared/lib/pluralize';

const DAY_MS = 24 * 60 * 60 * 1000;

export function formatDuration(start: string, end: string): string {
  const startTime = new Date(start).getTime();
  const endTime = new Date(end).getTime();
  if (Number.isNaN(startTime) || Number.isNaN(endTime)) return '';
  const days = Math.max(0, Math.floor((endTime - startTime) / DAY_MS));
  const weeks = Math.floor(days / 7);
  const remDays = days % 7;
  if (weeks === 0) return `${remDays} ${pluralize(remDays, 'день', 'дня', 'дней')}`;
  return `${weeks} ${pluralize(weeks, 'неделя', 'недели', 'недель')}${remDays ? `, ${remDays} ${pluralize(remDays, 'день', 'дня', 'дней')}` : ''}`;
}

export function formatAgo(iso: string): string {
  const endTime = new Date(iso).getTime();
  if (Number.isNaN(endTime)) return '';
  const days = Math.max(0, Math.floor((Date.now() - endTime) / DAY_MS));

  if (days === 0) {
    return 'сегодня';
  }

  const months = Math.floor(days / 30);
  const years = Math.floor(days / 365);

  if (years > 0) {
    const remMonths = Math.floor((days % 365) / 30);

    if (remMonths === 0) {
      return `${years} ${pluralize(years, 'год', 'года', 'лет')} назад`;
    }

    return `${years} ${pluralize(years, 'год', 'года', 'лет')}, ${remMonths} ${pluralize(remMonths, 'месяц', 'месяца', 'месяцев')} назад`;
  }

  if (months > 0) {
    return `${months} ${pluralize(months, 'месяц', 'месяца', 'месяцев')} назад`;
  }

  return `${days} ${pluralize(days, 'день', 'дня', 'дней')} назад`;
}


