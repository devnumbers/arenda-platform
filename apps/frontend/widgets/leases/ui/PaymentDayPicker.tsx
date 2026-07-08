'use client';

import type {JSX} from 'react';
import {useMemo} from 'react';
import {DateSelect} from './DateSelect';

const REFERENCE_DATE = '2026-01-01';

export type PaymentDayPickerProps = {
    readonly value?: number;
    readonly onChange: (day: number) => void;
};

export function PaymentDayPicker({value, onChange}: PaymentDayPickerProps): JSX.Element {
    const dateValue = useMemo(() => {
        if (value === undefined || value < 1 || value > 31) return undefined;
        return `${REFERENCE_DATE.slice(0, 8)}${String(value).padStart(2, '0')}`;
    }, [value]);

    const handleChange = (date: string | undefined) => {
        if (!date) return;
        onChange(Number(date.slice(8, 10)));
    };

    return (
        <DateSelect
            label="День оплаты"
            value={dateValue}
            onChange={handleChange}
            placeholder="Выбрать"
            mode="days-grid"
            required
            defaultFocusedValue={REFERENCE_DATE}
            renderValue={() => (value ? `${value}-е число` : undefined)}
        />
    );
}
