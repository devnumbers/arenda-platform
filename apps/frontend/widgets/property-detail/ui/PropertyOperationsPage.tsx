'use client';

import { useMemo, useState, type JSX } from 'react';
import { useParams } from 'next/navigation';
import { Plus } from '@/shared/assets/icons';
import { Icon } from '@/shared/ui/icon';
import { PageHeader } from '@/shared/ui/page-header';
import { Button } from '@/shared/ui/button';
import { LinkButton } from '@/shared/ui/link-button';
import { ROUTES } from '@/shared/config/routes';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import {
  useInfiniteOperations,
  usePropertyOperationsSummary,
} from '@/features/operations/api/hooks';
import { useProperty } from '@/features/properties/api/hooks';
import {
  formatDateForApi,
  startOfMonth,
  endOfMonth,
} from '@/entities/operation/lib/dates';
import { FinanceLoading } from '@/widgets/finance/ui/FinanceLoading';
import { FinanceErrorState } from '@/widgets/finance/ui/FinanceErrorState';
import { FinanceEmptyState } from '@/widgets/finance/ui/FinanceEmptyState';
import { SubscriptionReadonlyBanner } from '@/widgets/finance/ui/SubscriptionReadonlyBanner';
import { useSubscription } from '@/features/subscription/api/hooks';
import { isSubscriptionReadonly } from '@/features/subscription/lib/is-subscription-readonly';
import { OperationsList } from '@/widgets/operations/ui/OperationsList';
import styles from './PropertyOperationsPage.module.css';

type Period = 'month' | 'quarter' | 'year';
type StatusFilter = 'all' | 'actual';

const STATUS_FILTERS: ReadonlyArray<{
  readonly key: StatusFilter;
  readonly label: string;
}> = [
  { key: 'all', label: 'Все' },
  { key: 'actual', label: 'Актуальные' },
];

function getCurrentMonthRange(): { from: string; to: string } {
  const now = new Date();
  return {
    from: formatDateForApi(startOfMonth(now)),
    to: formatDateForApi(endOfMonth(now)),
  };
}

function getQuarterRange(date: Date): { from: string; to: string } {
  const quarter = Math.floor(date.getMonth() / 3);
  const from = new Date(date.getFullYear(), quarter * 3, 1);
  const to = new Date(date.getFullYear(), quarter * 3 + 3, 0);
  return { from: formatDateForApi(from), to: formatDateForApi(to) };
}

function getYearRange(date: Date): { from: string; to: string } {
  const from = new Date(date.getFullYear(), 0, 1);
  const to = new Date(date.getFullYear(), 11, 31);
  return { from: formatDateForApi(from), to: formatDateForApi(to) };
}

