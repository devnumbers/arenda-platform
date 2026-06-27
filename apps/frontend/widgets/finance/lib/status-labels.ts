import type { components } from '@/shared/api/generated';

type OperationStatus = components['schemas']['OperationStatus'];

export const statusLabels: Record<OperationStatus, string> = {
  pending: 'Ожидается',
  overdue: 'Просрочено',
  paid: 'Оплачено',
  received: 'Получено',
};

export const statusVariants: Record<OperationStatus, 'warning' | 'success'> = {
  pending: 'warning',
  overdue: 'warning',
  paid: 'success',
  received: 'success',
};
