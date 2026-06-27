'use client';

import { useParams, useRouter } from 'next/navigation';
import type { JSX } from 'react';
import type { components } from '@/shared/api/generated';
import { ROUTES } from '@/shared/config/routes';
import { Button } from '@/shared/ui/button';
import { IconLink } from '@/shared/ui/icon-link';
import { Icon } from '@/shared/ui/icon';
import { ArrowLeft } from '@/shared/assets/icons';
import {
  useCompleteOperation,
  useDeleteOperation,
  useOperation,
} from '@/features/operations/api/hooks';
import { getCategoryLabel } from '@/entities/operation/lib/categories';
import {
  getOperationStatusLabel,
  operationStatusOptions,
} from '@/entities/operation/lib/statuses';
import { formatOperationDate } from '@/entities/operation/lib/dates';
import { formatMoneyKopecks } from '@/entities/operation/lib/formatMoney';
import { FinanceLoading } from '@/widgets/finance/ui/FinanceLoading';
import { FinanceErrorState } from '@/widgets/finance/ui/FinanceErrorState';
import { SubscriptionReadonlyBanner } from '@/widgets/finance/ui/SubscriptionReadonlyBanner';
import { useSubscription } from '@/features/subscription/api/hooks';
import { isSubscriptionReadonly } from '@/features/subscription/lib/is-subscription-readonly';
import styles from './OperationDetailPage.module.css';

type OperationResponse = components['schemas']['OperationResponse'];
type OperationStatus = components['schemas']['OperationStatus'];
type OperationType = components['schemas']['OperationType'];

const TYPE_LABELS: Record<OperationType, string> = {
  income: 'Доход',
  expense: 'Расход',
};

const STATUS_VARIANT_CLASS: Record<
  NonNullable<ReturnType<typeof getStatusVariant>>,
  string
> = {
  warning: styles.statusWarning,
  danger: styles.statusDanger,
  success: styles.statusSuccess,
  default: styles.statusDefault,
};

function getStatusVariant(status: OperationStatus) {
  return operationStatusOptions.find((option) => option.value === status)
    ?.variant;
}

function useOperationId(): string | undefined {
  const params = useParams<{ readonly id: string }>();
  return params?.id;
}

function OperationDetailCard({
  operation,
  readonly,
}: {
  readonly operation: OperationResponse;
  readonly readonly: boolean;
}): JSX.Element {
  const router = useRouter();
  const completeMutation = useCompleteOperation();
  const deleteMutation = useDeleteOperation();

  const isIncome = operation.type === 'income';
  const sign = isIncome ? '+' : '-';
  const amountClass = isIncome ? styles.amountIncome : styles.amountExpense;
  const statusVariant = getStatusVariant(operation.status);
  const statusClass = statusVariant ? STATUS_VARIANT_CLASS[statusVariant] : '';
  const canComplete = operation.status === 'pending' || operation.status === 'overdue';

  const handleComplete = () => {
    completeMutation.mutate({
      id: operation.id,
      propertyId: operation.property_id,
    });
  };

  const handleEdit = () => {
    router.push(ROUTES.financeOperationEdit(operation.id));
  };

  const handleDelete = () => {
    if (!confirm('Удалить операцию?')) {
      return;
    }
    deleteMutation.mutate(
      { id: operation.id, propertyId: operation.property_id },
      { onSuccess: () => router.push(ROUTES.financeOperations) },
    );
  };

  return (
    <section className={styles.card}>
      <div className={styles.cardHeader}>
        <h1 className={styles.name}>{operation.name}</h1>
        <span className={`${styles.status} ${statusClass}`}>
          {getOperationStatusLabel(operation.status)}
        </span>
      </div>

      <div className={styles.amountRow}>
        <span className={`${styles.amount} ${amountClass}`}>
          {sign}
          {formatMoneyKopecks(operation.amount_kopecks, { round: true })}
        </span>
        <span className={styles.type}>{TYPE_LABELS[operation.type]}</span>
      </div>

      <dl className={styles.details}>
        <div className={styles.detailRow}>
          <dt className={styles.detailLabel}>Дата</dt>
          <dd className={styles.detailValue}>
            {formatOperationDate(operation.operation_date)}
          </dd>
        </div>
        <div className={styles.detailRow}>
          <dt className={styles.detailLabel}>Категория</dt>
          <dd className={styles.detailValue}>
            {getCategoryLabel(operation.category)}
          </dd>
        </div>
        <div className={styles.detailRow}>
          <dt className={styles.detailLabel}>Объект</dt>
          <dd className={styles.detailValue}>Объект: {operation.property_id}</dd>
        </div>
        {operation.comment && (
          <div className={styles.detailRow}>
            <dt className={styles.detailLabel}>Комментарий</dt>
            <dd className={styles.detailValue}>{operation.comment}</dd>
          </div>
        )}
      </dl>

      <div className={styles.actions}>
        {canComplete && (
          <Button
            variant="primary"
            size="medium"
            loading={completeMutation.isPending}
            disabled={readonly}
            onClick={handleComplete}
          >
            {isIncome ? 'Отметить полученной' : 'Отметить оплаченной'}
          </Button>
        )}
        <Button
          variant="secondary"
          size="medium"
          disabled={readonly}
          onClick={handleEdit}
        >
          Редактировать
        </Button>
        <Button
          variant="icon-black"
          size="medium"
          loading={deleteMutation.isPending}
          disabled={readonly}
          onClick={handleDelete}
        >
          Удалить
        </Button>
      </div>
    </section>
  );
}

export function OperationDetailPage(): JSX.Element {
  const id = useOperationId();
  const router = useRouter();
  const { data, isLoading, isError, refetch, isFetching } = useOperation(id ?? '');
  const { data: subscription } = useSubscription();
  const readonly = isSubscriptionReadonly(subscription);

  if (!id) {
    return (
      <FinanceErrorState
        onRetry={() => router.push(ROUTES.financeOperations)}
        isLoading={false}
      />
    );
  }

  return (
    <div className={styles.root}>
      <div className={styles.header}>
        <IconLink
          href={ROUTES.financeOperations}
          variant="icon-black"
          size="medium"
          icon={
            <Icon size="m">
              <ArrowLeft />
            </Icon>
          }
          aria-label="Назад к списку операций"
        />
        <span className={styles.title}>Операция</span>
      </div>

      <SubscriptionReadonlyBanner />

      {isLoading && <FinanceLoading />}

      {!isLoading && isError && (
        <FinanceErrorState onRetry={refetch} isLoading={isFetching} />
      )}

      {!isLoading && !isError && data && (
        <OperationDetailCard operation={data} readonly={readonly} />
      )}
    </div>
  );
}
