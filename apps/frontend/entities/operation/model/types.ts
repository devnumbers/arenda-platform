export type OperationType = 'income' | 'expense';

export type OperationCategory =
  | 'rent'
  | 'other_income'
  | 'utilities'
  | 'repair'
  | 'tax'
  | 'other_expense';

export type OperationFrequency = 'once' | 'monthly' | 'yearly';

export const incomeCategories: { value: OperationCategory; label: string }[] = [
  { value: 'rent', label: 'Арендная плата' },
  { value: 'other_income', label: 'Прочий доход' },
];

export const expenseCategories: { value: OperationCategory; label: string }[] = [
  { value: 'utilities', label: 'ЖКХ' },
  { value: 'repair', label: 'Ремонт' },
  { value: 'tax', label: 'Налог' },
  { value: 'other_expense', label: 'Прочее' },
];
