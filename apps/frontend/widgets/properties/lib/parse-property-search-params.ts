import {propertyTypeOptions} from '@/features/properties';
import {statusFilterOptions, type StatusFilterValue} from '@/features/properties';
import type {PropertyType} from '@/entities/property';
import type {PropertyFilters, PropertySort} from './filter-types';

export const DEFAULT_PROPERTY_SORT: PropertySort = 'name_asc';

const validPropertyTypes = new Set<PropertyType>(propertyTypeOptions.map((option) => option.value));
const validStatusValues = new Set<StatusFilterValue>(statusFilterOptions.map((option) => option.value));

type SearchParamsLike = Record<string, string | string[] | undefined>;

function readString(value: string | string[] | undefined): string | undefined {
    if (Array.isArray(value)) {
        return value[0];
    }
    return value;
}

export function parseFiltersFromParams(params: SearchParamsLike): PropertyFilters {
    const rawTypes = readString(params.types)?.split(',') ?? [];
    const rawStatuses = readString(params.statuses)?.split(',') ?? [];

    return {
        types: rawTypes.filter((value): value is PropertyType => validPropertyTypes.has(value as PropertyType)),
        statuses: rawStatuses.filter((value): value is StatusFilterValue =>
            validStatusValues.has(value as StatusFilterValue),
        ),
    };
}

export function parseSortFromParams(params: SearchParamsLike): PropertySort {
    const value = readString(params.sort);
    return value === 'name_asc' || value === 'name_desc' ? value : DEFAULT_PROPERTY_SORT;
}
