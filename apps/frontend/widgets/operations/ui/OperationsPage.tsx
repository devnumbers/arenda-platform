'use client';

import {type JSX, useMemo} from 'react';
import {usePathname, useRouter, useSearchParams} from 'next/navigation';
import NextLink from 'next/link';
import {Plus} from '@/shared/assets/icons';
import {PageHeader} from '@/shared/ui/page-header';
import {Icon} from '@/shared/ui/icon';
import {Button} from '@/shared/ui/button';
import {LinkButton} from '@/shared/ui/link-button';
import {ROUTES} from '@/shared/config/routes';
import {type OperationsFilters, useInfiniteOperations,} from '@/features/operations/api/hooks';
import {
    endOfMonth,
    endOfQuarter,
    endOfYear,
    formatDateForApi,
    parseDateForApi,
    startOfMonth,
    startOfQuarter,
    startOfYear,
} from '@/entities/operation/lib/dates';
import {FinanceLoading} from '@/widgets/finance/ui/FinanceLoading';
import {FinanceErrorState} from '@/widgets/finance/ui/FinanceErrorState';
import {FinanceEmptyState} from '@/widgets/finance/ui/FinanceEmptyState';
import {useArchivedProperties, useProperties} from '@/features/properties/api';
import {OperationFilters, type OperationFiltersState, type OperationPeriod,} from './OperationFilters';
import {OperationsList} from './OperationsList';
import {SubscriptionReadonlyBanner} from '@/widgets/finance/ui/SubscriptionReadonlyBanner';
import {useSubscription} from '@/features/subscription/api/hooks';
import {isSubscriptionReadonly} from '@/features/subscription/lib/is-subscription-readonly';
import styles from './OperationsPage.module.css';

type OperationTab = 'all' | 'income' | 'expense';
type OperationType = Exclude<OperationTab, 'all'>;
type OperationListSort = NonNullable<OperationsFilters['sort']>;
type PeriodRange = {
    readonly from: string;
    readonly to: string;
};
type ResolvedOperationPeriod = OperationFiltersState & {
    readonly isDefaultPeriod: boolean;
};

const DEFAULT_OPERATION_SORT: OperationListSort = 'operation_date_desc';

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

function buildTabHref(
    pathname: string,
    searchParams: { toString: () => string },
    tabKey: OperationTab,
): string {
    const params = new URLSearchParams(searchParams.toString());
    if (tabKey === 'all') {
        params.delete('type');
    } else {
        params.set('type', tabKey);
    }
    return buildHref(pathname, params);
}

function getOperationType(value: string | null): OperationType | undefined {
    if (value === 'income' || value === 'expense') {
        return value;
    }
    return undefined;
}

function getOperationPeriod(value: string | null): OperationPeriod | undefined {
    if (
        value === 'all' ||
        value === 'month' ||
        value === 'quarter' ||
        value === 'year' ||
        value === 'custom'
    ) {
        return value;
    }
    return undefined;
}

function getOperationSort(value: string | null): OperationListSort | undefined {
    if (value === 'operation_date_desc' || value === 'operation_date_asc') {
        return value;
    }
    return undefined;
}

function getMonthRange(date: Date): PeriodRange {
    return {
        from: formatDateForApi(startOfMonth(date)),
        to: formatDateForApi(endOfMonth(date)),
    };
}

function getQuarterRange(date: Date): PeriodRange {
    return {
        from: formatDateForApi(startOfQuarter(date)),
        to: formatDateForApi(endOfQuarter(date)),
    };
}

function getYearRange(date: Date): PeriodRange {
    return {
        from: formatDateForApi(startOfYear(date)),
        to: formatDateForApi(endOfYear(date)),
    };
}

function getPeriodRange(
    period: Exclude<OperationPeriod, 'all' | 'custom'>,
    date: Date,
): PeriodRange {
    if (period === 'month') return getMonthRange(date);
    if (period === 'quarter') return getQuarterRange(date);
    return getYearRange(date);
}

function isValidRange(from: string | null, to: string | null): boolean {
    const fromDate = parseDateForApi(from);
    const toDate = parseDateForApi(to);
    return Boolean(fromDate && toDate && fromDate <= toDate);
}

function isSameRange(first: PeriodRange, second: PeriodRange): boolean {
    return first.from === second.from && first.to === second.to;
}

function resolveOperationPeriod(searchParams: {
    get: (name: string) => string | null;
}): ResolvedOperationPeriod {
    const now = new Date();
    const currentMonthRange = getMonthRange(now);
    const requestedPeriod = getOperationPeriod(searchParams.get('period'));

    if (requestedPeriod === 'all') {
        return {period: 'all', status: [], isDefaultPeriod: false};
    }

    const requestedFrom = searchParams.get('from');
    const requestedTo = searchParams.get('to');

    if (requestedPeriod === 'custom') {
        if (isValidRange(requestedFrom, requestedTo)) {
            return {
                period: 'custom',
                from: requestedFrom ?? undefined,
                to: requestedTo ?? undefined,
                status: [],
                isDefaultPeriod: false,
            };
        }

        return {
            period: 'month',
            from: currentMonthRange.from,
            to: currentMonthRange.to,
            status: [],
            isDefaultPeriod: true,
        };
    }

    if (requestedPeriod) {
        const range = isValidRange(requestedFrom, requestedTo)
            ? {from: requestedFrom ?? currentMonthRange.from, to: requestedTo ?? currentMonthRange.to}
            : getPeriodRange(requestedPeriod, now);

        return {
            period: requestedPeriod,
            from: range.from,
            to: range.to,
            status: [],
            isDefaultPeriod: requestedPeriod === 'month' && isSameRange(range, currentMonthRange),
        };
    }

    return {
        period: 'month',
        from: currentMonthRange.from,
        to: currentMonthRange.to,
        status: [],
        isDefaultPeriod: true,
    };
}

