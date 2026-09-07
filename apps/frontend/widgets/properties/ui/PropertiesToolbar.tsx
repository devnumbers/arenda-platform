'use client';

import type {JSX, ReactNode} from 'react';
import {useCallback, useMemo, useState} from 'react';
import {Button} from '@/shared/ui/button';
import {Icon} from '@/shared/ui/icon';
import {Filter, SmallArrowDown, SmallArrowUp} from '@/shared/assets/icons';
import {propertyTypeOptions} from '@/features/properties';
import {statusFilterOptions, type StatusFilterValue,} from '@/features/properties';
import type {PropertyType} from '@/entities/property';
import {Select} from '@/shared/ui/select';
import {FilterDrawer} from './FilterDrawer';
import {type PropertyFilters, type PropertySort, sortOptions, toggleValue,} from '../lib/filter-types';
import type {PropertiesViewMode} from '../lib/apply-filters';
import styles from './PropertiesToolbar.module.css';

export type PropertiesToolbarProps = {
    readonly filters: PropertyFilters;
    readonly sort: PropertySort;
    readonly mode?: PropertiesViewMode;
    readonly onChange: (filters: PropertyFilters, sort: PropertySort) => void;
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
                         children,
                     }: {
    readonly children: ReactNode;
}): JSX.Element {
    return (
        <fieldset className={styles.group}>
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
        <FilterGroup>
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
        <FilterGroup>
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
        <FilterGroup>
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
                    <Select
                        label="Тип объекта"
                        multiple
                        value={filters.types}
                        options={propertyTypeOptions}
                        onChange={(nextTypes) => onChange({...filters, types: nextTypes}, sort)}
                        renderTrigger={({isOpen, onClick}) => (
                            <Button
                                variant={isOpen ? 'primary' : 'secondary'}
                                size="medium"
                                rightIcon={
                                    <Icon size="s">
                                        {isOpen ? <SmallArrowUp className="text-error"/> : <SmallArrowDown/>}
                                    </Icon>
                                }
                                onClick={onClick}
                            >
                                Тип объекта
                            </Button>
                        )}
                    />
                    {mode !== 'archived' && (
                        <Select
                            label="Статус"
                            multiple
                            value={filters.statuses}
                            options={statusFilterOptions}
                            onChange={(nextStatuses) => onChange({
                                ...filters,
                                statuses: nextStatuses
                            }, sort)}
                            renderTrigger={({isOpen, onClick}) => (
                                <Button
                                    variant={isOpen ? 'primary' : 'secondary'}
                                    size="medium"
                                    rightIcon={
                                        <Icon size="s">
                                            {isOpen ? <SmallArrowUp className="text-error"/> : <SmallArrowDown/>}
                                        </Icon>
                                    }
                                    onClick={onClick}
                                >
                                    Статус
                                </Button>
                            )}
                        />
                    )}
                    <Select
                        label="Сортировка"
                        value={sort}
                        options={sortOptions}
                        onChange={(nextSort) => onChange(filters, nextSort)}
                        renderTrigger={({isOpen, onClick}) => (
                            <Button
                                variant={isOpen ? 'primary' : 'secondary'}
                                size="medium"
                                rightIcon={
                                    <Icon size="s">
                                        {isOpen ? <SmallArrowUp className="text-error"/> : <SmallArrowDown/>}
                                    </Icon>
                                }
                                onClick={onClick}
                            >
                                Сортировка
                            </Button>
                        )}
                    />
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
