'use client';

import { useMemo, type JSX } from 'react';
import { Skeleton } from '@heroui/react/skeleton';
import type { components } from '@/shared/api/generated';
import { OperationListItem } from '@/widgets/operations/ui/OperationListItem';
import { FinanceErrorState } from './FinanceErrorState';
import sectionStyles from './FinanceSection.module.css';
import styles from './FinanceUpcomingOperations.module.css';

type OperationResponse = components['schemas']['OperationResponse'];

interface FinanceUpcomingOperationsProps {
  operations: OperationResponse[];
  isLoading: boolean;
  isFetching: boolean;
  isError: boolean;
  refetch: () => void;
}

export function FinanceUpcomingOperations({
  operations: rawOperations,
  isLoading,
  isFetching,
  isError,
  refetch,
}: FinanceUpcomingOperationsProps): JSX.Element {
  const operations = useMemo(() => {
    return [...rawOperations].sort(
      (a, b) => new Date(a.operation_date).getTime() - new Date(b.operation_date).getTime(),
    );
  }, [rawOperations]);

  if (isLoading) {
    return (
      <section className={sectionStyles.section}>
        <h2 className={sectionStyles.sectionTitle}>Ближайшие операции</h2>
        <ul className={sectionStyles.operationsList}>
          <li>
            <Skeleton className={styles.rowSkeleton} />
          </li>
          <li>
            <Skeleton className={styles.rowSkeleton} />
          </li>
          <li>
            <Skeleton className={styles.rowSkeleton} />
          </li>
        </ul>
      </section>
    );
  }

  if (isError) {
    return (
      <section className={sectionStyles.section}>
        <h2 className={sectionStyles.sectionTitle}>Ближайшие операции</h2>
        <FinanceErrorState onRetry={refetch} isLoading={isFetching} />
      </section>
    );
  }

  return (
    <section className={sectionStyles.section}>
      <h2 className={sectionStyles.sectionTitle}>Ближайшие операции</h2>
      {operations.length === 0 ? (
        <p className={styles.upcomingEmpty}>Нет запланированных операций</p>
      ) : (
        <ul className={sectionStyles.operationsList}>
          {operations.map((operation) => (
            <li key={operation.id}>
              <OperationListItem operation={operation} variant="dashboard" />
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
