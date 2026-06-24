'use client';

import { useState, type JSX } from 'react';
import { usePropertyListData } from '../lib/use-property-list-data';
import { applyFiltersAndSort, type PropertiesViewMode } from '../lib/apply-filters';
import { PropertiesToolbar } from './PropertiesToolbar';
import { PropertyCard } from './PropertyCard';
import { PropertiesEmptyState } from './PropertiesEmptyState';
import { PropertiesLoading } from './PropertiesLoading';
import { PropertiesArchiveLink } from './PropertiesArchiveLink';
import type { PropertyFilters, PropertySort } from '../lib/filter-types';
import styles from './PropertiesPage.module.css';

export type PropertiesPageProps = {
  readonly mode?: PropertiesViewMode;
};

const initialFilters: PropertyFilters = { types: [], statuses: [] };
const initialSort: PropertySort = 'name_asc';

export function PropertiesPage({ mode = 'active' }: PropertiesPageProps): JSX.Element {
  const { data, isLoading } = usePropertyListData();
  const [filters, setFilters] = useState<PropertyFilters>(initialFilters);
  const [sort, setSort] = useState<PropertySort>(initialSort);

  const visible = data ? applyFiltersAndSort(data, mode, filters, sort) : [];
  const isEmpty = !isLoading && visible.length === 0;
  const title = mode === 'archived' ? 'Архивные объекты' : 'Мои объекты';

  return (
    <div className={styles.root}>
      <h1 className={styles.title}>{title}</h1>
      <PropertiesToolbar
        filters={filters}
        sort={sort}
        onChange={(nextFilters, nextSort) => { setFilters(nextFilters); setSort(nextSort); }}
      />

      {isLoading && <PropertiesLoading />}

      {!isLoading && isEmpty && <PropertiesEmptyState />}

      {!isLoading && !isEmpty && (
        <ul className={styles.list}>
          {visible.map((property) => (
            <li key={property.id}>
              <PropertyCard property={property} />
            </li>
          ))}
        </ul>
      )}

      {mode === 'active' && !isLoading && <PropertiesArchiveLink />}
    </div>
  );
}
