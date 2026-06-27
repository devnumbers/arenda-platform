import { formatDuration } from '@/shared/lib/format-duration';

export function formatAwaitingStart(startDate: string, now: string | Date = new Date()): string {
  return `Аренда через ${formatDuration(typeof now === 'string' ? now : now.toISOString(), startDate)}`;
}
