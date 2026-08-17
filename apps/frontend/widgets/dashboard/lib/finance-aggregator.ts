import type { components } from '@/shared/api/dto';
import type { OperationStatus } from '@/entities/operation/model/types';

type OperationsResponse = components['schemas']['OperationsResponse'];
type OperationResponse = components['schemas']['OperationResponse'];

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
 * Backward compatibility: older responses may not contain `status`.
 * Such operations are treated as actual (paid/received) so the dashboard
 * keeps showing already-completed amounts instead of silently dropping them.
 *
 * `unconfirmed` operations (created retroactively, awaiting user confirmation)
 * do not participate in financial totals and are skipped entirely.
 */
function getOperationBucket(status: OperationStatus | undefined): 'actual' | 'pending' | null {
  if (status === 'unconfirmed') {
    return null;
  }

  if (status === 'pending' || status === 'overdue') {
    return 'pending';
  }

  // `paid`, `received`, or a missing/unknown status are treated as actual.
  return 'actual';
}

function addOperationToBucket(bucket: MutableIncomeExpense, operation: OperationResponse): void {
  if (operation.type === 'income') {
    bucket.incomeKopecks += operation.amount_kopecks;
  } else {
    bucket.expenseKopecks += operation.amount_kopecks;
  }
}

export function aggregateOperations(
  operationsList: ReadonlyArray<OperationsResponse>,
): AggregateResult {
  const actual: MutableIncomeExpense = { incomeKopecks: 0, expenseKopecks: 0 };
  const pending: MutableIncomeExpense = { incomeKopecks: 0, expenseKopecks: 0 };

  for (const response of operationsList) {
    for (const operation of response.items) {
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
