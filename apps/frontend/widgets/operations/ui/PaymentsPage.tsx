'use client';

import type { JSX } from 'react';
import { Plus } from '@/shared/assets/icons';
import { Icon } from '@/shared/ui/icon';
import { LinkButton } from '@/shared/ui/link-button';
import { ROUTES } from '@/shared/config/routes';
import { useOperations } from '@/features/operations/api/hooks';
import { FinanceLoading } from '@/widgets/finance/ui/FinanceLoading';
import { FinanceErrorState } from '@/widgets/finance/ui/FinanceErrorState';
import { FinanceEmptyState } from '@/widgets/finance/ui/FinanceEmptyState';
import { OperationsList } from './OperationsList';
import styles from './PaymentsPage.module.css';

export function PaymentsPage(): JSX.Element {
  const {
    data,
    isLoading,
    isFetching,
    isError,
    refetch,
  } = useOperations({
    status: ['pending', 'overdue'],
    limit: 100,
  });

  const operations = data?.items ?? [];

  return (
    <div className={styles.root}>
      <div className={styles.header}>
        <h1 className={styles.title}>Платежи</h1>
        <LinkButton
          href={ROUTES.financeCreateOperation}
          variant="primary"
          size="medium"
          leftIcon={
            <Icon size="s">
              <Plus />
            </Icon>
          }
        >
          Добавить операцию
        </LinkButton>
      </div>

      {isLoading && <FinanceLoading />}

      {!isLoading && isError && (
        <FinanceErrorState onRetry={refetch} isLoading={isFetching} />
      )}

      {!isLoading && !isError && operations.length === 0 && (
        <FinanceEmptyState
          title="Нет платежей"
          subtitle="Запланированные и просроченные операции появятся здесь."
        />
      )}

      {!isLoading && !isError && operations.length > 0 && (
        <OperationsList items={operations} showProperty />
      )}
    </div>
  );
}
