import { pluralize } from '@/shared/lib/pluralize';

type LeaseStatus =
  | 'awaiting_start'
  | 'active'
  | 'requires_action'
  | 'completed'
  | 'archived';

const DAY_MS = 24 * 60 * 60 * 1000;

function getDateTime(value: string | Date): number {
  return value instanceof Date ? value.getTime() : new Date(value).getTime();
}

export function formatLeaseRemainingDuration(
  start: string,
  end?: string | null,
  now: string | Date = new Date(),
): string {
  if (!end) {
    return 'Бессрочно';
  }

  const startTime = getDateTime(start);
  const endTime = getDateTime(end);
  const nowTime = getDateTime(now);

  if (
    Number.isNaN(startTime) ||
    Number.isNaN(endTime) ||
    Number.isNaN(nowTime)
  ) {
    return '';
  }

  if (endTime <= nowTime) {
    return 'Срок истёк';
  }

  const totalDays = Math.max(
    1,
    Math.ceil((endTime - Math.max(startTime, nowTime)) / DAY_MS),
  );

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

export function formatCurrentLeaseMonth(
  start: string,
  status?: LeaseStatus,
  now: string | Date = new Date(),
): string {
  if (status === 'awaiting_start') {
    return '';
  }

  const startDate = new Date(start);
  const nowDate = now instanceof Date ? now : new Date(now);

  if (
    Number.isNaN(startDate.getTime()) ||
    Number.isNaN(nowDate.getTime()) ||
    startDate.getTime() > nowDate.getTime()
  ) {
    return '';
  }

  const months =
    (nowDate.getFullYear() - startDate.getFullYear()) * 12 +
    (nowDate.getMonth() - startDate.getMonth()) +
    1;
  const normalized = Math.max(1, months);

  return `${normalized} ${pluralize(normalized, 'месяц', 'месяца', 'месяцев')}`;
}
