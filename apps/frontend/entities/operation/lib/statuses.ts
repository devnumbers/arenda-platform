import type { OperationStatus } from '../model/types';

export type OperationStatusOption = {
  readonly value: OperationStatus;
  readonly label: string;
  readonly variant: 'warning' | 'danger' | 'success' | 'default';
};

export const operationStatusOptions: ReadonlyArray<OperationStatusOption> = [
  { value: 'pending', label: 'Запланирована', variant: 'warning' },
  { value: 'overdue', label: 'Просрочена', variant: 'danger' },
  { value: 'paid', label: 'Выполнена', variant: 'success' },
  { value: 'received', label: 'Выполнена', variant: 'success' },
  { value: 'unconfirmed', label: 'Не подтверждена', variant: 'warning' },
];

const statusLabelMap: Record<OperationStatus, string> = {
  pending: 'Запланирована',
  overdue: 'Просрочена',
  paid: 'Выполнена',
  received: 'Выполнена',
  unconfirmed: 'Не подтверждена',
};

export function getOperationStatusLabel(status: OperationStatus): string {
  return statusLabelMap[status] ?? status;
}
