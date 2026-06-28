'use client';

import type { JSX } from 'react';
import type { components } from '@/shared/api/generated';
import { ROUTES } from '@/shared/config/routes';
import { Button } from '@/shared/ui/button';
import { LinkButton } from '@/shared/ui/link-button';
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
import { SubscriptionReadonlyBanner } from '@/widgets/finance/ui/SubscriptionReadonlyBanner';
import { useSubscription } from '@/features/subscription/api/hooks';
import { isSubscriptionReadonly } from '@/features/subscription/lib/is-subscription-readonly';
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
  readonly readonly: boolean;
};

function RecurringOperationItem({
  operation,
  readonly,
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
        <LinkButton
          href={ROUTES.financeRecurringOperationEdit(operation.id)}
          variant="secondary"
          size="tiny"
          fullWidth
        >
          Изменить серию
        </LinkButton>
        <Button
          variant="secondary"
          size="small"
          loading={isMutating}
          disabled={readonly}
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
  const { data: subscription } = useSubscription();
  const readonly = isSubscriptionReadonly(subscription);

  const items = data?.items ?? [];

  if (isLoading) {
    return <FinanceLoading />;
  }

  if (isError) {
    return <FinanceErrorState onRetry={refetch} isLoading={isFetching} />;
  }

  if (items.length === 0) {
    return (
      <>
        <SubscriptionReadonlyBanner />
        <FinanceEmptyState
          title="Нет регулярных операций"
          subtitle="Создайте регулярную операцию, чтобы платежи добавлялись автоматически."
          actionHref={readonly ? undefined : ROUTES.financeCreateOperation}
          actionText={readonly ? undefined : 'Добавить операцию'}
        />
      </>
    );
  }

  return (
    <>
      <SubscriptionReadonlyBanner />
      <ul className={styles.list}>
        {items.map((operation) => (
          <RecurringOperationItem
            key={operation.id}
            operation={operation}
            readonly={readonly}
          />
        ))}
      </ul>
    </>
  );
}
