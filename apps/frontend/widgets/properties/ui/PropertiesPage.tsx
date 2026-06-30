'use client';

import {type JSX, useCallback, useEffect, useMemo, useState} from 'react';
import {usePathname, useRouter, useSearchParams} from 'next/navigation';
import {propertyTypeOptions} from '@/features/properties/lib/property-types';
import type {StatusFilterValue} from '@/features/properties/lib/property-statuses';
import {statusFilterOptions} from '@/features/properties/lib/property-statuses';
import {useSubscription} from '@/features/subscription/api/hooks';
import {usePropertyListData} from '../lib/use-property-list-data';
import {applyFiltersAndSort, type PropertiesViewMode} from '../lib/apply-filters';
import {PropertiesToolbar} from './PropertiesToolbar';
import {PropertyCard} from './PropertyCard';
import {PropertiesEmptyState} from './PropertiesEmptyState';
import {PropertiesLoading} from './PropertiesLoading';
import {PropertiesErrorState} from './PropertiesErrorState';
import {PropertiesArchiveLink} from './PropertiesArchiveLink';
import type {PropertyFilters, PropertySort} from '../lib/filter-types';
import type {PropertyType} from '@/entities/property/model/types';
import styles from './PropertiesPage.module.css';

export type PropertiesPageProps = {
    readonly mode?: PropertiesViewMode;
};

const initialSort: PropertySort = 'name_asc';

const validPropertyTypes = new Set<PropertyType>(propertyTypeOptions.map((option) => option.value));
const validStatusValues = new Set<StatusFilterValue>(statusFilterOptions.map((option) => option.value));

function parseFiltersFromSearchParams(searchParams: URLSearchParams): PropertyFilters {
    const rawTypes = searchParams.get('types')?.split(',') ?? [];
    const rawStatuses = searchParams.get('statuses')?.split(',') ?? [];

    return {
        types: rawTypes.filter((value): value is PropertyType => validPropertyTypes.has(value as PropertyType)),
        statuses: rawStatuses.filter((value): value is StatusFilterValue =>
            validStatusValues.has(value as StatusFilterValue),
        ),
    };
}

function parseSortFromSearchParams(searchParams: URLSearchParams): PropertySort {
    const value = searchParams.get('sort');
    return value === 'name_asc' || value === 'name_desc' ? value : initialSort;
}

export function PropertiesPage({mode = 'active'}: PropertiesPageProps): JSX.Element {
    const {data, isLoading, isFetching, isError, refetch} = usePropertyListData(mode);
    const subscriptionQuery = useSubscription();
    const router = useRouter();
    const pathname = usePathname();
    const searchParams = useSearchParams();

    const [filters, setFilters] = useState<PropertyFilters>(() => parseFiltersFromSearchParams(searchParams));
    const [sort, setSort] = useState<PropertySort>(() => parseSortFromSearchParams(searchParams));

    useEffect(() => {
        const nextFilters = parseFiltersFromSearchParams(searchParams);
        const nextSort = parseSortFromSearchParams(searchParams);
        let cancelled = false;
        queueMicrotask(() => {
            if (cancelled) return;
            setFilters(nextFilters);
            setSort(nextSort);
        });
        return () => {
            cancelled = true;
        };
    }, [searchParams]);

    const visible = useMemo(
        () => (data ? applyFiltersAndSort(data, mode, filters, sort) : []),
        [data, mode, filters, sort],
    );

    const updateUrl = useCallback(
        (nextFilters: PropertyFilters, nextSort: PropertySort) => {
            const params = new URLSearchParams();

            if (nextFilters.types.length > 0) {
                params.set('types', nextFilters.types.join(','));
            }

            if (mode !== 'archived' && nextFilters.statuses.length > 0) {
                params.set('statuses', nextFilters.statuses.join(','));
            }

            if (nextSort !== initialSort) {
                params.set('sort', nextSort);
            }

            const query = params.toString();
            router.replace(query ? `${pathname}?${query}` : pathname, {scroll: false});
        },
        [mode, pathname, router],
    );

    const handleChange = useCallback(
        (nextFilters: PropertyFilters, nextSort: PropertySort) => {
            setFilters(nextFilters);
            setSort(nextSort);
            updateUrl(nextFilters, nextSort);
        },
        [updateUrl],
    );

    const isEmpty = !isLoading && !isError && visible.length === 0;
    const title = mode === 'archived' ? 'Архивные объекты' : 'Мои объекты';

    const canAdd = useMemo(() => {
        if (!subscriptionQuery.data) return true;
        const limit = subscriptionQuery.data.tariff.activePropertyLimit;
        if (limit < 0) return true;
        return (data?.length ?? 0) < limit;
    }, [subscriptionQuery.data, data]);

    return (
        <div className={styles.root}>
            <h1 className={styles.title}>{title}</h1>
            <PropertiesToolbar mode={mode} filters={filters} sort={sort} onChange={handleChange} canAdd={canAdd}/>

            {isLoading && <PropertiesLoading/>}

            {!isLoading && isError && <PropertiesErrorState onRetry={refetch} isLoading={isFetching}/>}

            {!isLoading && !isError && isEmpty && <PropertiesEmptyState/>}

            {!isLoading && !isError && !isEmpty && (
                <ul className={styles.list}>
                    {visible.map((property) => (
                        <li key={property.id}>
                            <PropertyCard property={property}/>
                        </li>
                    ))}
                </ul>
            )}

            {mode === 'active' && !isLoading && !isError && <PropertiesArchiveLink/>}
        </div>
    );
}
