'use client';

import { useParams, useRouter } from 'next/navigation';
import { useState, type JSX } from 'react';
import { Modal } from '@heroui/react';
import type { components } from '@/shared/api/generated';
import { ROUTES } from '@/shared/config/routes';
import { Button } from '@/shared/ui/button';
import { PageHeader } from '@/shared/ui/page-header';
import {
  useCompleteOperation,
  useDeleteOperation,
  useMarkOperationIncomplete,
  useOperation,
} from '@/features/operations/api/hooks';
import { useProperty } from '@/features/properties/api';
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
import { OperationActionMenu } from './OperationActionMenu';
import styles from './OperationDetailPage.module.css';

type OperationResponse = components['schemas']['OperationResponse'];
type OperationStatus = components['schemas']['OperationStatus'];

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

function formatReminderLabel(offsetDays: number | null | undefined): string {
  if (offsetDays === null || offsetDays === undefined) {
    return 'Нет напоминания';
  }
  return `Напоминание за ${offsetDays} ${offsetDays === 1 ? 'день' : 'дня'}`;
}

type DeleteOperationModalProps = {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onConfirm: () => void;
  readonly isLoading: boolean;
};

function DeleteOperationModal({
  isOpen,
  onClose,
  onConfirm,
  isLoading,
}: DeleteOperationModalProps): JSX.Element {
  const handleOpenChange = (open: boolean): void => {
    if (!open) {
      onClose();
    }
  };

  return (
    <Modal>
      <Modal.Backdrop isOpen={isOpen} onOpenChange={handleOpenChange}>
        <Modal.Container placement="center" size="sm">
          <Modal.Dialog aria-label="Удалить операцию">
            <Modal.Header>
              <Modal.Heading>Удалить операцию?</Modal.Heading>
            </Modal.Header>
            <Modal.Body>
              <p>Операция будет удалена безвозвратно.</p>
            </Modal.Body>
            <Modal.Footer>
              <Button variant="secondary" onClick={onClose} type="button">
                Отменить
              </Button>
              <Button
                variant="primary"
                onClick={onConfirm}
                type="button"
                loading={isLoading}
              >
                Удалить
              </Button>
            </Modal.Footer>
          </Modal.Dialog>
        </Modal.Container>
      </Modal.Backdrop>
    </Modal>
  );
}

function OperationDetailCard({
  operation,
  propertyName,
  isArchived,
}: {
  readonly operation: OperationResponse;
  readonly propertyName?: string;
  readonly isArchived: boolean;
}): JSX.Element {
  const isIncome = operation.type === 'income';
  const sign = isIncome ? '+' : '-';
  const amountClass = isIncome ? styles.amountIncome : styles.amountExpense;
  const statusVariant = getStatusVariant(operation.status);
  const statusClass = statusVariant ? STATUS_VARIANT_CLASS[statusVariant] : '';

  return (
    <section className={styles.card}>
      <div className={styles.cardHeader}>
        <h2 className={styles.name}>{operation.name}</h2>
        <span className={`${styles.status} ${statusClass}`}>
          {getOperationStatusLabel(operation.status)}
        </span>
      </div>

      <div className={styles.amountRow}>
        <span className={`${styles.amount} ${amountClass}`}>
          {sign}
          {formatMoneyKopecks(operation.amount_kopecks, { round: true })}
        </span>
      </div>

      {isArchived && (
        <p className={styles.banner}>
          Объект в архиве, операция только для просмотра.
        </p>
      )}

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
          <dd className={styles.detailValue}>{propertyName ?? operation.property_id}</dd>
        </div>
        <div className={styles.detailRow}>
          <dt className={styles.detailLabel}>Напоминание</dt>
          <dd className={styles.detailValue}>
            {formatReminderLabel(operation.reminder_offset_days)}
          </dd>
        </div>
        {operation.comment && (
          <div className={styles.detailRow}>
            <dt className={styles.detailLabel}>Комментарий</dt>
            <dd className={styles.detailValue}>{operation.comment}</dd>
          </div>
        )}
      </dl>
    </section>
  );
}

export function OperationDetailPage(): JSX.Element {
  const id = useOperationId();
  const router = useRouter();
  const { data, isLoading, isError, refetch, isFetching } = useOperation(id ?? '');
  const { data: subscription } = useSubscription();
  const readonly = isSubscriptionReadonly(subscription);

  const completeMutation = useCompleteOperation();
  const markIncompleteMutation = useMarkOperationIncomplete();
  const deleteMutation = useDeleteOperation();
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const { data: property } = useProperty(data?.property_id ?? '');

  const propertyName = property?.name;
  const isArchived = property?.status === 'archived';
  const viewOnly = readonly || isArchived;

  const handleComplete = () => {
    if (!data) return;
    completeMutation.mutate({ id: data.id, propertyId: data.property_id });
  };

  const handleMarkIncomplete = () => {
    if (!data) return;
    markIncompleteMutation.mutate({ id: data.id, propertyId: data.property_id });
  };

  const handleEdit = () => {
    if (!data) return;
    router.push(ROUTES.financeOperationEdit(data.id));
  };

  const handleDelete = () => setIsDeleteModalOpen(true);

  const handleConfirmDelete = () => {
    if (!data) return;
    deleteMutation.mutate(
      { id: data.id, propertyId: data.property_id },
      {
        onSuccess: () => {
          setIsDeleteModalOpen(false);
          router.push(
            data.lease_id
              ? ROUTES.lease(data.lease_id)
              : ROUTES.financeOperations,
          );
        },
      },
    );
  };

  const actionMenu = (
    <OperationActionMenu
      type={data?.type}
      status={data?.status}
      disabled={isLoading || isError || !data || viewOnly}
      onComplete={handleComplete}
      onMarkIncomplete={handleMarkIncomplete}
      onEdit={handleEdit}
      onDelete={handleDelete}
    />
  );

  return (
    <div className={styles.root}>
      <PageHeader title="Операция" backHref={ROUTES.financeOperations} actions={actionMenu} />
      <SubscriptionReadonlyBanner />
      {!id && (
        <FinanceErrorState onRetry={() => router.push(ROUTES.financeOperations)} isLoading={false} />
      )}
      {id && isLoading && <FinanceLoading />}
      {id && !isLoading && isError && (
        <FinanceErrorState onRetry={refetch} isLoading={isFetching} />
      )}
      {id && !isLoading && !isError && data && (
        <OperationDetailCard operation={data} propertyName={propertyName} isArchived={isArchived} />
      )}
      <DeleteOperationModal
        isOpen={isDeleteModalOpen}
        onClose={() => setIsDeleteModalOpen(false)}
        onConfirm={handleConfirmDelete}
        isLoading={deleteMutation.isPending}
      />
    </div>
  );
}
