import type { components } from '@/shared/api/generated';

type OperationCategory = components['schemas']['OperationCategory'];

export const categoryLabels: Partial<Record<OperationCategory, string>> = {
  rent: 'Арендная плата',
  other_income: 'Прочий доход',
  utilities: 'ЖКХ',
  repair: 'Ремонт',
  tax: 'Налог',
  other_expense: 'Прочее',
};
