import type { PropertyType } from '@/entities/property/model/types';
import type { StatusFilterValue } from '@/features/properties/lib/property-statuses';

export type PropertyFilters = {
  types: PropertyType[];
  statuses: StatusFilterValue[];
};

export type PropertySort = 'name_asc' | 'name_desc';

export const sortOptions: { value: PropertySort; label: string }[] = [
  { value: 'name_asc', label: 'По названию А-Я' },
  { value: 'name_desc', label: 'По названию Я-А' },
];
