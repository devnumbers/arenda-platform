'use client';

import { useCallback, useState } from 'react';
import type { ComponentPropsWithoutRef, FC, JSX, ReactNode } from 'react';
import clsx from 'clsx';
import { Button as HeroButton } from '@heroui/react/button';
import { Filter } from '@/shared/assets/icons';
import { Icon } from '@/shared/ui/icon';
import { propertyTypeOptions } from '@/features/properties/lib/property-types';
import {
  statusFilterOptions,
  type StatusFilterValue,
} from '@/features/properties/lib/property-statuses';
import type { PropertyType } from '@/entities/property/model/types';
import { FilterPopover } from './FilterPopover';
import { FilterDrawer } from './FilterDrawer';
import {
  sortOptions,
  toggleValue,
  type PropertyFilters,
  type PropertySort,
} from '../lib/filter-types';
import styles from './PropertiesToolbar.module.css';

type HeroButtonProps = ComponentPropsWithoutRef<typeof HeroButton>;
type SortButtonProps = HeroButtonProps & { role?: string };
const SortButton = HeroButton as FC<SortButtonProps>;

export type PropertiesToolbarProps = {
  readonly filters: PropertyFilters;
  readonly sort: PropertySort;
  readonly onChange: (filters: PropertyFilters, sort: PropertySort) => void;
};

type TypeFilterSectionProps = {
  readonly selected: readonly PropertyType[];
  readonly onToggle: (type: PropertyType) => void;
};

type StatusFilterSectionProps = {
  readonly selected: readonly StatusFilterValue[];
  readonly onToggle: (status: StatusFilterValue) => void;
};

type SortSectionProps = {
  readonly selected: PropertySort;
  readonly onSelect: (sort: PropertySort) => void;
};

function OptionChip({
  label,
  selected,
  onClick,
}: {
  readonly label: string;
  readonly selected: boolean;
  readonly onClick: () => void;
}): JSX.Element {
  return (
    <HeroButton
      className={clsx(
        styles.optionChip,
        selected ? styles.optionChipSelected : styles.optionChipUnselected,
      )}
      type="button"
      variant="secondary"
      aria-pressed={selected}
      onClick={onClick}
    >
      {label}
    </HeroButton>
  );
}

function SortOption({
  label,
  selected,
  onClick,
}: {
  readonly label: string;
  readonly selected: boolean;
  readonly onClick: () => void;
}): JSX.Element {
  return (
    <SortButton
      className={clsx(
        styles.optionChip,
        selected ? styles.optionChipSelected : styles.optionChipUnselected,
      )}
      type="button"
      variant="secondary"
      role="radio"
      aria-checked={selected}
      onClick={onClick}
    >
      {label}
    </SortButton>
  );
}

function FilterGroup({ title, children }: { readonly title: string; readonly children: ReactNode }): JSX.Element {
  return (
    <fieldset className={styles.group}>
      <legend className={styles.groupTitle}>{title}</legend>
      <div className={styles.options}>{children}</div>
    </fieldset>
  );
}

function TypeFilterSection({ selected, onToggle }: TypeFilterSectionProps): JSX.Element {
  return (
    <FilterGroup title="Тип объекта">
      {propertyTypeOptions.map((option) => (
        <OptionChip
          key={option.value}
          label={option.label}
          selected={selected.includes(option.value)}
          onClick={() => onToggle(option.value)}
        />
      ))}
    </FilterGroup>
  );
}

function StatusFilterSection({ selected, onToggle }: StatusFilterSectionProps): JSX.Element {
  return (
    <FilterGroup title="Статус">
      {statusFilterOptions.map((option) => (
        <OptionChip
          key={option.value}
          label={option.label}
          selected={selected.includes(option.value)}
          onClick={() => onToggle(option.value)}
        />
      ))}
    </FilterGroup>
  );
}

function SortSection({ selected, onSelect }: SortSectionProps): JSX.Element {
  return (
    <div className={styles.group}>
      <span className={styles.groupTitle}>Сортировка</span>
      <div className={styles.options} role="radiogroup" aria-label="Сортировка">
        {sortOptions.map((option) => (
          <SortOption
            key={option.value}
            label={option.label}
            selected={selected === option.value}
            onClick={() => onSelect(option.value)}
          />
        ))}
      </div>
    </div>
  );
}

export function PropertiesToolbar({ filters, sort, onChange }: PropertiesToolbarProps): JSX.Element {
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
      onChange({ ...filters, types: toggleValue(filters.types, type) }, sort);
    },
    [filters, sort, onChange],
  );

  const handleDesktopStatusToggle = useCallback(
    (status: StatusFilterValue) => {
      onChange({ ...filters, statuses: toggleValue(filters.statuses, status) }, sort);
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
    setDraftFilters((prev) => ({ ...prev, types: toggleValue(prev.types, type) }));
  }, []);

  const handleDraftStatusToggle = useCallback((status: StatusFilterValue) => {
    setDraftFilters((prev) => ({ ...prev, statuses: toggleValue(prev.statuses, status) }));
  }, []);

  const handleDraftSortSelect = useCallback((nextSort: PropertySort) => {
    setDraftSort(nextSort);
  }, []);

  return (
    <div className={styles.root}>
      <div className={styles.desktopOnly}>
        <FilterPopover label="Тип объекта" activeCount={filters.types.length}>
          <div className={styles.popoverBody}>
            <TypeFilterSection selected={filters.types} onToggle={handleDesktopTypeToggle} />
          </div>
        </FilterPopover>
        <FilterPopover label="Статус" activeCount={filters.statuses.length}>
          <div className={styles.popoverBody}>
            <StatusFilterSection selected={filters.statuses} onToggle={handleDesktopStatusToggle} />
          </div>
        </FilterPopover>
        <FilterPopover label="Сортировка" activeCount={0}>
          <div className={styles.popoverBody}>
            <SortSection selected={sort} onSelect={handleDesktopSortSelect} />
          </div>
        </FilterPopover>
      </div>

      <div className={styles.mobileOnly}>
        <HeroButton
          className={styles.mobileButton}
          variant="secondary"
          size="sm"
          onClick={openDrawer}
        >
          <Icon size="s">
            <Filter />
          </Icon>
          Фильтры
        </HeroButton>
        <FilterDrawer
          isOpen={isDrawerOpen}
          onClose={() => setIsDrawerOpen(false)}
          onApply={handleApply}
        >
          <TypeFilterSection selected={draftFilters.types} onToggle={handleDraftTypeToggle} />
          <StatusFilterSection selected={draftFilters.statuses} onToggle={handleDraftStatusToggle} />
          <SortSection selected={draftSort} onSelect={handleDraftSortSelect} />
        </FilterDrawer>
      </div>
    </div>
  );
}
