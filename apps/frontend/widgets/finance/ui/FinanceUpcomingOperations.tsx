'use client';

import { useMemo, type JSX } from 'react';
import { Skeleton } from '@heroui/react/skeleton';
import type { components } from '@/shared/api/generated';
import { ArrowRight } from '@/shared/assets/icons';
import { Icon } from '@/shared/ui/icon';
import { LinkButton } from '@/shared/ui/link-button';
import { OperationListItem } from '@/widgets/operations/ui/OperationListItem';
import { FinanceErrorState } from './FinanceErrorState';
import sectionStyles from './FinanceSection.module.css';
import styles from './FinanceUpcomingOperations.module.css';

type OperationResponse = components['schemas']['OperationResponse'];

interface FinanceOperationsPreviewProps {
  readonly title: string;
  readonly operations: OperationResponse[];
  readonly emptyText: string;
  readonly isLoading: boolean;
  readonly isFetching: boolean;
  readonly isError: boolean;
  readonly refetch: () => void;
  readonly actionHref?: string;
  readonly actionText?: string;
}

export function FinanceOperationsPreview({
  title,
  operations: rawOperations,
  emptyText,
  isLoading,
  isFetching,
  isError,
  refetch,
  actionHref,
  actionText = 'Смотреть все',
}: FinanceOperationsPreviewProps): JSX.Element {
  const operations = useMemo(() => {
    return [...rawOperations].sort(
      (a, b) => new Date(a.operation_date).getTime() - new Date(b.operation_date).getTime(),
    );
  }, [rawOperations]);

  if (isLoading) {
    return (
      <section className={sectionStyles.section}>
        <PreviewHeader title={title} actionHref={actionHref} actionText={actionText} />
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
        <PreviewHeader title={title} actionHref={actionHref} actionText={actionText} />
        <FinanceErrorState onRetry={refetch} isLoading={isFetching} />
      </section>
    );
  }

  return (
    <section className={sectionStyles.section}>
      <PreviewHeader title={title} actionHref={actionHref} actionText={actionText} />
      {operations.length === 0 ? (
        <p className={styles.upcomingEmpty}>{emptyText}</p>
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

function PreviewHeader({
  title,
  actionHref,
  actionText,
}: {
  readonly title: string;
  readonly actionHref?: string;
  readonly actionText: string;
}): JSX.Element {
  return (
    <div className={styles.header}>
      <h2 className={sectionStyles.sectionTitle}>{title}</h2>
      {actionHref && (
        <LinkButton
          href={actionHref}
          variant="clear"
          size="small"
          rightIcon={
            <Icon size="s">
              <ArrowRight />
            </Icon>
          }
          className={styles.headerLink}
        >
          {actionText}
        </LinkButton>
      )}
    </div>
  );
}

export function FinanceUpcomingOperations(
  props: Omit<FinanceOperationsPreviewProps, 'title' | 'emptyText'>,
): JSX.Element {
  return (
    <FinanceOperationsPreview
      {...props}
      title="Ближайшие операции"
      emptyText="Нет запланированных операций"
    />
  );
}
