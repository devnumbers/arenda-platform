'use client';

import {
    type ChangeEvent,
    type JSX,
    type KeyboardEvent,
    type ReactNode,
    useCallback,
    useEffect,
    useMemo,
    useRef,
    useState,
} from 'react';
import clsx from 'clsx';
import { ChevronDown } from '@/shared/assets/icons';
import { TextField } from '@/shared/ui/text-field';
import styles from './Select.module.css';

export type SelectOption<Value extends string = string> = {
    readonly value: Value;
    readonly label: string;
};

export type SelectProps<Value extends string = string> = {
    readonly label: string;
    readonly value?: Value;
    readonly options: readonly SelectOption<Value>[];
    readonly onChange: (value: Value) => void;
    readonly placeholder?: string;
    readonly error?: string;
    readonly disabled?: boolean;
    readonly loading?: boolean;
    readonly emptyMessage?: ReactNode;
    readonly searchable?: boolean;
    readonly inputValue?: string;
    readonly onInputChange?: (event: ChangeEvent<HTMLInputElement>) => void;
    readonly onFocus?: () => void;
    readonly onKeyDown?: (event: KeyboardEvent<HTMLInputElement>) => void;
    readonly activeIndex?: number | null;
    readonly minQueryLength?: number;
    readonly required?: boolean;
    readonly maxLength?: number;
    readonly open?: boolean;
    readonly onOpenChange?: (open: boolean) => void;
};

export function Select<Value extends string = string>({
    label,
    value,
    options,
    onChange,
    placeholder,
    error,
    disabled,
    loading,
    emptyMessage,
    searchable,
    inputValue,
    onInputChange,
    onFocus,
    onKeyDown,
    activeIndex,
    minQueryLength,
    open,
    onOpenChange,
    required,
    maxLength,
}: SelectProps<Value>): JSX.Element {
    const [internalOpen, setInternalOpen] = useState(false);
    const isOpen = open !== undefined ? open : internalOpen;

    const setIsOpen = useCallback((nextOpen: boolean) => {
        if (open === undefined) {
            setInternalOpen(nextOpen);
        }
        onOpenChange?.(nextOpen);
    }, [open, onOpenChange]);

    const wrapperRef = useRef<HTMLDivElement>(null);

    const selectedLabel = useMemo(
        () => options.find((option) => option.value === value)?.label ?? '',
        [options, value]
    );

    useEffect(() => {
        function handleClickOutside(event: MouseEvent) {
            if (
                wrapperRef.current &&
                !wrapperRef.current.contains(event.target as Node)
            ) {
                setIsOpen(false);
            }
        }

        document.addEventListener('mousedown', handleClickOutside);
        return () => document.removeEventListener('mousedown', handleClickOutside);
    }, [setIsOpen]);

    const handleSelect = (selectedValue: Value) => {
        onChange(selectedValue);
        setIsOpen(false);
    };

    const showMessage = (loading || emptyMessage) && options.length === 0;

    const canShowDropdown = searchable
        ? (inputValue?.trim().length ?? 0) >= (minQueryLength ?? 0)
        : true;

    return (
        <div className={styles.root} ref={wrapperRef}>
            {searchable ? (
                <TextField
                    label={label}
                    value={inputValue}
                    onChange={onInputChange}
                    onFocus={onFocus}
                    onKeyDown={onKeyDown}
                    error={error}
                    fullWidth
                    placeholder={placeholder}
                    required={required}
                    maxLength={maxLength}
                />
            ) : (
                <button
                    type="button"
                    className={clsx(styles.trigger, error && styles.error)}
                    onClick={() => setIsOpen(!isOpen)}
                    disabled={disabled}
                >
                    <span className={styles.label}>{label}</span>
                    <span className={styles.control}>
                        <span className={styles.value}>{selectedLabel || placeholder}</span>
                        <span className={styles.chevron} aria-hidden="true">
                            <ChevronDown />
                        </span>
                    </span>
                    {error && <span className={styles.errorText}>{error}</span>}
                </button>
            )}

            {isOpen && canShowDropdown && (
                <div className={styles.dropdown}>
                    {showMessage ? (
                        <div className={styles.message}>
                            {loading ? 'Загрузка...' : emptyMessage}
                        </div>
                    ) : (
                        <ul className={styles.list} role="listbox" aria-label={label}>
                            {options.map((option, index) => (
                                <li
                                    key={option.value}
                                    className={clsx(
                                        styles.listItem,
                                        value === option.value && styles.listItemSelected,
                                        activeIndex === index && styles.listItemActive
                                    )}
                                    role="option"
                                    aria-selected={value === option.value}
                                    data-active={activeIndex === index}
                                    onClick={() => handleSelect(option.value)}
                                >
                                    {option.label}
                                </li>
                            ))}
                        </ul>
                    )}
                </div>
            )}
        </div>
    );
}
