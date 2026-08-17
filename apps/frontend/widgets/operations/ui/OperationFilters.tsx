'use client';

import {useMemo, type JSX} from 'react';
import {ArrowRight} from '@/shared/assets/icons';
import {Button} from '@/shared/ui/button';
import {Icon} from '@/shared/ui/icon';
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
import styles from './OperationFilters.module.css';
import clsx from "clsx";
import {PropertyFilter} from './PropertyFilter';

export type OperationPeriod = 'all' | 'month' | 'quarter' | 'year' | 'custom';

export type OperationFiltersState = {
    readonly period: OperationPeriod;
    readonly from?: string;
    readonly to?: string;
    readonly status: ReadonlyArray<string>;
    readonly propertyId?: string;
};

export type OperationFiltersProps = {
    readonly filters: OperationFiltersState;
    readonly properties?: ReadonlyArray<{readonly id: string; readonly name: string}>;
    readonly propertiesLoading?: boolean;
    readonly hasExternalFilters?: boolean;
    readonly isDefaultPeriod?: boolean;
    readonly onChange: (filters: OperationFiltersState) => void;
    readonly onReset?: () => void;
};

const COMPLETED_STATUSES: ReadonlyArray<string> = ['paid', 'received'];

type PeriodRange = {
    readonly from: string;
    readonly to: string;
};

const MONTH_LABELS = [
    'Январь',
    'Февраль',
    'Март',
    'Апрель',
    'Май',
    'Июнь',
    'Июль',
    'Август',
    'Сентябрь',
    'Октябрь',
    'Ноябрь',
    'Декабрь',
] as const;

