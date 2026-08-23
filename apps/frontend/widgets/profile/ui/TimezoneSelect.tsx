'use client';

import {type ChangeEvent, type JSX, type KeyboardEvent, useCallback, useMemo, useState} from 'react';
import {Select, type SelectOption} from '@/shared/ui/select';

export type TimezoneSelectProps = {
    readonly value?: string;
    readonly onChange: (timezone: string) => void;
    readonly error?: string;
    readonly disabled?: boolean;
    readonly required?: boolean;
};

// All 22 IANA timezones covering the territory of the Russian Federation,
// sorted by UTC offset (west → east). Each entry pairs the canonical IANA
// identifier (the stored value) with a Russian display label that combines
// the representative city and the UTC offset, so users can find their zone
// by either the city name or the offset.
const TIMEZONE_OPTIONS: readonly SelectOption<string>[] = [
    {value: 'Europe/Kaliningrad', label: 'Калининград (UTC+2)'},
    {value: 'Europe/Moscow', label: 'Москва (UTC+3)'},
    {value: 'Europe/Samara', label: 'Самара (UTC+4)'},
    {value: 'Europe/Saratov', label: 'Саратов (UTC+4)'},
    {value: 'Asia/Yekaterinburg', label: 'Екатеринбург (UTC+5)'},
    {value: 'Asia/Omsk', label: 'Омск (UTC+6)'},
    {value: 'Asia/Novosibirsk', label: 'Новосибирск (UTC+7)'},
    {value: 'Asia/Barnaul', label: 'Барнаул (UTC+7)'},
    {value: 'Asia/Tomsk', label: 'Томск (UTC+7)'},
    {value: 'Asia/Novokuznetsk', label: 'Новокузнецк (UTC+7)'},
    {value: 'Asia/Krasnoyarsk', label: 'Красноярск (UTC+7)'},
    {value: 'Asia/Irkutsk', label: 'Иркутск (UTC+8)'},
    {value: 'Asia/Chita', label: 'Чита (UTC+9)'},
    {value: 'Asia/Yakutsk', label: 'Якутск (UTC+9)'},
    {value: 'Asia/Khandyga', label: 'Хандыга (UTC+9)'},
    {value: 'Asia/Vladivostok', label: 'Владивосток (UTC+10)'},
    {value: 'Asia/Ust-Nera', label: 'Усть-Нера (UTC+10)'},
    {value: 'Asia/Magadan', label: 'Магадан (UTC+11)'},
    {value: 'Asia/Sakhalin', label: 'Южно-Сахалинск (UTC+11)'},
    {value: 'Asia/Srednekolymsk', label: 'Среднеколымск (UTC+11)'},
    {value: 'Asia/Kamchatka', label: 'Петропавловск-Камчатский (UTC+12)'},
    {value: 'Asia/Anadyr', label: 'Анадырь (UTC+12)'},
];

export function TimezoneSelect({value, onChange, error, disabled, required}: TimezoneSelectProps): JSX.Element {
    const [inputValue, setInputValue] = useState(() => labelFor(value));
    const [isOpen, setIsOpen] = useState(false);
    const [activeIndex, setActiveIndex] = useState<number | null>(null);

    const filteredOptions = useMemo(() => {
        const query = inputValue.trim().toLowerCase();
        if (query === '') {
            return TIMEZONE_OPTIONS;
        }
        return TIMEZONE_OPTIONS.filter((option) => option.label.toLowerCase().includes(query));
    }, [inputValue]);

    // Sync local input with the timezone value controlled by the parent form —
    // the render-time prop adjustment from the React docs (no effect).
    const [prevValue, setPrevValue] = useState(value);
    if (prevValue !== value) {
        setPrevValue(value);
        setInputValue(labelFor(value));
    }

    const handleSelect = useCallback(
        (timezone: string) => {
            setInputValue(labelFor(timezone));
            onChange(timezone);
            setIsOpen(false);
            setActiveIndex(null);
        },
        [onChange]
    );

    const handleChange = (event: ChangeEvent<HTMLInputElement>) => {
        setInputValue(event.currentTarget.value);
        setIsOpen(true);
        setActiveIndex(null);
    };

    const handleFocus = () => {
        setIsOpen(true);
    };

    const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
        const items = filteredOptions;
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
            const item = items[activeIndex];
            if (item !== undefined) {
                handleSelect(item.value);
            }
        } else if (event.key === 'Escape') {
            setIsOpen(false);
            setActiveIndex(null);
        }
    };

    return (
        <Select
            label="Часовой пояс"
            searchable
            inputValue={inputValue}
            onInputChange={handleChange}
            onFocus={handleFocus}
            onKeyDown={handleKeyDown}
            value={value}
            options={filteredOptions}
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

function labelFor(timezone: string | undefined): string {
    if (!timezone) {
        return '';
    }
    return TIMEZONE_OPTIONS.find((option) => option.value === timezone)?.label ?? '';
}
