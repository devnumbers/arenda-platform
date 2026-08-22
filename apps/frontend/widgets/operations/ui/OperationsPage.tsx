'use client';

import {type JSX, type MouseEvent as ReactMouseEvent, useMemo, useState} from 'react';
import {usePathname, useRouter} from 'next/navigation';
import NextLink from 'next/link';
import clsx from 'clsx';
import {Plus} from '@/shared/assets/icons';
import {PageHeader} from '@/shared/ui/page-header';
import {Icon} from '@/shared/ui/icon';
import {Button} from '@/shared/ui/button';
import {ROUTES} from '@/shared/config/routes';
import {type OperationsFilters, useInfiniteOperations,} from '@/features/operations';
import {endOfMonth, formatDateForApi, startOfMonth,} from '@/entities/operation';
import {FinanceErrorState} from '@/shared/ui/finance-error-state';
import {FinanceEmptyState} from '@/shared/ui/finance-empty-state';
import {useArchivedProperties, useProperties} from '@/features/properties';
import {OperationFilters, type OperationFiltersState, type OperationPeriod,} from './OperationFilters';
import {OperationsList} from '@/entities/operation';
import {OperationsListLoading} from './OperationsListLoading';
import {SubscriptionReadonlyBanner} from '@/features/subscription';
import {useSubscription} from '@/features/subscription';
import {isSubscriptionReadonly} from '@/features/subscription';
import {
    DEFAULT_OPERATION_SORT,
    type OperationInitialFilters,
    type OperationListSort,
} from '../lib/parse-operation-search-params';
import styles from './OperationsPage.module.css';

type OperationTab = 'all' | 'income' | 'expense';
type OperationType = Exclude<OperationTab, 'all'>;

type FilterSnapshot = {
    readonly type: OperationType | undefined;
    readonly propertyId: string | undefined;
    readonly period: OperationPeriod;
    readonly from: string | undefined;
    readonly to: string | undefined;
    readonly status: ReadonlyArray<string>;
    readonly sort: OperationListSort;
};

const TAB_ITEMS: ReadonlyArray<{
    readonly key: OperationTab;
    readonly label: string;
}> = [
    {key: 'all', label: 'Все'},
    {key: 'income', label: 'Доходы'},
    {key: 'expense', label: 'Расходы'},
];

function buildHref(pathname: string, params: URLSearchParams): string {
    const query = params.toString();
    return query ? `${pathname}?${query}` : pathname;
}

function buildQuery(snapshot: FilterSnapshot): URLSearchParams {
    const params = new URLSearchParams();

    if (snapshot.type) {
        params.set('type', snapshot.type);
    }

    if (snapshot.propertyId) {
        params.set('property_id', snapshot.propertyId);
    }

    if (snapshot.period === 'all') {
        params.set('period', 'all');
    } else {
        params.set('period', snapshot.period);
        if (snapshot.from) {
            params.set('from', snapshot.from);
        }
        if (snapshot.to) {
            params.set('to', snapshot.to);
        }
    }

    snapshot.status.forEach((value) => params.append('status', value));

    if (snapshot.sort !== DEFAULT_OPERATION_SORT) {
        params.set('sort', snapshot.sort);
    }

    return params;
}

function getDefaultMonthPeriod(): {
    readonly period: OperationPeriod;
    readonly from: string;
    readonly to: string;
    readonly isDefaultPeriod: boolean;
} {
    const now = new Date();
    return {
        period: 'month',
        from: formatDateForApi(startOfMonth(now)),
        to: formatDateForApi(endOfMonth(now)),
        isDefaultPeriod: true,
    };
}

export type OperationsPageProps = {
    readonly initial: OperationInitialFilters;
};

