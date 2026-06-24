import type { PropertyType } from '@/entities/property/model/types';
import type { StatusFilterValue } from '@/features/properties/lib/property-statuses';

export type PropertyFilters = {
  readonly types: readonly PropertyType[];
  readonly statuses: readonly StatusFilterValue[];
};

export type PropertySort = 'name_asc' | 'name_desc';

export const sortOptions = [
  { value: 'name_asc', label: 'По названию А-Я' },
  { value: 'name_desc', label: 'По названию Я-А' },
] as const;

export function toggleValue<T>(list: readonly T[], value: T): readonly T[] {
  return list.includes(value)
    ? list.filter((v) => v !== value)
    : [...list, value];
}
