import type { components } from '@/shared/api/generated';

type OperationCategory = components['schemas']['OperationCategory'];
type OperationType = components['schemas']['OperationType'];

type CategoryOption = {
  value: OperationCategory;
  label: string;
};

export const incomeCategories: CategoryOption[] = [
  { value: 'rent', label: 'Арендная плата' },
  { value: 'other_income', label: 'Прочий доход' },
];

export const expenseCategories: CategoryOption[] = [
  { value: 'utilities', label: 'ЖКХ' },
  { value: 'repair', label: 'Ремонт' },
  { value: 'tax', label: 'Налог' },
  { value: 'other_expense', label: 'Прочее' },
];

export function getCategoriesByType(type: OperationType): CategoryOption[] {
  return type === 'income' ? incomeCategories : expenseCategories;
}

export function getCategoryLabel(category: OperationCategory): string {
  const option = [...incomeCategories, ...expenseCategories].find(
    (item) => item.value === category,
  );
  return option?.label ?? category;
}
