import { formatCountdownLabel } from '@/shared/lib/format-countdown';
import { diffDays, parseLocalDate, startOfDay } from '@/shared/lib/lease-payment';

export function formatAwaitingStart(startDate: string, now: string | Date = new Date()): string {
  const nowDate = typeof now === 'string' ? new Date(now) : now;
  const days = diffDays(startOfDay(nowDate), parseLocalDate(startDate));
  const label = formatCountdownLabel(days);
  return `Аренда начнётся ${label.charAt(0).toLowerCase()}${label.slice(1)}`;
}
