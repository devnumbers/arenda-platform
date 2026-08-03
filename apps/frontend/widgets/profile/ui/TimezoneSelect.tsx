'use client';

import {type ChangeEvent, type JSX, type KeyboardEvent, useCallback, useEffect, useMemo, useState} from 'react';
import {Select} from '@/shared/ui/select';

export type TimezoneSelectProps = {
    readonly value?: string;
    readonly onChange: (timezone: string) => void;
    readonly error?: string;
    readonly disabled?: boolean;
    readonly required?: boolean;
};

function getAllTimezones(): readonly string[] {
    if (typeof Intl !== 'undefined' && typeof Intl.supportedValuesOf === 'function') {
        try {
            const zones = Intl.supportedValuesOf('timeZone');
            if (zones.length > 0) {
                return zones;
            }
        } catch {
            // Fall through to default.
        }
    }
    return ['Europe/Moscow'];
}

export function TimezoneSelect({value, onChange, error, disabled, required}: TimezoneSelectProps): JSX.Element {
    const [inputValue, setInputValue] = useState(value ?? '');
    const [isOpen, setIsOpen] = useState(false);
    const [activeIndex, setActiveIndex] = useState<number | null>(null);

    const allTimezones = useMemo(() => getAllTimezones(), []);

    const filteredTimezones = useMemo(() => {
        const query = inputValue.trim().toLowerCase();
        if (query === '') {
            return allTimezones;
        }
        return allTimezones.filter((zone) => zone.toLowerCase().includes(query));
    }, [allTimezones, inputValue]);

    useEffect(() => {
        // Sync local input with the timezone value controlled by the parent form.
        // eslint-disable-next-line react-hooks/set-state-in-effect
        setInputValue(value ?? '');
    }, [value]);

    const handleSelect = useCallback(
        (timezone: string) => {
            setInputValue(timezone);
            onChange(timezone);
            setIsOpen(false);
            setActiveIndex(null);
        },
        [onChange]
    );

    const handleChange = (event: ChangeEvent<HTMLInputElement>) => {
        const nextValue = event.currentTarget.value;
        setInputValue(nextValue);
        setIsOpen(true);
        setActiveIndex(null);
    };

    const handleFocus = () => {
        setIsOpen(true);
    };

    const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
        const items = filteredTimezones;
        if (items.length === 0) {
            if (event.key === 'Escape') {
                setIsOpen(false);
                setActiveIndex(null);
            }
            return;
        }

        if (event.key === 'ArrowDown') {
            event.preventDefault();
            setIsOpen(true);
            setActiveIndex((prev) => (prev === null ? 0 : Math.min(prev + 1, items.length - 1)));
        } else if (event.key === 'ArrowUp') {
            event.preventDefault();
            setActiveIndex((prev) =>
                prev === null ? items.length - 1 : Math.max(prev - 1, 0)
            );
        } else if (event.key === 'Enter' && activeIndex !== null) {
            event.preventDefault();
            handleSelect(items[activeIndex]);
        } else if (event.key === 'Escape') {
            setIsOpen(false);
            setActiveIndex(null);
        }
    };

    const options = useMemo(
        () => filteredTimezones.map((zone) => ({value: zone, label: zone})),
        [filteredTimezones]
    );

    return (
        <Select
            label="Часовой пояс"
            searchable
            inputValue={inputValue}
            onInputChange={handleChange}
            onFocus={handleFocus}
            onKeyDown={handleKeyDown}
            value={value}
            options={options}
            onChange={handleSelect}
            error={error}
            disabled={disabled}
            emptyMessage="Часовой пояс не найден"
            placeholder=""
            required={required}
            activeIndex={activeIndex}
            minQueryLength={0}
            open={isOpen}
            onOpenChange={setIsOpen}
        />
    );
}
