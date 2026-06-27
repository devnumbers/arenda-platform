'use client';

import type { JSX } from 'react';
import type { components } from '@/shared/api/generated';
import { ROUTES } from '@/shared/config/routes';
import { Button } from '@/shared/ui/button';
import { getCategoryLabel } from '@/entities/operation/lib/categories';
import { formatOperationDate } from '@/entities/operation/lib/dates';
import { formatMoneyKopecks } from '@/entities/operation/lib/formatMoney';
import {
  useRecurringOperations,
  usePauseRecurringOperation,
  useResumeRecurringOperation,
} from '@/features/recurring-operations/api/hooks';
import { FinanceLoading } from '@/widgets/finance/ui/FinanceLoading';
import { FinanceErrorState } from '@/widgets/finance/ui/FinanceErrorState';
import { FinanceEmptyState } from '@/widgets/finance/ui/FinanceEmptyState';
import styles from './RecurringOperationsTab.module.css';

type RecurringOperationResponse =
  components['schemas']['RecurringOperationResponse'];

const PERIODICITY_LABELS: Record<RecurringOperationResponse['periodicity'], string> = {
  monthly: 'Ежемесячно',
  yearly: 'Ежегодно',
};

const STATUS_LABELS: Record<RecurringOperationResponse['status'], string> = {
  active: 'Активна',
  paused: 'Приостановлена',
};

function getPeriodicityLabel(
  periodicity: RecurringOperationResponse['periodicity'],
): string {
  return PERIODICITY_LABELS[periodicity] ?? periodicity;
}

function getStatusLabel(status: RecurringOperationResponse['status']): string {
  return STATUS_LABELS[status] ?? status;
}

type RecurringOperationItemProps = {
  readonly operation: RecurringOperationResponse;
};

function RecurringOperationItem({
  operation,
}: RecurringOperationItemProps): JSX.Element {
  const pauseMutation = usePauseRecurringOperation();
  const resumeMutation = useResumeRecurringOperation();

  const isIncome = operation.type === 'income';
  const sign = isIncome ? '+' : '-';
  const amountClass = isIncome ? styles.amountIncome : styles.amountExpense;
  const isMutating = pauseMutation.isPending || resumeMutation.isPending;

  const handleToggle = () => {
    const variables = { id: operation.id, propertyId: operation.property_id };
    if (operation.status === 'active') {
      pauseMutation.mutate(variables);
    } else {
      resumeMutation.mutate(variables);
    }
  };

  const actionText =
    operation.status === 'active' ? 'Приостановить' : 'Возобновить';

  return (
    <li className={styles.item}>
      <div className={styles.main}>
        <span className={styles.name}>{operation.name}</span>
        <span className={styles.meta}>
          {getCategoryLabel(operation.category)} ·{' '}
          {getPeriodicityLabel(operation.periodicity)} ·{' '}
          {formatOperationDate(operation.start_date)}
        </span>
      </div>
      <div className={styles.right}>
        <span className={`${styles.amount} ${amountClass}`}>
          {sign}
          {formatMoneyKopecks(operation.amount_kopecks, { round: true })}
        </span>
        <span className={styles.status}>{getStatusLabel(operation.status)}</span>
        <Button
          variant="secondary"
          size="small"
          loading={isMutating}
          onClick={handleToggle}
        >
          {actionText}
        </Button>
      </div>
    </li>
  );
}

export function RecurringOperationsTab(): JSX.Element {
  const {
    data,
    isLoading,
    isFetching,
    isError,
    refetch,
  } = useRecurringOperations();

  const items = data?.items ?? [];

  if (isLoading) {
    return <FinanceLoading />;
  }

  if (isError) {
    return <FinanceErrorState onRetry={refetch} isLoading={isFetching} />;
  }

  if (items.length === 0) {
    return (
      <FinanceEmptyState
        title="Нет регулярных операций"
        subtitle="Создайте регулярную операцию, чтобы платежи добавлялись автоматически."
        actionHref={ROUTES.financeCreateOperation}
        actionText="Добавить операцию"
      />
    );
  }

  return (
    <ul className={styles.list}>
      {items.map((operation) => (
        <RecurringOperationItem key={operation.id} operation={operation} />
      ))}
    </ul>
  );
}
