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
  usePauseRecurringOperation,
  useResumeRecurringOperation,
} from '@/features/recurring-operations/api/hooks';
import { useSubscription } from '@/features/subscription/api/hooks';
import { isSubscriptionReadonly } from '@/features/subscription/lib/is-subscription-readonly';
import styles from './RecurringOperationListItem.module.css';

type RecurringOperationResponse = components['schemas']['RecurringOperationResponse'];

const PERIODICITY_LABELS: Record<RecurringOperationResponse['periodicity'], string> = {
  monthly: 'Ежемесячно',
  yearly: 'Ежегодно',
};

const STATUS_LABELS: Record<RecurringOperationResponse['status'], string> = {
  active: 'Активна',
  paused: 'Приостановлена',
};

function isSeriesEnded(endDate: string | null | undefined): boolean {
  if (!endDate) return false;
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  return new Date(endDate) < today;
}

export type RecurringOperationListItemProps = {
  readonly operation: RecurringOperationResponse;
};

export function RecurringOperationListItem({
  operation,
}: RecurringOperationListItemProps): JSX.Element {
  const pauseMutation = usePauseRecurringOperation();
  const resumeMutation = useResumeRecurringOperation();
  const { data: subscription } = useSubscription();
  const readonly = isSubscriptionReadonly(subscription);

  const isIncome = operation.type === 'income';
  const sign = isIncome ? '+' : '-';
  const amountClass = isIncome ? styles.amountIncome : styles.amountExpense;
  const isMutating = pauseMutation.isPending || resumeMutation.isPending;
  const ended = isSeriesEnded(operation.end_date);
  const canToggle = !ended && !readonly;

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
    <li className={styles.root}>
      <div className={styles.main}>
        <span className={styles.nameRow}>
          <span className={styles.name}>{operation.name}</span>
          <span className={styles.badge}>Регулярная</span>
        </span>
        <span className={styles.meta}>
          {getCategoryLabel(operation.category)} ·{' '}
          {PERIODICITY_LABELS[operation.periodicity]} ·{' '}
          {formatOperationDate(operation.start_date)}
          {ended && ' · Серия завершена'}
        </span>
      </div>
      <div className={styles.right}>
        <span className={`${styles.amount} ${amountClass}`}>
          {sign}
          {formatMoneyKopecks(operation.amount_kopecks, { round: true })}
        </span>
        <span className={styles.status}>{STATUS_LABELS[operation.status]}</span>
        <div className={styles.actions}>
          <LinkButton
            href={ROUTES.financeRecurringOperationEdit(operation.id)}
            variant="secondary"
            size="tiny"
          >
            Изменить серию
          </LinkButton>
          {canToggle && (
            <Button
              variant="secondary"
              size="tiny"
              loading={isMutating}
              disabled={readonly}
              onClick={handleToggle}
            >
              {actionText}
            </Button>
          )}
        </div>
      </div>
    </li>
  );
}
