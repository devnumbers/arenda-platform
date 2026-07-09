'use client';

import {type JSX, useCallback, useMemo, useState} from 'react';
import {usePathname, useRouter} from 'next/navigation';
import {useProperties} from '@/features/properties/api/hooks';
import {useSubscription} from '@/features/subscription/api/hooks';
import {PageHeader} from '@/shared/ui/page-header';
import {ROUTES} from '@/shared/config/routes';
import {usePropertyListData} from '../lib/use-property-list-data';
import {applyFiltersAndSort, type PropertiesViewMode} from '../lib/apply-filters';
import {DEFAULT_PROPERTY_SORT} from '../lib/parse-property-search-params';
import {PropertiesToolbar} from './PropertiesToolbar';
import {PropertyCreateButton} from './PropertyCreateButton';
import {PropertyCard} from './PropertyCard';
import {PropertiesEmptyState} from './PropertiesEmptyState';
import {PropertiesLoading} from './PropertiesLoading';
import {PropertiesErrorState} from './PropertiesErrorState';
import {PropertiesArchiveLink} from './PropertiesArchiveLink';
import type {PropertyFilters, PropertySort} from '../lib/filter-types';
import styles from './PropertiesPage.module.css';

export type PropertiesPageProps = {
    readonly mode?: PropertiesViewMode;
    readonly initialFilters?: PropertyFilters;
    readonly initialSort?: PropertySort;
};

export function PropertiesPage({mode = 'active', initialFilters, initialSort}: PropertiesPageProps): JSX.Element {
    const {data, isLoading, isFetching, isError, refetch} = usePropertyListData(mode);
    const {data: activeProperties} = useProperties();
    const subscriptionQuery = useSubscription();
    const router = useRouter();
    const pathname = usePathname();

    const [filters, setFilters] = useState<PropertyFilters>(initialFilters ?? {types: [], statuses: []});
    const [sort, setSort] = useState<PropertySort>(initialSort ?? DEFAULT_PROPERTY_SORT);

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

            if (nextSort !== DEFAULT_PROPERTY_SORT) {
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

    const canAdd = useMemo(() => {
        if (subscriptionQuery.isLoading || !subscriptionQuery.data) return false;
        if (activeProperties === undefined) return false;
        const limit = subscriptionQuery.data.tariff.activePropertyLimit;
        if (limit < 0) return true;
        return activeProperties.length < limit;
    }, [subscriptionQuery.isLoading, subscriptionQuery.data, activeProperties]);

    return (
        <div className={styles.root}>
            <PageHeader
                title={mode === 'archived' ? 'Архивные объекты' : 'Мои объекты'}
                backHref={mode === 'archived' ? ROUTES.properties : undefined}
                actions={<PropertyCreateButton canAdd={canAdd}/>}
            />

            <PropertiesToolbar mode={mode} filters={filters} sort={sort} onChange={handleChange}/>

            {isLoading && <PropertiesLoading/>}

            {!isLoading && isError && <PropertiesErrorState onRetry={refetch} isLoading={isFetching}/>}

            {!isLoading && !isError && isEmpty && <PropertiesEmptyState canAdd={canAdd}/>}

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
