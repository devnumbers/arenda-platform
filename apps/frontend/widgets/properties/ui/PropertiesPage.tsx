'use client';

import {type JSX, useCallback, useMemo, useState} from 'react';
import {usePathname, useRouter} from 'next/navigation';
import {useArchivedProperties, useProperties, usePropertiesWithMeta} from '@/features/properties';
import {useSubscription} from '@/features/subscription';
import {Add, ArrowLeft, Search} from '@/shared/assets/icons';
import {goBack} from '@/shared/lib/navigation';
import {useKeyboardActivation} from '@/shared/lib/hooks/useKeyboardActivation';
import {HubCollapseAnchor, HubTitle, IconButton, PageContent, TopNav, TopNavTitle} from '@/shared/ui/design';
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

    const openCreate = useCallback(() => {
        // При исчерпанном лимите тарифа «+» ведёт на смену тарифа
        // (объяснение — в имени для screen reader и пустом состоянии списка).
        if (canAdd) router.push(ROUTES.propertyNew);
        else router.push(ROUTES.profileTariffChange);
    }, [canAdd, router]);

    const createButton = (
        <IconButton
            icon={<Add/>}
            label={isActionLoading ? 'Создать объект' : canAdd ? 'Создать объект' : 'Достигнут лимит объектов по тарифу — сменить тариф'}
            disabled={isActionLoading}
            onClick={openCreate}
        />
    );

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
             * стандартное — в потоке на мобайле, закреплена на десктопе.
             * Компакт-бар: лупа поиска в левом слоте (у хаба есть пилюля,
             * #601), заголовок по центру. */}
            <TopNav
                mobileWings
                collapse={{
                    title: 'Объекты',
                    search: { href: ROUTES.propertySearch, label: 'Найти объект' },
                }}
            />

            <PageContent>
                <HubCollapseAnchor>
                    <HubTitle>Объекты</HubTitle>
                    {/* Пилюля поиска с «+» создания (макет 1603:89183,
                     * канон книги контактов #600): видна всегда, вне фазы
                     * загрузки — контент встаёт на её место без сдвига
                     * (§7). */}
                    <div className="mt-4 mb-4 px-6">
                        <PropertySearchPill
                            onOpenSearch={() => router.push(ROUTES.propertySearch)}
                            onCreate={openCreate}
                            disabled={isActionLoading}
                            createLabel={canAdd ? 'Создать объект' : 'Достигнут лимит объектов по тарифу — сменить тариф'}
                        />
                    </div>
                </HubCollapseAnchor>

                {content}
            </PageContent>
        </>
    );
}

/**
 * Поисковая пилюля хаба «Объекты» (макет 1603:89183, Search Button
 * 1758:105271; структура — пилюля книги контактов): серая пилюля 56px,
 * лупа слева, плейсхолдер «Найти объект»; тап открывает поисковую
 * страницу /properties/search, «+» справа — создание (кнопка внутри
 * строки-кнопки — паттерн useKeyboardActivation, DESIGN.md §6). При
 * исчерпанном лимите тарифа «+» ведёт на смену тарифа.
 */
export function PropertySearchPill({
    onOpenSearch,
    onCreate,
    disabled = false,
    createLabel = 'Создать объект',
}: {
    readonly onOpenSearch: () => void;
    readonly onCreate: () => void;
    readonly disabled?: boolean;
    readonly createLabel?: string;
}): JSX.Element {
    const activatorProps = useKeyboardActivation({ onSelect: onOpenSearch });

    return (
        <div
            {...activatorProps}
            className="flex h-14 w-full cursor-pointer items-center rounded-pill bg-surface-muted pl-[18px] pr-2 text-left outline-none transition-opacity hover:opacity-90 focus-visible:ring-4 focus-visible:ring-primary active:opacity-90"
        >
            <Search className="h-6 w-6 shrink-0 text-content" aria-hidden/>
            <span className="min-w-0 flex-1 truncate px-2 text-base font-medium text-content">
                Найти объект
            </span>
            <IconButton
                icon={<Add/>}
                label={createLabel}
                disabled={disabled}
                onClick={(event) => {
                    event.stopPropagation();
                    onCreate();
                }}
            />
        </div>
    );
}
