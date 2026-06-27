import { pluralize } from '@/shared/lib/pluralize';

const DAY_MS = 24 * 60 * 60 * 1000;

export function formatDeadline(iso: string): string {
  const target = new Date(iso).getTime();

  if (Number.isNaN(target)) {
    return '';
  }

  const now = Date.now();
  const diffDays = Math.ceil((target - now) / DAY_MS);

  if (diffDays < 0) {
    return 'Просрочено';
  }

  if (diffDays === 0) {
    return 'Сегодня';
  }

  return `${diffDays} ${pluralize(diffDays, 'день', 'дня', 'дней')}`;
}
