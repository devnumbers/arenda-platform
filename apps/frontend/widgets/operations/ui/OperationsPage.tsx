'use client';

import { useMemo, type JSX } from 'react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import type { components } from '@/shared/api/generated';
import NextLink from 'next/link';
import { Plus } from '@/shared/assets/icons';
import { Icon } from '@/shared/ui/icon';
import { LinkButton } from '@/shared/ui/link-button';
import { ROUTES } from '@/shared/config/routes';
import { useOperations } from '@/features/operations/api/hooks';
import { useRecurringOperations } from '@/features/recurring-operations/api/hooks';
import {
  formatDateForApi,
  startOfMonth,
  endOfMonth,
} from '@/entities/operation/lib/dates';
import { FinanceLoading } from '@/widgets/finance/ui/FinanceLoading';
import { FinanceErrorState } from '@/widgets/finance/ui/FinanceErrorState';
import { FinanceEmptyState } from '@/widgets/finance/ui/FinanceEmptyState';
import { useProperties } from '@/features/properties/api';
import { OperationFilters } from './OperationFilters';
import { OperationsList } from './OperationsList';
import { SubscriptionReadonlyBanner } from '@/widgets/finance/ui/SubscriptionReadonlyBanner';
import { useSubscription } from '@/features/subscription/api/hooks';
import { isSubscriptionReadonly } from '@/features/subscription/lib/is-subscription-readonly';
import styles from './OperationsPage.module.css';

const TAB_ITEMS: ReadonlyArray<{
  readonly key: 'all' | 'income' | 'expense';
  readonly label: string;
}> = [
  { key: 'all', label: 'Все' },
  { key: 'income', label: 'Доходы' },
  { key: 'expense', label: 'Расходы' },
];

export type OperationKind = 'all' | 'onetime' | 'recurring';

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
  tabKey: 'all' | 'income' | 'expense',
): string {
  const params = new URLSearchParams(searchParams.toString());
  if (tabKey === 'all') {
    params.delete('type');
  } else {
    params.set('type', tabKey);
  }
  const query = params.toString();
  return query ? `${pathname}?${query}` : pathname;
}

function buildKindHref(
  pathname: string,
  searchParams: URLSearchParams,
  kind: OperationKind,
): string {
  const params = new URLSearchParams(searchParams.toString());
  if (kind === 'all') {
    params.delete('kind');
  } else {
    params.set('kind', kind);
  }
  const query = params.toString();
  return query ? `${pathname}?${query}` : pathname;
}

type UnifiedOperationItem =
  | { kind: 'onetime'; date: string; data: components['schemas']['OperationResponse'] }
  | { kind: 'recurring'; date: string; data: components['schemas']['RecurringOperationResponse'] };

export function OperationsPage(): JSX.Element {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const type = searchParams.get('type') as 'income' | 'expense' | null;
  const activeTabKey: 'all' | 'income' | 'expense' = type ?? 'all';
  const kind = (searchParams.get('kind') as OperationKind) ?? 'all';

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
    data: operationsData,
    isLoading: operationsLoading,
    isFetching: operationsFetching,
    isError: operationsError,
    refetch: refetchOperations,
  } = useOperations(filters);

  const {
    data: recurringData,
    isLoading: recurringLoading,
    isFetching: recurringFetching,
    isError: recurringError,
    refetch: refetchRecurring,
  } = useRecurringOperations();

  const {
    data: properties,
    isLoading: propertiesLoading,
    isError: propertiesError,
  } = useProperties();

  const hasProperties = !propertiesLoading && !propertiesError && (properties?.length ?? 0) > 0;
  const isLoading = operationsLoading || recurringLoading || propertiesLoading;
  const isError = operationsError || recurringError || propertiesError;
  const isFetching = operationsFetching || recurringFetching;

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

  const handleKindChange = (nextKind: OperationKind) => {
    const href = buildKindHref(pathname, searchParams, nextKind);
    router.replace(href, { scroll: false });
  };

  const filteredItems = useMemo<UnifiedOperationItem[]>(() => {
    const operations = operationsData?.items ?? [];
    const recurring = recurringData?.items ?? [];
    const items: UnifiedOperationItem[] = [];

    if (kind !== 'recurring') {
      operations.forEach((operation) => {
        if (kind === 'onetime' && operation.recurring_operation_id) {
          return;
        }
        items.push({ kind: 'onetime', date: operation.operation_date, data: operation });
      });
    }

    if (kind !== 'onetime') {
      recurring.forEach((operation) => {
        items.push({ kind: 'recurring', date: operation.start_date, data: operation });
      });
    }

    return items.sort((a, b) => {
      return new Date(b.date).getTime() - new Date(a.date).getTime();
    });
  }, [operationsData, recurringData, kind]);

  const { data: subscription } = useSubscription();
  const readonly = isSubscriptionReadonly(subscription);

  const showContent = !isLoading && !isError && hasProperties;

  return (
    <div className={styles.root}>
      <div className={styles.header}>
        <h1 className={styles.title}>Операции</h1>
        {!readonly && (
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
        )}
      </div>

      <SubscriptionReadonlyBanner />

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

      {isLoading && <FinanceLoading />}

      {!isLoading && isError && (
        <FinanceErrorState
          onRetry={() => {
            refetchOperations();
            refetchRecurring();
          }}
          isLoading={isFetching}
        />
      )}

      {!isLoading && !isError && !hasProperties && (
        <FinanceEmptyState
          title="Нет объектов"
          subtitle="Добавьте объект, чтобы создавать операции доходов и расходов."
          actionHref={readonly ? undefined : ROUTES.propertyNew}
          actionText={readonly ? undefined : 'Добавить объект'}
        />
      )}

      {showContent && (
        <OperationFilters
          filters={{ from, to, status, kind }}
          onChange={handleFilterChange}
          onKindChange={handleKindChange}
        />
      )}

      {showContent && (
        <OperationsList
          items={filteredItems}
          emptyState={
            <FinanceEmptyState
              title={kind === 'all' ? 'Нет операций' : 'Нет совпадений'}
              subtitle={
                kind === 'all'
                  ? 'Добавьте первую операцию, чтобы увидеть её в списке'
                  : 'Попробуйте изменить фильтры'
              }
              actionHref={readonly || kind !== 'all' ? undefined : ROUTES.financeCreateOperation}
              actionText={readonly || kind !== 'all' ? undefined : 'Добавить операцию'}
            />
          }
        />
      )}
    </div>
  );
}
