'use client';

import type {JSX, ReactNode} from 'react';
import {useId, useMemo, useState} from 'react';
import clsx from 'clsx';
import {type CalendarDate, parseDate} from '@internationalized/date';
import {
    Popover,
    PopoverContent,
    PopoverDialog,
    PopoverTrigger,
} from '@heroui/react/popover';
import {Calendar} from '@heroui/react/calendar';
import {Calendar as CalendarIcon} from '@/shared/assets/icons';
import {formatDate} from '@/shared/lib/format-date';
import styles from './DateSelect.module.css';

export type DateSelectMode = 'calendar' | 'days-grid';

export type DateSelectProps = {
    readonly label: string;
    readonly value?: string;
    readonly placeholder?: string;
    readonly required?: boolean;
    readonly minValue?: string;
    readonly maxValue?: string;
    readonly defaultFocusedValue?: string;
    readonly renderValue?: (value: string) => ReactNode;
    readonly mode?: DateSelectMode;
    readonly error?: string;
    readonly onChange: (date: string | undefined) => void;
};

const DAYS = Array.from({length: 31}, (_, index) => index + 1);

function getYearMonthReference(value: string | undefined, defaultFocusedValue: string | undefined): string {
    const source = value ?? defaultFocusedValue;
    if (source && source.length >= 7) {
        return source.slice(0, 7);
    }
    const now = new Date();
    return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`;
}

export function DateSelect({
    label,
    value,
    placeholder,
    required,
    minValue,
    maxValue,
    defaultFocusedValue,
    renderValue,
    mode = 'calendar',
    error,
    onChange,
}: DateSelectProps): JSX.Element {
    const triggerId = useId();
    const [isOpen, setIsOpen] = useState(false);

    const handleCalendarChange = (date: CalendarDate | null) => {
        if (!date) return;
        onChange(date.toString());
        setIsOpen(false);
    };

    const handleDaySelect = (day: number) => {
        const yearMonth = getYearMonthReference(value, defaultFocusedValue);
        const date = `${yearMonth}-${String(day).padStart(2, '0')}`;
        onChange(date);
        setIsOpen(false);
    };

    const calendarValue = useMemo(() => {
        if (!value) return null;
        try {
            return parseDate(value);
        } catch {
            return null;
        }
    }, [value]);

    const calendarMinValue = useMemo(() => {
        if (!minValue) return undefined;
        try {
            return parseDate(minValue);
        } catch {
            return undefined;
        }
    }, [minValue]);

    const calendarMaxValue = useMemo(() => {
        if (!maxValue) return undefined;
        try {
            return parseDate(maxValue);
        } catch {
            return undefined;
        }
    }, [maxValue]);

    const calendarDefaultFocusedValue = useMemo(() => {
        if (!defaultFocusedValue) return undefined;
        try {
            return parseDate(defaultFocusedValue);
        } catch {
            return undefined;
        }
    }, [defaultFocusedValue]);

    const displayValue = useMemo(() => {
        if (!value) return null;
        if (renderValue) return renderValue(value);
        return formatDate(value);
    }, [value, renderValue]);

    const selectedDay = useMemo(() => {
        if (!value || value.length < 10) return undefined;
        const day = Number(value.slice(8, 10));
        return Number.isNaN(day) ? undefined : day;
    }, [value]);

    return (
        <div className={styles.root}>
            <Popover isOpen={isOpen} onOpenChange={setIsOpen}>
                <PopoverTrigger>
                    <button
                        id={triggerId}
                        type="button"
                        className={clsx(styles.trigger, error && styles.error)}
                        aria-haspopup="dialog"
                        aria-expanded={isOpen}
                    >
                        <span className={styles.label}>
                            {label}
                            {required && <span className={styles.required}>*</span>}
                        </span>
                        <span className={styles.control}>
                            <span className={styles.value}>
                                {displayValue ?? placeholder ?? 'Выбрать дату'}
                            </span>
                            <span className={styles.icon} aria-hidden="true">
                                <CalendarIcon/>
                            </span>
                        </span>
                        {error && <span className={styles.errorText}>{error}</span>}
                    </button>
                </PopoverTrigger>
                <PopoverContent
                    className={clsx(
                        styles.popover,
                        mode === 'days-grid' && styles.popoverDaysGrid,
                    )}
                >
                    <PopoverDialog className={styles.body} aria-label={label}>
                        {mode === 'days-grid' ? (
                            <div className={styles.daysGrid} role="grid" aria-label={label}>
                                {DAYS.map((day) => {
                                    const isSelected = selectedDay === day;
                                    return (
                                        <button
                                            key={day}
                                            type="button"
                                            className={clsx(
                                                styles.dayButton,
                                                isSelected && styles.dayButtonSelected,
                                            )}
                                            role="gridcell"
                                            aria-selected={isSelected}
                                            onClick={() => handleDaySelect(day)}
                                        >
                                            {day}
                                        </button>
                                    );
                                })}
                            </div>
                        ) : (
                            <Calendar
                                aria-label={label}
                                value={calendarValue}
                                onChange={handleCalendarChange}
                                defaultFocusedValue={calendarDefaultFocusedValue}
                                minValue={calendarMinValue}
                                maxValue={calendarMaxValue}
                            >
                                <Calendar.Header>
                                    <Calendar.YearPickerTrigger>
                                        <Calendar.YearPickerTriggerHeading/>
                                        <Calendar.YearPickerTriggerIndicator/>
                                    </Calendar.YearPickerTrigger>
                                    <Calendar.NavButton slot="previous"/>
                                    <Calendar.NavButton slot="next"/>
                                </Calendar.Header>
                                <Calendar.Grid>
                                    <Calendar.GridHeader>
                                        {(day) => <Calendar.HeaderCell>{day}</Calendar.HeaderCell>}
                                    </Calendar.GridHeader>
                                    <Calendar.GridBody>
                                        {(date) => <Calendar.Cell date={date}/>}
                                    </Calendar.GridBody>
                                </Calendar.Grid>
                                <Calendar.YearPickerGrid>
                                    <Calendar.YearPickerGridBody>
                                        {({year}) => <Calendar.YearPickerCell year={year}/>}
                                    </Calendar.YearPickerGridBody>
                                </Calendar.YearPickerGrid>
                            </Calendar>
                        )}
                    </PopoverDialog>
                </PopoverContent>
            </Popover>
        </div>
    );
}
