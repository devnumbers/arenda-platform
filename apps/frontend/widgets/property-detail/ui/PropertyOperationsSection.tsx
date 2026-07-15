'use client';

import type { JSX } from 'react';
import { Skeleton } from '@heroui/react/skeleton';
import { Button } from '@/shared/ui/button';
import { useOperationsByProperty } from '@/features/operations/api/hooks';
import type { OperationsFilters } from '@/features/operations/api/hooks';
import { OperationListItem } from '@/widgets/operations/ui/OperationListItem';
import { PropertyDetailSection } from './PropertyDetailSection';
import styles from './PropertyOperationsSection.module.css';

export type PropertyOperationsSectionProps = {
  readonly propertyId: string;
  readonly title: string;
  readonly emptyText: string;
  readonly filters: Omit<OperationsFilters, 'property_id'>;
};

export function PropertyOperationsSection({
  propertyId,
  title,
  emptyText,
  filters,
}: PropertyOperationsSectionProps): JSX.Element {
  const operationsQuery = useOperationsByProperty(propertyId, filters);

  const operations = operationsQuery.data?.items ?? [];

  return (
    <PropertyDetailSection>
      <div className={styles.header}>
        <h2 className={styles.title}>{title}</h2>
      </div>

      {operationsQuery.isLoading && (
        <ul className={styles.list}>
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
      )}

      {!operationsQuery.isLoading && operationsQuery.isError && (
        <div className={styles.error} role="alert" aria-live="polite">
          <p className={styles.errorText}>Не удалось загрузить операции</p>
          <Button
            variant="secondary"
            size="small"
            loading={operationsQuery.isFetching}
            onClick={() => {
              void operationsQuery.refetch();
            }}
          >
            Повторить
          </Button>
        </div>
      )}

      {!operationsQuery.isLoading && !operationsQuery.isError && operations.length === 0 && (
        <p className={styles.emptyText}>{emptyText}</p>
      )}

      {!operationsQuery.isLoading && !operationsQuery.isError && operations.length > 0 && (
        <ul className={styles.list}>
          {operations.map((operation) => (
            <li key={operation.id}>
              <OperationListItem operation={operation} />
            </li>
          ))}
        </ul>
      )}
    </PropertyDetailSection>
  );
}
