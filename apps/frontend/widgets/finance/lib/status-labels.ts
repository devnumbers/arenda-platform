import type { OperationStatus } from '@/entities/operation/model/types';

export const statusLabels: Record<OperationStatus, string> = {
  pending: 'Ожидается',
  overdue: 'Просрочено',
  paid: 'Оплачено',
  received: 'Получено',
  unconfirmed: 'Не подтверждено',
};

export const statusVariants: Record<OperationStatus, 'warning' | 'success'> = {
  pending: 'warning',
  overdue: 'warning',
  paid: 'success',
  received: 'success',
  unconfirmed: 'warning',
};
