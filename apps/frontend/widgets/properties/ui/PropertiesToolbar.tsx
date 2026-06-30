'use client';

import type {JSX, ReactNode} from 'react';
import {useCallback, useMemo, useState} from 'react';
import NextLink from 'next/link';
import {Tooltip} from '@heroui/react';
import {Button} from '@/shared/ui/button';
import {Icon} from '@/shared/ui/icon';
import {Cancel, Filter, HomeAdd} from '@/shared/assets/icons';
import {ROUTES} from '@/shared/config/routes';
import {propertyTypeOptions} from '@/features/properties/lib/property-types';
import {statusFilterOptions, type StatusFilterValue,} from '@/features/properties/lib/property-statuses';
import type {PropertyType} from '@/entities/property/model/types';
import {FilterPopover} from './FilterPopover';
import {FilterDrawer} from './FilterDrawer';
import {type PropertyFilters, type PropertySort, sortOptions, toggleValue,} from '../lib/filter-types';
import type {PropertiesViewMode} from '../lib/apply-filters';
import styles from './PropertiesToolbar.module.css';

export type PropertiesToolbarProps = {
    readonly filters: PropertyFilters;
    readonly sort: PropertySort;
    readonly mode?: PropertiesViewMode;
    readonly onChange: (filters: PropertyFilters, sort: PropertySort) => void;
    readonly canAdd?: boolean;
};

type TypeFilterSectionProps = {
    readonly selected: readonly PropertyType[];
    readonly onToggle: (type: PropertyType) => void;
    readonly layout: 'popover' | 'drawer';
};

type StatusFilterSectionProps = {
    readonly selected: readonly StatusFilterValue[];
    readonly onToggle: (status: StatusFilterValue) => void;
    readonly layout: 'popover' | 'drawer';
};

type SortSectionProps = {
    readonly selected: PropertySort;
    readonly onSelect: (sort: PropertySort) => void;
    readonly layout: 'popover' | 'drawer';
};

function FilterOption({
                          label,
                          selected,
                          onClick,
                          layout,
                      }: {
    readonly label: string;
    readonly selected: boolean;
    readonly onClick: () => void;
    readonly layout: 'popover' | 'drawer';
}): JSX.Element {
    if (layout === 'popover') {
        return (
            <Button
                className={styles.popoverOption}
                variant={selected ? 'secondary' : 'icon-black'}
                size="small"
                fullWidth
                onClick={onClick}
            >
                {label}
            </Button>
        );
    }

    return (
        <Button
            variant={selected ? 'primary' : 'secondary'}
            size="small"
            onClick={onClick}
        >
            {label}
        </Button>
    );
}

function FilterGroup({
                         title,
                         children,
                     }: {
    readonly title: string;
    readonly children: ReactNode;
}): JSX.Element {
    return (
        <fieldset className={styles.group}>
            <legend className={styles.groupTitle}>{title}</legend>
            <div className={styles.options}>{children}</div>
        </fieldset>
    );
}

function TypeFilterSection({
                               selected,
                               onToggle,
                               layout,
                           }: TypeFilterSectionProps): JSX.Element {
    return (
        <FilterGroup title="Тип объекта">
            {propertyTypeOptions.map((option) => (
                <FilterOption
                    key={option.value}
                    label={option.label}
                    selected={selected.includes(option.value)}
                    onClick={() => onToggle(option.value)}
                    layout={layout}
                />
            ))}
        </FilterGroup>
    );
}

function StatusFilterSection({
                                 selected,
                                 onToggle,
                                 layout,
                             }: StatusFilterSectionProps): JSX.Element {
    return (
        <FilterGroup title="Статус">
            {statusFilterOptions.map((option) => (
                <FilterOption
                    key={option.value}
                    label={option.label}
                    selected={selected.includes(option.value)}
                    onClick={() => onToggle(option.value)}
                    layout={layout}
                />
            ))}
        </FilterGroup>
    );
}

function SortSection({
                         selected,
                         onSelect,
                         layout,
                     }: SortSectionProps): JSX.Element {
    return (
        <FilterGroup title="Сортировка">
            <div className={styles.options} role="radiogroup" aria-label="Сортировка">
                {sortOptions.map((option) => (
                    <FilterOption
                        key={option.value}
                        label={option.label}
                        selected={selected === option.value}
                        onClick={() => onSelect(option.value)}
                        layout={layout}
                    />
                ))}
            </div>
        </FilterGroup>
    );
}

type SelectedChip = {
    readonly key: string;
    readonly label: string;
    readonly onRemove: () => void;
};

