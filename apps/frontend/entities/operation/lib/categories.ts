import {
  incomeCategories as incomeCategoryOptions,
  expenseCategories as expenseCategoryOptions,
  type OperationCategory,
  type OperationType,
} from '../model/types';

export { incomeCategoryOptions as incomeCategories };
export { expenseCategoryOptions as expenseCategories };

const categoryLabelMap: Record<OperationCategory, string> = {
  rent: 'Арендная плата',
  other_income: 'Прочий доход',
  utilities: 'ЖКХ',
  repair: 'Ремонт',
  tax: 'Налог',
  other_expense: 'Прочее',
};

export function getCategoryLabel(category: string): string {
  return categoryLabelMap[category as OperationCategory] ?? category;
}

export function getCategoriesByType(
  type: OperationType,
): ReadonlyArray<{ readonly value: OperationCategory; readonly label: string }> {
  return type === 'income' ? incomeCategoryOptions : expenseCategoryOptions;
}