function getCurrentMonthRange(): PeriodRange {
    const now = new Date();
    return {
        from: formatDateForApi(startOfMonth(now)),
        to: formatDateForApi(endOfMonth(now)),
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
    if (period === 'month') return getCurrentMonthRangeForDate(date);
    if (period === 'quarter') return getQuarterRange(date);
    return getYearRange(date);
}

function getCurrentMonthRangeForDate(date: Date): PeriodRange {
    return {
        from: formatDateForApi(startOfMonth(date)),
        to: formatDateForApi(endOfMonth(date)),
    };
}

function isValidRange(from?: string, to?: string): boolean {
    const fromDate = parseDateForApi(from);
    const toDate = parseDateForApi(to);
    return Boolean(fromDate && toDate && fromDate <= toDate);
}

function getSelectedRange(filters: OperationFiltersState): PeriodRange | undefined {
    if (filters.period === 'all') return undefined;
    if (isValidRange(filters.from, filters.to)) {
        return {from: filters.from, to: filters.to} as PeriodRange;
    }
    return getCurrentMonthRange();
}

function addDays(date: Date, days: number): Date {
    return new Date(date.getFullYear(), date.getMonth(), date.getDate() + days);
}

function addMonths(date: Date, months: number): Date {
    return new Date(date.getFullYear(), date.getMonth() + months, 1);
}

function addYears(date: Date, years: number): Date {
    return new Date(date.getFullYear() + years, date.getMonth(), 1);
}

function getDaySerial(date: Date): number {
    return Date.UTC(date.getFullYear(), date.getMonth(), date.getDate());
}

function getInclusiveDayCount(from: Date, to: Date): number {
    const days = Math.round((getDaySerial(to) - getDaySerial(from)) / 86_400_000);
    return Math.max(days + 1, 1);
}

function formatShortDate(dateString: string): string {
    const date = parseDateForApi(dateString);
    if (!date) return dateString;

    const day = String(date.getDate()).padStart(2, '0');
    const month = String(date.getMonth() + 1).padStart(2, '0');
    return `${day}.${month}.${date.getFullYear()}`;
}

function getPeriodLabel(filters: OperationFiltersState): string {
    if (filters.period === 'all') return 'Все время';

    const range = getSelectedRange(filters);
    if (!range) return 'Все время';

    const fromDate = parseDateForApi(range.from);
    const toDate = parseDateForApi(range.to);
    if (!fromDate || !toDate) return 'Текущий месяц';

    if (filters.period === 'custom') {
        return `${formatShortDate(range.from)}-${formatShortDate(range.to)}`;
    }

    if (filters.period === 'month') {
        return `${MONTH_LABELS[fromDate.getMonth()]} ${fromDate.getFullYear()}`;
    }

    if (filters.period === 'quarter') {
        const quarter = Math.floor(fromDate.getMonth() / 3) + 1;
        return `${quarter} квартал ${fromDate.getFullYear()}`;
    }

    return `${fromDate.getFullYear()} год`;
}

function setPeriod(
    period: OperationPeriod,
    onChange: (filters: OperationFiltersState) => void,
    currentFilters: OperationFiltersState,
): void {
    if (period === 'all') {
        onChange({period: 'all', status: currentFilters.status, propertyId: currentFilters.propertyId});
        return;
    }

    const now = new Date();
    const range = period === 'custom' ? getCurrentMonthRange() : getPeriodRange(period, now);
    onChange({...currentFilters, period, from: range.from, to: range.to});
}

function movePeriod(
    filters: OperationFiltersState,
    direction: -1 | 1,
): OperationFiltersState {
    const range = getSelectedRange(filters);
    if (!range) return filters;

    const fromDate = parseDateForApi(range.from);
    const toDate = parseDateForApi(range.to);
    if (!fromDate || !toDate) return filters;

    if (filters.period === 'custom') {
        const dayCount = getInclusiveDayCount(fromDate, toDate);
        return {
            ...filters,
            from: formatDateForApi(addDays(fromDate, dayCount * direction)),
            to: formatDateForApi(addDays(toDate, dayCount * direction)),
        };
    }

    if (filters.period === 'month') {
        return {...filters, ...getPeriodRange('month', addMonths(fromDate, direction))};
    }

    if (filters.period === 'quarter') {
        return {...filters, ...getPeriodRange('quarter', addMonths(fromDate, direction * 3))};
    }

    if (filters.period === 'year') {
        return {...filters, ...getPeriodRange('year', addYears(fromDate, direction))};
    }

    return filters;
}

function isCompletedSelected(status: ReadonlyArray<string>): boolean {
    return COMPLETED_STATUSES.every((value) => status.includes(value));
}

function toggleCompleted(status: ReadonlyArray<string>): string[] {
    if (isCompletedSelected(status)) {
        return status.filter((value) => !COMPLETED_STATUSES.includes(value));
    }
    const next = new Set(status);
    COMPLETED_STATUSES.forEach((value) => next.add(value));
    return Array.from(next);
}

function toggleStatus(status: ReadonlyArray<string>, value: string): string[] {
    if (status.includes(value)) {
        return status.filter((item) => item !== value);
    }
    return [...status, value];
}

function hasActiveFilters(
    filters: OperationFiltersState,
    hasExternalFilters: boolean,
    isDefaultPeriod: boolean,
): boolean {
    const statusActive = filters.status.length > 0;
    const periodActive = !isDefaultPeriod;
    const propertyActive = Boolean(filters.propertyId);
    return statusActive || periodActive || propertyActive || hasExternalFilters;
}

const PERIOD_CHIPS: ReadonlyArray<{
    readonly key: OperationPeriod;
    readonly label: string;
}> = [
    {key: 'month', label: 'Месяц'},
    {key: 'quarter', label: 'Квартал'},
    {key: 'year', label: 'Год'},
    {key: 'all', label: 'Все время'},
];

const STATUS_CHIPS: ReadonlyArray<{
    readonly key: 'all' | 'pending' | 'overdue' | 'completed' | 'unconfirmed';
    readonly label: string;
}> = [
    {key: 'all', label: 'Все'},
    {key: 'pending', label: 'Запланирована'},
    {key: 'overdue', label: 'Просрочена'},
    {key: 'completed', label: 'Выполнена'},
    {key: 'unconfirmed', label: 'Не подтверждена'},
];

export function OperationFilters({
                                     filters,
                                     properties = [],
                                     propertiesLoading = false,
                                     hasExternalFilters = false,
                                     isDefaultPeriod = false,
                                     onChange,
                                     onReset,
                                 }: OperationFiltersProps): JSX.Element {
    const canMovePeriod = filters.period !== 'all';

    const propertyOptions = useMemo(
        () => properties.map((property) => ({value: property.id, label: property.name})),
        [properties],
    );

    const handleStatusClick = (key: (typeof STATUS_CHIPS)[number]['key']) => {
        if (key === 'all') {
            onChange({...filters, status: []});
            return;
        }

        if (key === 'completed') {
            onChange({...filters, status: toggleCompleted(filters.status)});
            return;
        }

        onChange({...filters, status: toggleStatus(filters.status, key)});
    };

    const isSelected = (key: (typeof STATUS_CHIPS)[number]['key']): boolean => {
        if (key === 'all') return filters.status.length === 0;
        if (key === 'completed') return isCompletedSelected(filters.status);
        return filters.status.includes(key);
    };

    const handleReset = () => {
        if (onReset) {
            onReset();
            return;
        }
        const currentMonth = getCurrentMonthRange();
        onChange({
            period: 'month',
            from: currentMonth.from,
            to: currentMonth.to,
            status: [],
            propertyId: undefined,
        });
    };

    const handleMovePeriod = (direction: -1 | 1) => {
        onChange(movePeriod(filters, direction));
    };

    return (
        <div className={styles.root}>
            <div className={styles.periodHeader}>
                <div className={styles.periodNavigator}>
                    <Button
                        variant="icon-black"
                        size="small"
                        className={clsx(styles.periodArrow, styles.arrowLeft)}
                        aria-label="Предыдущий период"
                        disabled={!canMovePeriod}
                        onClick={() => handleMovePeriod(-1)}
                    >
                        <Icon size="s">
                            <ArrowRight/>
                        </Icon>
                    </Button>

                    <span className={styles.periodLabel}>{getPeriodLabel(filters)}</span>

                    <Button
                        variant="icon-black"
                        size="small"
                        className={styles.periodArrow}
                        aria-label="Следующий период"
                        disabled={!canMovePeriod}
                        onClick={() => handleMovePeriod(1)}
                    >
                        <Icon size="s">
                            <ArrowRight/>
                        </Icon>
                    </Button>
                </div>
            </div>

            <div className={styles.row}>
                <div className={styles.periodGroup} role="group" aria-label="Быстрый выбор периода">
                    {PERIOD_CHIPS.map((chip) => (
                        <Button
                            key={chip.key}
                            variant={filters.period === chip.key ? 'secondary' : 'icon-black'}
                            size="small"
                            onClick={() => setPeriod(chip.key, onChange, filters)}
                        >
                            {chip.label}
                        </Button>
                    ))}
                </div>

                {hasActiveFilters(filters, hasExternalFilters, isDefaultPeriod) && (
                    <Button
                        variant="clear"
                        size="small"
                        onClick={handleReset}
                        className={styles.resetButton}
                    >
                        Сбросить
                    </Button>
                )}
            </div>

            <div className={styles.chipGroup} role="group" aria-label="Фильтр по статусу">
                {STATUS_CHIPS.map((chip) => (
                    <Button
                        key={chip.key}
                        variant={isSelected(chip.key) ? 'secondary' : 'icon-black'}
                        size="small"
                        onClick={() => handleStatusClick(chip.key)}
                    >
                        {chip.label}
                    </Button>
                ))}
            </div>

            {(properties.length > 0 || propertiesLoading) && (
                <div className={styles.chipGroup}>
                    <PropertyFilter
                        value={filters.propertyId}
                        options={propertyOptions}
                        loading={propertiesLoading}
                        onChange={(value) => onChange({...filters, propertyId: value})}
                    />
                </div>
            )}
        </div>
    );
}
