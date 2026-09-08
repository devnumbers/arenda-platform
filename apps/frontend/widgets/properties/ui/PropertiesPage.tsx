'use client';

import {type JSX, useCallback, useMemo, useState} from 'react';
import {usePathname, useRouter} from 'next/navigation';
import {useArchivedProperties, useProperties, usePropertiesWithMeta} from '@/features/properties';
import {useSubscription} from '@/features/subscription';
import {Add, ArrowLeft} from '@/shared/assets/icons';
import {goBack} from '@/shared/lib/navigation';
import {IconButton, PageContent, TopNav, TopNavTitle} from '@/shared/ui/design';
import {ROUTES} from '@/shared/config/routes';
import {applyFiltersAndSort, type PropertiesViewMode} from '../lib/apply-filters';
import {DEFAULT_PROPERTY_SORT} from '../lib/parse-property-search-params';
import {formatHiddenSharedFootnote} from '../lib/format-hidden-shared-footnote';
import {PropertiesToolbar} from './PropertiesToolbar';
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
    const propertiesQuery = useProperties({enabled: mode === 'active'});
    const archivedPropertiesQuery = useArchivedProperties({enabled: mode === 'archived'});
    const listQuery = mode === 'archived' ? archivedPropertiesQuery : propertiesQuery;
    const {data, isLoading, isFetching, isError, refetch} = listQuery;
    const {data: activeProperties} = useProperties();
    // Shares the /properties request with useProperties via the shared
    // propertyKeys.list prefix; surfaces how many shared objects are hidden
    // from the recipient by a tariff slot shortage.
    const metaQuery = usePropertiesWithMeta({enabled: mode === 'active'});
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

    const hiddenSharedCount = metaQuery.data?.hiddenSharedCount ?? 0;
    const showHiddenSharedNote =
        mode === 'active'
        && !isLoading
        && !isError
        && !metaQuery.isLoading
        && hiddenSharedCount > 0;

    const isActionLoading = subscriptionQuery.isPending || activeProperties === undefined;

    const canAdd = useMemo(() => {
        if (!subscriptionQuery.data || activeProperties === undefined) return false;
        const limit = subscriptionQuery.data.tariff.activePropertyLimit;
        if (limit < 0) return true;
        return activeProperties.length < limit;
    }, [subscriptionQuery.data, activeProperties]);

    // Кнопка создания на каноне хаба (как «Создать задачу» #525): «+» в
    // ряду заголовка на списке и в trailing-слоте хедера на архиве. При
    // исчерпанном лимите тарифа ведёт на смену тарифа (объяснение — в
    // имени для screen reader и пустом состоянии списка).
    const createButton = isActionLoading
        ? <IconButton icon={<Add/>} label="Создать объект" disabled/>
        : canAdd
            ? <IconButton icon={<Add/>} label="Создать объект" onClick={() => router.push(ROUTES.propertyNew)}/>
            : <IconButton
                icon={<Add/>}
                label="Достигнут лимит объектов по тарифу — сменить тариф"
                onClick={() => router.push(ROUTES.profileTariffChange)}
            />;

    const content = (
        <div className={styles.root}>
            <PropertiesToolbar mode={mode} filters={filters} sort={sort} onChange={handleChange}/>

            {isLoading && <PropertiesLoading/>}

            {!isLoading && isError && <PropertiesErrorState onRetry={() => void refetch()} isLoading={isFetching}/>}

            {!isLoading && !isError && isEmpty && <PropertiesEmptyState canAdd={canAdd} isLoading={isActionLoading}/>}

            {!isLoading && !isError && !isEmpty && (
                <ul className={styles.list}>
                    {visible.map((property) => (
                        <li key={property.id}>
                            <PropertyCard property={property}/>
                        </li>
                    ))}
                </ul>
            )}

            {showHiddenSharedNote && (
                <p className={styles.hiddenSharedNote}>
                    {formatHiddenSharedFootnote(hiddenSharedCount)}
                </p>
            )}

            {mode === 'active' && !isLoading && !isError && <PropertiesArchiveLink/>}
        </div>
    );

    if (mode === 'archived') {
        return (
            <>
                <TopNav
                    leading={
                        <IconButton
                            icon={<ArrowLeft/>}
                            label="Назад"
                            onClick={() => goBack(router, ROUTES.properties)}
                        />
                    }
                    trailing={createButton}
                >
                    <TopNavTitle title="Архивные объекты"/>
                </TopNav>

                <PageContent>{content}</PageContent>
            </>
        );
    }

    return (
        <>
            {/* Хаб-шапка: «крылья» (лого + профиль) и на мобайле, поведение
             * стандартное — в потоке на мобайле, закреплена на десктопе. */}
            <TopNav mobileWings/>

            <PageContent>
                <div className="flex items-center justify-between pr-3.5 pl-6">
                    <h1 className="text-[28px] font-semibold leading-8 text-content">Объекты</h1>
                    {createButton}
                </div>

                {content}
            </PageContent>
        </>
    );
}
