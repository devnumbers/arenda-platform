export type OperationCategoryType = 'income' | 'expense';

export const categoryKeys = {
  all: ['operation-categories'] as const,
  list: (type: OperationCategoryType) =>
    ['operation-categories', 'list', type] as const,
};
