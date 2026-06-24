import type { components } from '@/shared/api/generated';

type OperationsResponse = components['schemas']['OperationsResponse'];

export type AggregateResult = {
  readonly incomeKopecks: number;
  readonly expenseKopecks: number;
  readonly profitKopecks: number;
};

export function aggregateOperations(
  operationsList: ReadonlyArray<OperationsResponse>,
): AggregateResult {
  let incomeKopecks = 0;
  let expenseKopecks = 0;

  for (const response of operationsList) {
    for (const operation of response.items) {
      if (operation.type === 'income') {
        incomeKopecks += operation.amount_kopecks;
      } else {
        expenseKopecks += operation.amount_kopecks;
      }
    }
  }

  return {
    incomeKopecks,
    expenseKopecks,
    profitKopecks: incomeKopecks - expenseKopecks,
  };
}