export function PropertiesToolbar({
                                      filters,
                                      sort,
                                      mode = 'active',
                                      onChange,
                                      canAdd = true,
                                  }: PropertiesToolbarProps): JSX.Element {
    const [draftFilters, setDraftFilters] = useState<PropertyFilters>(filters);
    const [draftSort, setDraftSort] = useState<PropertySort>(sort);
    const [isDrawerOpen, setIsDrawerOpen] = useState(false);

    const openDrawer = useCallback(() => {
        setDraftFilters(filters);
        setDraftSort(sort);
        setIsDrawerOpen(true);
    }, [filters, sort]);

    const handleApply = useCallback(() => {
        onChange(draftFilters, draftSort);
        setIsDrawerOpen(false);
    }, [draftFilters, draftSort, onChange]);

    const handleDesktopTypeToggle = useCallback(
        (type: PropertyType) => {
            onChange({...filters, types: toggleValue(filters.types, type)}, sort);
        },
        [filters, sort, onChange],
    );

    const handleDesktopStatusToggle = useCallback(
        (status: StatusFilterValue) => {
            onChange(
                {...filters, statuses: toggleValue(filters.statuses, status)},
                sort,
            );
        },
        [filters, sort, onChange],
    );

    const handleDesktopSortSelect = useCallback(
        (nextSort: PropertySort) => {
            onChange(filters, nextSort);
        },
        [filters, onChange],
    );

    const handleDraftTypeToggle = useCallback((type: PropertyType) => {
        setDraftFilters((prev) => ({
            ...prev,
            types: toggleValue(prev.types, type),
        }));
    }, []);

    const handleDraftStatusToggle = useCallback((status: StatusFilterValue) => {
        setDraftFilters((prev) => ({
            ...prev,
            statuses: toggleValue(prev.statuses, status),
        }));
    }, []);

    const handleDraftSortSelect = useCallback((nextSort: PropertySort) => {
        setDraftSort(nextSort);
    }, []);

    const selectedChips = useMemo<readonly SelectedChip[]>(() => {
        const chips: SelectedChip[] = [];

        filters.types.forEach((value) => {
            const option = propertyTypeOptions.find((item) => item.value === value);
            if (!option) return;
            chips.push({
                key: `type-${value}`,
                label: `Тип: ${option.label}`,
                onRemove: () =>
                    onChange(
                        {...filters, types: filters.types.filter((v) => v !== value)},
                        sort,
                    ),
            });
        });

        filters.statuses.forEach((value) => {
            const option = statusFilterOptions.find((item) => item.value === value);
            if (!option) return;
            chips.push({
                key: `status-${value}`,
                label: `Статус: ${option.label}`,
                onRemove: () =>
                    onChange(
                        {
                            ...filters,
                            statuses: filters.statuses.filter((v) => v !== value),
                        },
                        sort,
                    ),
            });
        });

        return chips;
    }, [filters, sort, onChange]);

    const hasSelectedFilters =
        filters.types.length > 0 || filters.statuses.length > 0;

    const handleReset = useCallback(() => {
        onChange({types: [], statuses: []}, sort);
    }, [onChange, sort]);

    return (
        <div className={styles.root}>
            <div className={styles.row}>
                <div className={styles.desktopOnly}>
                    <FilterPopover
                        label="Тип объекта"
                        activeCount={filters.types.length}
                    >
                        <TypeFilterSection
                            selected={filters.types}
                            onToggle={handleDesktopTypeToggle}
                            layout="popover"
                        />
                    </FilterPopover>
                    {mode !== 'archived' && (
                        <FilterPopover
                            label="Статус"
                            activeCount={filters.statuses.length}
                        >
                            <StatusFilterSection
                                selected={filters.statuses}
                                onToggle={handleDesktopStatusToggle}
                                layout="popover"
                            />
                        </FilterPopover>
                    )}
                    <FilterPopover label="Сортировка" activeCount={0}>
                        <SortSection
                            selected={sort}
                            onSelect={handleDesktopSortSelect}
                            layout="popover"
                        />
                    </FilterPopover>
                </div>

                <div className={styles.mobileOnly}>
                    <Button
                        className={styles.mobileButton}
                        variant="secondary"
                        size="small"
                        leftIcon={
                            <Icon size="s">
                                <Filter/>
                            </Icon>
                        }
                        onClick={openDrawer}
                    >
                        Фильтры
                    </Button>
                    <FilterDrawer
                        isOpen={isDrawerOpen}
                        onClose={() => setIsDrawerOpen(false)}
                        onApply={handleApply}
                    >
                        <TypeFilterSection
                            selected={draftFilters.types}
                            onToggle={handleDraftTypeToggle}
                            layout="drawer"
                        />
                        {mode !== 'archived' && (
                            <StatusFilterSection
                                selected={draftFilters.statuses}
                                onToggle={handleDraftStatusToggle}
                                layout="drawer"
                            />
                        )}
                        <SortSection
                            selected={draftSort}
                            onSelect={handleDraftSortSelect}
                            layout="drawer"
                        />
                    </FilterDrawer>
                </div>

                {canAdd ? (
                    <NextLink
                        href={ROUTES.propertyNew}
                        className={styles.addButton}
                        aria-label="Добавить объект"
                    >
                        <Icon size="s">
                            <HomeAdd/>
                        </Icon>
                    </NextLink>
                ) : (
                    <Tooltip>
                        <Tooltip.Trigger>
                            <span
                                className={styles.addButtonDisabled}
                                role="button"
                                aria-label="Добавить объект (достигнут лимит)"
                                aria-disabled="true"
                            >
                                <Icon size="s">
                                    <HomeAdd/>
                                </Icon>
                            </span>
                        </Tooltip.Trigger>
                        <Tooltip.Content>
                            Достигнут лимит объектов по тарифу
                        </Tooltip.Content>
                    </Tooltip>
                )}
            </div>

            {hasSelectedFilters && (
                <div className={styles.chipsRow}>
                    <div className={styles.chips}>
                        {selectedChips.map((chip) => (
                            <Button
                                key={chip.key}
                                className={styles.chip}
                                variant="secondary"
                                size="tiny"
                                rightIcon={
                                    <Icon size="xs">
                                        <Cancel/>
                                    </Icon>
                                }
                                onClick={chip.onRemove}
                            >
                                {chip.label}
                            </Button>
                        ))}
                    </div>
                    <button
                        type="button"
                        className={styles.reset}
                        onClick={handleReset}
                    >
                        Сбросить
                    </button>
                </div>
            )}
        </div>
    );
}