export function PropertyOperationsPage(): JSX.Element {
  const params = useParams<{ id: string }>();
  const id = params.id ?? '';

  const [period, setPeriod] = useState<Period>('month');
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all');

  const dateRange = useMemo(() => {
    const now = new Date();
    if (period === 'month') {
      return getCurrentMonthRange();
    }
    if (period === 'quarter') {
      return getQuarterRange(now);
    }
    return getYearRange(now);
  }, [period]);

  const filters = useMemo(
    () => ({
      property_id: id,
      ...dateRange,
      status: statusFilter === 'actual' ? ['pending', 'overdue'] : undefined,
      limit: 50,
    }),
    [id, dateRange, statusFilter],
  );

  const propertyQuery = useProperty(id);
  const summaryQuery = usePropertyOperationsSummary(id);
  const operationsQuery = useInfiniteOperations(filters, { enabled: Boolean(id) });

  const isLoading =
    propertyQuery.isLoading || summaryQuery.isLoading || operationsQuery.isLoading;
  const isInitialOperationsError = operationsQuery.isError && !operationsQuery.data;
  const isError =
    propertyQuery.isError || summaryQuery.isError || isInitialOperationsError;
  const isFetching =
    propertyQuery.isFetching || summaryQuery.isFetching || operationsQuery.isFetching;

  const handleRetry = () => {
    propertyQuery.refetch();
    summaryQuery.refetch();
    operationsQuery.refetch();
  };

  const propertyName = propertyQuery.data?.name ?? 'Мой объект';
  const isArchived = propertyQuery.data?.status === 'archived';
  const operations = operationsQuery.data?.pages.flatMap((page) => page.items) ?? [];
  const { data: subscription, isPending: isSubscriptionPending } = useSubscription();
  const readonly = isSubscriptionPending || isSubscriptionReadonly(subscription);

  return (
    <div className={styles.root}>
      <PageHeader
        title={propertyName}
        backHref={ROUTES.property(id)}
        actions={
          !readonly && (
            <LinkButton
              href={`${ROUTES.financeCreateOperation}?propertyId=${id}`}
              variant="primary"
              size="small"
              disabled={isArchived}
              title={isArchived ? 'Объект в архиве' : undefined}
              leftIcon={
                <Icon size="s">
                  <Plus />
                </Icon>
              }
            >
              Добавить операцию
            </LinkButton>
          )
        }
      />

      <SubscriptionReadonlyBanner />

      {isLoading && <FinanceLoading />}

      {!isLoading && isError && (
        <FinanceErrorState onRetry={handleRetry} isLoading={isFetching} />
      )}

      {!isLoading && !isError && (
        <>
          <section className={styles.summary} aria-label="Сводка">
            <div className={styles.summaryCard}>
              <span className={styles.summaryLabel}>Прибыль за месяц</span>
              <span className={styles.summaryValue}>
                {formatMoneyKopecks(summaryQuery.data?.monthly_profit_kopecks ?? 0)}
              </span>
            </div>
            <div className={styles.summaryCard}>
              <span className={styles.summaryLabel}>Прибыль за всё время</span>
              <span className={styles.summaryValue}>
                {formatMoneyKopecks(summaryQuery.data?.all_time_profit_kopecks ?? 0)}
              </span>
            </div>
            <div className={styles.summaryCard}>
              <span className={styles.summaryLabel}>Просрочено</span>
              <span className={styles.summaryValue}>
                {summaryQuery.data?.overdue_total_count ?? 0}
              </span>
            </div>
          </section>

          <div className={styles.filters}>
            <div className={styles.periodGroup} role="group" aria-label="Период">
              {(['month', 'quarter', 'year'] as const).map((key) => (
                <Button
                  key={key}
                  variant={period === key ? 'secondary' : 'icon-black'}
                  size="small"
                  onClick={() => setPeriod(key)}
                >
                  {key === 'month' && 'Месяц'}
                  {key === 'quarter' && 'Квартал'}
                  {key === 'year' && 'Год'}
                </Button>
              ))}
            </div>

            <div className={styles.chipGroup} role="group" aria-label="Фильтр по статусу">
              {STATUS_FILTERS.map((chip) => (
                <Button
                  key={chip.key}
                  variant={statusFilter === chip.key ? 'secondary' : 'icon-black'}
                  size="small"
                  onClick={() => setStatusFilter(chip.key)}
                >
                  {chip.label}
                </Button>
              ))}
            </div>
          </div>

          {operations.length === 0 ? (
            <FinanceEmptyState
              title="Нет операций"
              subtitle="Добавьте первую операцию, чтобы увидеть её в списке"
              actionHref={readonly ? undefined : `${ROUTES.financeCreateOperation}?propertyId=${id}`}
              actionText={readonly ? undefined : 'Добавить операцию'}
              actionDisabled={isArchived}
            />
          ) : (
            <>
              <OperationsList operations={operations} />

              {(operationsQuery.hasNextPage || operationsQuery.isFetchNextPageError) && (
                <div className={styles.loadMore}>
                  <Button
                    variant="secondary"
                    size="medium"
                    loading={operationsQuery.isFetchingNextPage}
                    onClick={() => {
                      void operationsQuery.fetchNextPage();
                    }}
                  >
                    {operationsQuery.isFetchNextPageError ? 'Повторить' : 'Показать ещё'}
                  </Button>
                  {operationsQuery.isFetchNextPageError && (
                    <span className={styles.loadMoreError} role="alert">
                      Не удалось загрузить следующие операции
                    </span>
                  )}
                </div>
              )}
            </>
          )}
        </>
      )}
    </div>
  );
}
