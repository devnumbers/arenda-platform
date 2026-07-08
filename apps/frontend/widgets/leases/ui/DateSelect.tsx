'use client';

import type {JSX, ReactNode} from 'react';
import {useId, useMemo, useState} from 'react';
import {parseDate, type CalendarDate} from '@internationalized/date';
import {
    Popover,
    PopoverContent,
    PopoverDialog,
    PopoverTrigger,
} from '@heroui/react/popover';
import {Calendar} from '@heroui/react/calendar';
import {ChevronDown, ChevronUp} from '@/shared/assets/icons';
import {formatDate} from '@/shared/lib/format-date';
import styles from './DateSelect.module.css';

export type DateSelectProps = {
    readonly label: string;
    readonly value?: string;
    readonly placeholder?: string;
    readonly required?: boolean;
    readonly minValue?: string;
    readonly maxValue?: string;
    readonly defaultFocusedValue?: string;
    readonly renderValue?: (value: string) => ReactNode;
    readonly onChange: (date: string | undefined) => void;
};

export function DateSelect({
    label,
    value,
    placeholder,
    required,
    minValue,
    maxValue,
    defaultFocusedValue,
    renderValue,
    onChange,
}: DateSelectProps): JSX.Element {
    const triggerId = useId();
    const [isOpen, setIsOpen] = useState(false);

    const handleChange = (date: CalendarDate | null) => {
        if (!date) return;
        onChange(date.toString());
        setIsOpen(false);
    };

    const handleClear = () => {
        onChange(undefined);
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

    return (
        <div className={styles.root}>
            <Popover isOpen={isOpen} onOpenChange={setIsOpen}>
                <PopoverTrigger>
                    <button
                        id={triggerId}
                        type="button"
                        className={styles.trigger}
                        aria-haspopup="dialog"
                        aria-expanded={isOpen}
                    >
                        <span className={styles.label}>
                            {label}
                            {required && <span className={styles.required}>*</span>}
                        </span>
                        <span className={styles.control}>
                            <span className={styles.value}>
                                {displayValue ?? placeholder ?? 'Выбрать'}
                            </span>
                            <span className={styles.chevron} aria-hidden="true">
                                {isOpen ? <ChevronUp/> : <ChevronDown/>}
                            </span>
                        </span>
                    </button>
                </PopoverTrigger>
                <PopoverContent className={styles.popover}>
                    <PopoverDialog aria-label={label}>
                        <Calendar
                            aria-label={label}
                            value={calendarValue}
                            onChange={handleChange}
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
                        </Calendar>
                        {value && (
                            <button
                                type="button"
                                className={styles.clearButton}
                                onClick={handleClear}
                            >
                                Очистить
                            </button>
                        )}
                    </PopoverDialog>
                </PopoverContent>
            </Popover>
        </div>
    );
}
