import type { LeaseStatus } from '@/entities/lease';

const labels: Record<LeaseStatus, string> = {
  awaiting_start: 'Скоро начнётся',
  active: 'Активна',
  requires_action: 'Требует действия',
  completed: 'Завершена',
  archived: 'В архиве',
};

export function getLeaseStatusLabel(status: LeaseStatus): string {
  return labels[status] ?? status;
}
