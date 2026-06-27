'use client';

import { useMemo, type JSX } from 'react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import NextLink from 'next/link';
import { Plus } from '@/shared/assets/icons';
import { Icon } from '@/shared/ui/icon';
import { LinkButton } from '@/shared/ui/link-button';
import { ROUTES } from '@/shared/config/routes';
import { useOperations } from '@/features/operations/api/hooks';
import {
  formatDateForApi,
  startOfMonth,
  endOfMonth,
} from '@/entities/operation/lib/dates';
import { FinanceLoading } from '@/widgets/finance/ui/FinanceLoading';
import { FinanceErrorState } from '@/widgets/finance/ui/FinanceErrorState';
import { FinanceEmptyState } from '@/widgets/finance/ui/FinanceEmptyState';
import { OperationFilters } from './OperationFilters';
import { OperationsList } from './OperationsList';
import { RecurringOperationsTab } from './RecurringOperationsTab';
import { ProfitReport } from './ProfitReport';
import styles from './OperationsPage.module.css';

const TAB_ITEMS: ReadonlyArray<{
  readonly key: 'all' | 'income' | 'expense' | 'recurring' | 'profit';
  readonly label: string;
}> = [
  { key: 'all', label: 'Все' },
  { key: 'income', label: 'Доходы' },
  { key: 'expense', label: 'Расходы' },
  { key: 'recurring', label: 'Регулярные' },
  { key: 'profit', label: 'Прибыль' },
];

function getCurrentMonthRange(): { from: string; to: string } {
  const now = new Date();
  return {
    from: formatDateForApi(startOfMonth(now)),
    to: formatDateForApi(endOfMonth(now)),
  };
}

function buildTabHref(
  pathname: string,
  searchParams: URLSearchParams,
  tabKey: 'all' | 'income' | 'expense' | 'recurring' | 'profit',
): string {
  const params = new URLSearchParams(searchParams.toString());
  if (tabKey === 'all') {
    params.delete('type');
    params.delete('tab');
  } else if (tabKey === 'recurring' || tabKey === 'profit') {
    params.delete('type');
    params.set('tab', tabKey);
  } else {
    params.set('type', tabKey);
    params.delete('tab');
  }
  const query = params.toString();
  return query ? `${pathname}?${query}` : pathname;
}

export function OperationsPage(): JSX.Element {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const type = searchParams.get('type') as 'income' | 'expense' | null;
  const tab = searchParams.get('tab');
  const isRecurringTab = tab === 'recurring';
  const isProfitTab = tab === 'profit';
  const activeTabKey: 'all' | 'income' | 'expense' | 'recurring' | 'profit' = isProfitTab
    ? 'profit'
    : isRecurringTab
      ? 'recurring'
      : type ?? 'all';

  const status = useMemo(
    () => searchParams.getAll('status'),
    [searchParams],
  );
  const currentMonth = useMemo(() => getCurrentMonthRange(), []);
  const from = searchParams.get('from') ?? currentMonth.from;
  const to = searchParams.get('to') ?? currentMonth.to;

  const filters = useMemo(
    () => ({
      type: type ?? undefined,
      status,
      from,
      to,
      limit: 100,
    }),
    [type, status, from, to],
  );

  const {
    data,
    isLoading,
    isFetching,
    isError,
    refetch,
  } = useOperations(filters, { enabled: !isProfitTab });

  const handleFilterChange = (nextFilters: {
    from: string;
    to: string;
    status: ReadonlyArray<string>;
  }) => {
    const params = new URLSearchParams(searchParams.toString());

    params.set('from', nextFilters.from);
    params.set('to', nextFilters.to);

    params.delete('status');
    nextFilters.status.forEach((value) => params.append('status', value));

    const query = params.toString();
    router.replace(query ? `${pathname}?${query}` : pathname, { scroll: false });
  };

  const operations = data?.items ?? [];

  return (
    <div className={styles.root}>
      <div className={styles.header}>
        <h1 className={styles.title}>Операции</h1>
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

      <nav className={styles.tabs} aria-label="Тип операции">
        {TAB_ITEMS.map((tabItem) => {
          const isActive = activeTabKey === tabItem.key;
          return (
            <NextLink
              key={tabItem.key}
              href={buildTabHref(pathname, searchParams, tabItem.key)}
              className={`${styles.tab} ${isActive ? styles.tabActive : ''}`}
              aria-current={isActive ? 'page' : undefined}
            >
              {tabItem.label}
            </NextLink>
          );
        })}
      </nav>

      {isProfitTab ? (
        <ProfitReport />
      ) : isRecurringTab ? (
        <RecurringOperationsTab />
      ) : (
        <>
          {isLoading && <FinanceLoading />}

          {!isLoading && isError && (
            <FinanceErrorState onRetry={refetch} isLoading={isFetching} />
          )}

          {!isLoading && !isError && operations.length === 0 && (
            <FinanceEmptyState
              title="Нет операций"
              subtitle="Добавьте первую операцию, чтобы увидеть её в списке"
              actionHref={ROUTES.financeCreateOperation}
              actionText="Добавить операцию"
            />
          )}

          {!isLoading && !isError && operations.length > 0 && (
            <>
              <OperationFilters
                filters={{ from, to, status }}
                onChange={handleFilterChange}
              />
              <OperationsList items={operations} />
            </>
          )}
        </>
      )}
    </div>
  );
}
