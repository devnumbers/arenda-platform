'use client';

import { useMemo, useState, type JSX } from 'react';
import { useParams } from 'next/navigation';
import { Plus, ArrowLeft } from '@/shared/assets/icons';
import { Icon } from '@/shared/ui/icon';
import { IconLink } from '@/shared/ui/icon-link';
import { Button } from '@/shared/ui/button';
import { LinkButton } from '@/shared/ui/link-button';
import { ROUTES } from '@/shared/config/routes';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import {
  useOperationsByProperty,
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
import { OperationsList } from '@/widgets/operations/ui/OperationsList';
import styles from './PropertyOperationsPage.module.css';

type Period = 'month' | 'quarter' | 'year';
type StatusFilter = 'all' | 'payments';

const STATUS_FILTERS: ReadonlyArray<{
  readonly key: StatusFilter;
  readonly label: string;
}> = [
  { key: 'all', label: 'Все' },
  { key: 'payments', label: 'Платежи' },
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
      ...dateRange,
      status: statusFilter === 'payments' ? ['pending', 'overdue'] : undefined,
    }),
    [dateRange, statusFilter],
  );

  const propertyQuery = useProperty(id);
  const summaryQuery = usePropertyOperationsSummary(id);
  const operationsQuery = useOperationsByProperty(id, filters);

  const isLoading =
    propertyQuery.isLoading || summaryQuery.isLoading || operationsQuery.isLoading;
  const isError =
    propertyQuery.isError || summaryQuery.isError || operationsQuery.isError;
  const isFetching =
    propertyQuery.isFetching || summaryQuery.isFetching || operationsQuery.isFetching;

  const handleRetry = () => {
    propertyQuery.refetch();
    summaryQuery.refetch();
    operationsQuery.refetch();
  };

  const propertyName = propertyQuery.data?.name ?? 'Мой объект';
  const operations = operationsQuery.data?.items ?? [];

  return (
    <div className={styles.root}>
      <header className={styles.header}>
        <div className={styles.headerLeft}>
          <IconLink
            href={ROUTES.property(id)}
            aria-label="Назад"
            icon={<ArrowLeft />}
          />
          <h1 className={styles.title}>{propertyName}</h1>
        </div>
        <LinkButton
          href={`${ROUTES.financeCreateOperation}?propertyId=${id}`}
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
      </header>

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
              actionHref={`${ROUTES.financeCreateOperation}?propertyId=${id}`}
              actionText="Добавить операцию"
            />
          ) : (
            <OperationsList items={operations} />
          )}
        </>
      )}
    </div>
  );
}
