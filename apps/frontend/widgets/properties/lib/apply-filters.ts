import type { PropertyWithLease } from './use-property-list-data';
import type { PropertyFilters, PropertySort } from './filter-types';
import { getDisplayStatus } from '@/features/properties/lib/property-statuses';

export type PropertiesViewMode = 'active' | 'archived';

export function applyFiltersAndSort(
  items: readonly PropertyWithLease[],
  mode: PropertiesViewMode,
  filters: PropertyFilters,
  sort: PropertySort,
): PropertyWithLease[] {
  let result = items.filter((p) => {
    if (mode === 'active') return p.status !== 'archived';
    return p.status === 'archived';
  });

  if (filters.types.length > 0) {
    result = result.filter((p) => filters.types.includes(p.type));
  }

  if (filters.statuses.length > 0) {
    result = result.filter((p) => {
      const display = getDisplayStatus(p.status, p.occupancy);
      return display ? filters.statuses.includes(display) : false;
    });
  }

  result = [...result].sort((a, b) => {
    const cmp = a.name.localeCompare(b.name, 'ru');
    return sort === 'name_asc' ? cmp : -cmp;
  });

  return result;
}
