import type { components } from '@/shared/api/generated';

type OperationStatus = components['schemas']['OperationStatus'];

type OperationStatusOption = {
  value: OperationStatus;
  label: string;
  variant: 'warning' | 'danger' | 'success' | 'default';
};

export const operationStatusOptions: OperationStatusOption[] = [
  { value: 'pending', label: 'Запланирована', variant: 'warning' },
  { value: 'overdue', label: 'Просрочена', variant: 'danger' },
  { value: 'paid', label: 'Оплачена', variant: 'success' },
  { value: 'received', label: 'Получена', variant: 'success' },
];

export function getOperationStatusLabel(status: OperationStatus): string {
  return operationStatusOptions.find((option) => option.value === status)?.label ?? status;
}
