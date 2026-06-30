'use client';

import type { JSX } from 'react';
import NextLink from 'next/link';
import { Skeleton } from '@heroui/react/skeleton';
import { ArrowRight } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { Button } from '@/shared/ui/button';
import { Icon } from '@/shared/ui/icon';
import { useOperationsByProperty } from '@/features/operations/api/hooks';
import { OperationListItem } from '@/widgets/operations/ui/OperationListItem';
import { PropertyDetailSection } from './PropertyDetailSection';
import styles from './PropertyOverdueOperationsCard.module.css';

export type PropertyOverdueOperationsCardProps = {
  readonly propertyId: string;
};

function getOverdueOperationsHref(propertyId: string): string {
  const params = new URLSearchParams({
    property_id: propertyId,
    period: 'all',
    status: 'overdue',
    sort: 'operation_date_asc',
  });
  return `${ROUTES.financeOperations}?${params.toString()}`;
}

export function PropertyOverdueOperationsCard({
  propertyId,
}: PropertyOverdueOperationsCardProps): JSX.Element {
  const overdueQuery = useOperationsByProperty(propertyId, {
    status: ['overdue'],
    sort: 'operation_date_asc',
    limit: 5,
  });

  const operations = overdueQuery.data?.items ?? [];

  return (
    <PropertyDetailSection>
      <div className={styles.header}>
        <h2 className={styles.title}>Просроченные операции</h2>
        <NextLink
          href={getOverdueOperationsHref(propertyId)}
          className={styles.headerLink}
          aria-label="Все просроченные операции"
        >
          <Icon size="s">
            <ArrowRight />
          </Icon>
        </NextLink>
      </div>

      {overdueQuery.isLoading && (
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

      {!overdueQuery.isLoading && overdueQuery.isError && (
        <div className={styles.error} role="alert" aria-live="polite">
          <p className={styles.errorText}>Не удалось загрузить просроченные операции</p>
          <Button
            variant="secondary"
            size="small"
            loading={overdueQuery.isFetching}
            onClick={() => {
              void overdueQuery.refetch();
            }}
          >
            Повторить
          </Button>
        </div>
      )}

      {!overdueQuery.isLoading && !overdueQuery.isError && operations.length === 0 && (
        <p className={styles.emptyText}>Нет просроченных операций</p>
      )}

      {!overdueQuery.isLoading && !overdueQuery.isError && operations.length > 0 && (
        <ul className={styles.list}>
          {operations.map((operation) => (
            <li key={operation.id}>
              <OperationListItem operation={operation} variant="dashboard" />
            </li>
          ))}
        </ul>
      )}
    </PropertyDetailSection>
  );
}