export function OperationsPage(): JSX.Element {
    const router = useRouter();
    const pathname = usePathname();
    const searchParams = useSearchParams();

    const type = getOperationType(searchParams.get('type'));
    const activeTabKey: OperationTab = type ?? 'all';
    const propertyId = searchParams.get('property_id') || undefined;
    const resolvedPeriod = useMemo(
        () => resolveOperationPeriod(searchParams),
        [searchParams],
    );
    const period = resolvedPeriod.period;
    const from = resolvedPeriod.from;
    const to = resolvedPeriod.to;
    const sort = getOperationSort(searchParams.get('sort'));

    const status = useMemo(
        () => searchParams.getAll('status'),
        [searchParams],
    );

    const filters = useMemo<Omit<OperationsFilters, 'offset'>>(() => {
        const nextFilters: Omit<OperationsFilters, 'offset'> = {
            limit: 50,
        };

        if (type) {
            nextFilters.type = type;
        }

        if (status.length > 0) {
            nextFilters.status = status;
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

        nextFilters.sort = sort ?? DEFAULT_OPERATION_SORT;

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

    const propertyNameById = useMemo<ReadonlyMap<string, string>>(() => {
        return new Map(
            properties.map((property) => [property.id, property.name] as const),
        );
    }, [properties]);

    const operations = operationsData?.pages.flatMap((page) => page.items) ?? [];
    const hasProperties = properties.length > 0;
    const propertiesLoading = activePropertiesLoading || archivedPropertiesLoading;
    const propertiesError = activePropertiesError || archivedPropertiesError;
    const isInitialOperationsError = operationsError && !operationsData;
    const isLoading = operationsLoading || propertiesLoading;
    const isError = isInitialOperationsError || propertiesError;
    const showContent = !isLoading && !isError && hasProperties;
    const hasActiveFilters =
        activeTabKey !== 'all' ||
        Boolean(propertyId) ||
        !resolvedPeriod.isDefaultPeriod ||
        status.length > 0;

    const {data: subscription} = useSubscription();
    const readonly = isSubscriptionReadonly(subscription);

    const resetFiltersHref = buildHref(pathname, new URLSearchParams());

    const handleResetFilters = () => {
        router.replace(resetFiltersHref, {scroll: false});
    };

    const handleFilterChange = (nextFilters: OperationFiltersState) => {
        const params = new URLSearchParams(searchParams.toString());

        if (nextFilters.period === 'all') {
            params.set('period', 'all');
            params.delete('from');
            params.delete('to');
        } else {
            params.set('period', nextFilters.period);
            if (nextFilters.from) {
                params.set('from', nextFilters.from);
            } else {
                params.delete('from');
            }
            if (nextFilters.to) {
                params.set('to', nextFilters.to);
            } else {
                params.delete('to');
            }
        }

        params.delete('status');
        nextFilters.status.forEach((value) => params.append('status', value));

        router.replace(buildHref(pathname, params), {scroll: false});
    };

    return (
        <div className={styles.root}>
            <PageHeader
                title="Операции"
                actions={
                    !readonly && (
                        <LinkButton
                            href={ROUTES.financeCreateOperation}
                            variant="primary"
                            size="small"
                            leftIcon={
                                <Icon size="s">
                                    <Plus/>
                                </Icon>
                            }
                        >
                            Добавить операцию
                        </LinkButton>
                    )
                }
            />

            <SubscriptionReadonlyBanner/>

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

            {isLoading && <FinanceLoading/>}

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

            {showContent && (
                <OperationFilters
                    filters={{period, from, to, status}}
                    hasExternalFilters={activeTabKey !== 'all' || Boolean(propertyId)}
                    isDefaultPeriod={resolvedPeriod.isDefaultPeriod}
                    onChange={handleFilterChange}
                    onReset={handleResetFilters}
                />
            )}

            {showContent && propertyId && (
                <div className={styles.appliedFilters}>
          <span className={styles.appliedFilter}>
            Объект: {propertyNameById.get(propertyId) ?? 'Выбранный объект'}
          </span>
                </div>
            )}

            {showContent && (
                <>
                    <OperationsList
                        operations={operations}
                        showProperty
                        propertyNameById={propertyNameById}
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
                                        ? resetFiltersHref
                                        : readonly
                                            ? undefined
                                            : ROUTES.financeCreateOperation
                                }
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
