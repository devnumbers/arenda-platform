import type { Operation, OperationStatus, OperationsPage } from '@/entities/operation';

type IncomeExpense = {
  readonly incomeKopecks: number;
  readonly expenseKopecks: number;
};

type MutableIncomeExpense = {
  incomeKopecks: number;
  expenseKopecks: number;
};

export type AggregateResult = {
  readonly actual: IncomeExpense & {
    readonly profitKopecks: number;
  };
  readonly pending: IncomeExpense;
};

/**
 * Returns the bucket an operation belongs to.
 *
 * `unconfirmed` operations (created retroactively, awaiting user confirmation)
 * do not participate in financial totals and are skipped entirely.
 */
function getOperationBucket(status: OperationStatus): 'actual' | 'pending' | null {
  if (status === 'unconfirmed') {
    return null;
  }

  if (status === 'pending' || status === 'overdue') {
    return 'pending';
  }

  // `paid` and `received` are treated as actual.
  return 'actual';
}

function addOperationToBucket(bucket: MutableIncomeExpense, operation: Pick<Operation, 'type' | 'amountKopecks'>): void {
  if (operation.type === 'income') {
    bucket.incomeKopecks += operation.amountKopecks;
  } else {
    bucket.expenseKopecks += operation.amountKopecks;
  }
}

export function aggregateOperations(
  operationsList: ReadonlyArray<OperationsPage>,
): AggregateResult {
  const actual: MutableIncomeExpense = { incomeKopecks: 0, expenseKopecks: 0 };
  const pending: MutableIncomeExpense = { incomeKopecks: 0, expenseKopecks: 0 };

  for (const page of operationsList) {
    for (const operation of page.items) {
      const bucket = getOperationBucket(operation.status);
      if (bucket === null) continue;
      addOperationToBucket(bucket === 'actual' ? actual : pending, operation);
    }
  }

  return {
    actual: {
      incomeKopecks: actual.incomeKopecks,
      expenseKopecks: actual.expenseKopecks,
      profitKopecks: actual.incomeKopecks - actual.expenseKopecks,
    },
    pending,
  };
}
