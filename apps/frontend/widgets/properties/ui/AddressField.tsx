'use client';

import {type ChangeEvent, type JSX, type KeyboardEvent, useCallback, useEffect, useState} from 'react';
import {useAddressSuggestions} from '@/features/properties/api';
import {useDebounce} from '@/shared/lib/hooks/useDebounce';
import {Select} from '@/shared/ui/select';

export type AddressFieldProps = {
    readonly value?: string;
    readonly onChange: (address: string) => void;
    readonly error?: string;
};

export function AddressField({value, onChange, error}: AddressFieldProps): JSX.Element {
    const [inputValue, setInputValue] = useState(value ?? '');
    const [isOpen, setIsOpen] = useState(false);
    const [activeIndex, setActiveIndex] = useState<number | null>(null);
    const debouncedQuery = useDebounce(inputValue, 300);
    const {data: suggestions, isLoading} = useAddressSuggestions(debouncedQuery);

    useEffect(() => {
        // Sync local input with the address value controlled by the parent form.
        // eslint-disable-next-line react-hooks/set-state-in-effect
        setInputValue(value ?? '');
    }, [value]);

    const handleSelect = useCallback(
        (address: string) => {
            setInputValue(address);
            onChange(address);
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
        onChange(nextValue);
    };

    const handleFocus = () => {
        setIsOpen(true);
    };

    const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
        const items = suggestions ?? [];
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
            handleSelect(items[activeIndex].value);
        } else if (event.key === 'Escape') {
            setIsOpen(false);
            setActiveIndex(null);
        }
    };

    const options = suggestions?.map((suggestion) => ({
        value: suggestion.value,
        label: suggestion.value,
    })) ?? [];

    return (
        <Select
            label="Адрес"
            searchable
            inputValue={inputValue}
            onInputChange={handleChange}
            onFocus={handleFocus}
            onKeyDown={handleKeyDown}
            value={value}
            options={options}
            onChange={handleSelect}
            error={error}
            loading={isLoading}
            emptyMessage="Адреса не найдены"
            placeholder=""
            required
            maxLength={500}
            activeIndex={activeIndex}
            minQueryLength={3}
            open={isOpen}
            onOpenChange={setIsOpen}
        />
    );
}