export function OperationsPage({initial}: OperationsPageProps): JSX.Element {
    const router = useRouter();
    const pathname = usePathname();

    const [type, setType] = useState<OperationType | undefined>(initial.type);
    const [propertyId, setPropertyId] = useState<string | undefined>(initial.propertyId);
    const [periodState, setPeriodState] = useState({
        period: initial.period,
        from: initial.from,
        to: initial.to,
        isDefaultPeriod: initial.isDefaultPeriod,
    });
    const [status, setStatus] = useState<ReadonlyArray<string>>(initial.status);
    const [sort, setSort] = useState<OperationListSort>(initial.sort ?? DEFAULT_OPERATION_SORT);

    const activeTabKey: OperationTab = type ?? 'all';
    const {period, from, to, isDefaultPeriod} = periodState;

    const filters = useMemo<Omit<OperationsFilters, 'offset'>>(() => {
        const nextFilters: Omit<OperationsFilters, 'offset'> = {
            limit: 50,
        };

        if (type) {
            nextFilters.type = type;
        }

        if (status.length > 0) {
            nextFilters.status = [...status];
        }

        if (propertyId) {
            nextFilters.property_id = propertyId;
        }

        if (period !== 'all') {
            if (from) {
                nextFilters.from = from;
            }
            if (to) {
                nextFilters.to = to;
            }
        }

        nextFilters.sort = sort;

        return nextFilters;
    }, [type, status, propertyId, period, from, to, sort]);

    const {
        data: operationsData,
        isLoading: operationsLoading,
        isFetching: operationsFetching,
        isError: operationsError,
        refetch: refetchOperations,
        fetchNextPage,
        hasNextPage,
        isFetchNextPageError,
        isFetchingNextPage,
    } = useInfiniteOperations(filters);

    const {
        data: activeProperties,
        isLoading: activePropertiesLoading,
        isFetching: activePropertiesFetching,
        isError: activePropertiesError,
        refetch: refetchActiveProperties,
    } = useProperties();

    const {
        data: archivedProperties,
        isLoading: archivedPropertiesLoading,
        isFetching: archivedPropertiesFetching,
        isError: archivedPropertiesError,
        refetch: refetchArchivedProperties,
    } = useArchivedProperties();

    const properties = useMemo(() => {
        return [...(activeProperties ?? []), ...(archivedProperties ?? [])];
    }, [activeProperties, archivedProperties]);

    const operations = operationsData?.pages.flatMap((page) => page.items) ?? [];
    const hasProperties = properties.length > 0;
    const propertiesLoading = activePropertiesLoading || archivedPropertiesLoading;
    const propertiesError = activePropertiesError || archivedPropertiesError;
    const isInitialOperationsError = operationsError && !operationsData;
    const isLoading = operationsLoading || propertiesLoading;
    const isError = isInitialOperationsError || propertiesError;
    const hasActiveFilters =
        activeTabKey !== 'all' ||
        Boolean(propertyId) ||
        !isDefaultPeriod ||
        status.length > 0;

    const {data: subscription, isPending: isSubscriptionPending} = useSubscription();
    const readonly = isSubscriptionPending || isSubscriptionReadonly(subscription);

    const handleResetFilters = () => {
        setType(undefined);
        setPropertyId(undefined);
        setStatus([]);
        setSort(DEFAULT_OPERATION_SORT);
        setPeriodState(getDefaultMonthPeriod());
        router.replace(pathname, {scroll: false});
    };

    const handleFilterChange = (nextFilters: OperationFiltersState) => {
        const nextPeriod = {
            period: nextFilters.period,
            from: nextFilters.from,
            to: nextFilters.to,
            isDefaultPeriod: false,
        };
        setPeriodState(nextPeriod);
        setStatus(nextFilters.status);
        setPropertyId(nextFilters.propertyId);
        router.replace(
            buildHref(
                pathname,
                buildQuery({
                    type,
                    propertyId: nextFilters.propertyId,
                    period: nextFilters.period,
                    from: nextFilters.from,
                    to: nextFilters.to,
                    status: nextFilters.status,
                    sort,
                }),
            ),
            {scroll: false},
        );
    };

    return (
        <div className={styles.root}>
            <PageHeader
                title="Операции"
                backHref={ROUTES.finance}
                actions={
                    !readonly && (
                        <NextLink
                            href={ROUTES.financeCreateOperation}
                            className={styles.addButton}
                            aria-label="Добавить операцию"
                        >
                            <Icon size="l">
                                <Plus/>
                            </Icon>
                        </NextLink>
                    )
                }
            />

            <SubscriptionReadonlyBanner/>

            <nav className={styles.tabs} aria-label="Тип операции">
                {TAB_ITEMS.map((tabItem) => {
                    const isActive = activeTabKey === tabItem.key;
                    const nextType = tabItem.key === 'all' ? undefined : tabItem.key;
                    const href = buildHref(
                        pathname,
                        buildQuery({
                            type: nextType,
                            propertyId,
                            period,
                            from,
                            to,
                            status,
                            sort,
                        }),
                    );
                    return (
                        <NextLink
                            key={tabItem.key}
                            href={href}
                            onClick={(event: ReactMouseEvent<HTMLAnchorElement>) => {
                                if (
                                    event.defaultPrevented ||
                                    event.button !== 0 ||
                                    event.metaKey ||
                                    event.ctrlKey ||
                                    event.shiftKey ||
                                    event.altKey
                                ) {
                                    return;
                                }
                                event.preventDefault();
                                setType(nextType);
                                router.replace(href, {scroll: false});
                            }}
                            className={clsx(styles.tab, isActive && styles.tabActive)}
                            aria-current={isActive ? 'page' : undefined}
                        >
                            {tabItem.label}
                        </NextLink>
                    );
                })}
            </nav>

            <OperationFilters
                filters={{period, from, to, status, propertyId}}
                properties={activeProperties ?? []}
                propertiesLoading={activePropertiesLoading}
                hasExternalFilters={activeTabKey !== 'all' || Boolean(propertyId)}
                isDefaultPeriod={isDefaultPeriod}
                onChange={handleFilterChange}
                onReset={handleResetFilters}
            />

            {isLoading && <OperationsListLoading/>}

            {!isLoading && isError && (
                <FinanceErrorState
                    onRetry={() => {
                        void refetchOperations();
                        void refetchActiveProperties();
                        void refetchArchivedProperties();
                    }}
                    isLoading={
                        operationsFetching ||
                        activePropertiesFetching ||
                        archivedPropertiesFetching
                    }
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

            {!isLoading && !isError && hasProperties && (
                <>
                    <OperationsList
                        operations={operations}
                        emptyState={
                            <FinanceEmptyState
                                title={hasActiveFilters ? 'Нет совпадений' : 'Нет операций за период'}
                                subtitle={
                                    hasActiveFilters
                                        ? 'Попробуйте изменить фильтры'
                                        : 'Добавьте первую операцию, чтобы увидеть её в списке'
                                }
                                actionHref={
                                    hasActiveFilters
                                        ? undefined
                                        : readonly
                                            ? undefined
                                            : ROUTES.financeCreateOperation
                                }
                                actionOnClick={hasActiveFilters ? handleResetFilters : undefined}
                                actionText={
                                    hasActiveFilters
                                        ? 'Сбросить фильтры'
                                        : readonly
                                            ? undefined
                                            : 'Добавить операцию'
                                }
                            />
                        }
                    />

                    {(hasNextPage || isFetchNextPageError) && (
                        <div className={styles.loadMore}>
                            <Button
                                variant="secondary"
                                size="medium"
                                loading={isFetchingNextPage}
                                onClick={() => {
                                    void fetchNextPage();
                                }}
                            >
                                {isFetchNextPageError ? 'Повторить' : 'Показать ещё'}
                            </Button>
                            {isFetchNextPageError && (
                                <span className={styles.loadMoreError} role="alert">
                  Не удалось загрузить следующие операции
                </span>
                            )}
                        </div>
                    )}
                </>
            )}
        </div>
    );
}
