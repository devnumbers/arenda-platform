import {
    endOfMonth,
    endOfQuarter,
    endOfYear,
    formatDateForApi,
    parseDateForApi,
    startOfMonth,
    startOfQuarter,
    startOfYear,
} from '@/entities/operation';
import type {OperationPeriod} from '../ui/OperationFilters';

export type OperationTypeFilter = 'income' | 'expense';
export type OperationListSort = 'operation_date_desc' | 'operation_date_asc';

export type OperationInitialFilters = {
    readonly type?: OperationTypeFilter;
    readonly propertyId?: string;
    readonly period: OperationPeriod;
    readonly from?: string;
    readonly to?: string;
    readonly isDefaultPeriod: boolean;
    readonly status: ReadonlyArray<string>;
    readonly sort?: OperationListSort;
};

type SearchParamsLike = Record<string, string | string[] | undefined>;

type PeriodRange = {
    readonly from: string;
    readonly to: string;
};

type ResolvedOperationPeriod = {
    readonly period: OperationPeriod;
    readonly from?: string;
    readonly to?: string;
    readonly isDefaultPeriod: boolean;
};

export const DEFAULT_OPERATION_SORT: OperationListSort = 'operation_date_asc';

function readString(value: string | string[] | undefined): string | undefined {
    return Array.isArray(value) ? value[0] : value;
}

function readAll(value: string | string[] | undefined): ReadonlyArray<string> {
    if (Array.isArray(value)) {
        return value;
    }
    return value ? [value] : [];
}

function getOperationType(value: string | null): OperationTypeFilter | undefined {
    if (value === 'income' || value === 'expense') {
        return value;
    }
    return undefined;
}

function getOperationPeriod(value: string | null): OperationPeriod | undefined {
    if (
        value === 'all' ||
        value === 'month' ||
        value === 'quarter' ||
        value === 'year' ||
        value === 'custom'
    ) {
        return value;
    }
    return undefined;
}

function getOperationSort(value: string | null): OperationListSort | undefined {
    if (value === 'operation_date_desc' || value === 'operation_date_asc') {
        return value;
    }
    return undefined;
}

function getMonthRange(date: Date): PeriodRange {
    return {
        from: formatDateForApi(startOfMonth(date)),
        to: formatDateForApi(endOfMonth(date)),
    };
}

function getQuarterRange(date: Date): PeriodRange {
    return {
        from: formatDateForApi(startOfQuarter(date)),
        to: formatDateForApi(endOfQuarter(date)),
    };
}

function getYearRange(date: Date): PeriodRange {
    return {
        from: formatDateForApi(startOfYear(date)),
        to: formatDateForApi(endOfYear(date)),
    };
}

function getPeriodRange(
    period: Exclude<OperationPeriod, 'all' | 'custom'>,
    date: Date,
): PeriodRange {
    if (period === 'month') return getMonthRange(date);
    if (period === 'quarter') return getQuarterRange(date);
    return getYearRange(date);
}

function isValidRange(from: string | null, to: string | null): boolean {
    const fromDate = parseDateForApi(from);
    const toDate = parseDateForApi(to);
    return Boolean(fromDate && toDate && fromDate <= toDate);
}

function isSameRange(first: PeriodRange, second: PeriodRange): boolean {
    return first.from === second.from && first.to === second.to;
}

function resolveOperationPeriod(searchParams: {
    get: (name: string) => string | null;
}): ResolvedOperationPeriod {
    const now = new Date();
    const currentMonthRange = getMonthRange(now);
    const requestedPeriod = getOperationPeriod(searchParams.get('period'));

    if (requestedPeriod === 'all') {
        return {period: 'all', isDefaultPeriod: false};
    }

    const requestedFrom = searchParams.get('from');
    const requestedTo = searchParams.get('to');

    if (requestedPeriod === 'custom') {
        if (isValidRange(requestedFrom, requestedTo)) {
            return {
                period: 'custom',
                from: requestedFrom ?? undefined,
                to: requestedTo ?? undefined,
                isDefaultPeriod: false,
            };
        }

        return {
            period: 'month',
            from: currentMonthRange.from,
            to: currentMonthRange.to,
            isDefaultPeriod: true,
        };
    }

    if (requestedPeriod) {
        const range = isValidRange(requestedFrom, requestedTo)
            ? {from: requestedFrom ?? currentMonthRange.from, to: requestedTo ?? currentMonthRange.to}
            : getPeriodRange(requestedPeriod, now);

        return {
            period: requestedPeriod,
            from: range.from,
            to: range.to,
            isDefaultPeriod: requestedPeriod === 'month' && isSameRange(range, currentMonthRange),
        };
    }

    return {
        period: 'month',
        from: currentMonthRange.from,
        to: currentMonthRange.to,
        isDefaultPeriod: true,
    };
}

export function parseOperationsFromParams(params: SearchParamsLike): OperationInitialFilters {
    const searchParams = {
        get: (name: string) => readString(params[name]) ?? null,
    };
    const resolved = resolveOperationPeriod(searchParams);

    return {
        type: getOperationType(readString(params.type) ?? null),
        propertyId: readString(params.property_id) || undefined,
        period: resolved.period,
        from: resolved.from,
        to: resolved.to,
        isDefaultPeriod: resolved.isDefaultPeriod,
        status: readAll(params.status),
        sort: getOperationSort(readString(params.sort) ?? null),
    };
}
