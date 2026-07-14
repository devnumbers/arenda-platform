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

type SelectBaseProps<Value extends string = string> = {
    readonly label?: string;
    readonly options: readonly SelectOption<Value>[];
    readonly placeholder?: string;
    readonly error?: string;
    readonly disabled?: boolean;
    readonly loading?: boolean;
    readonly emptyMessage?: ReactNode;
    readonly footerRow?: ReactNode;
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
    readonly dropdownAlign?: 'left' | 'right';
    readonly dropdownClassName?: string;
    readonly renderTrigger?: (props: {
        isOpen: boolean;
        onClick: () => void;
    }) => ReactNode;
};

type SelectSingleProps<Value extends string = string> = SelectBaseProps<Value> & {
    readonly multiple?: false;
    readonly value?: Value;
    readonly onChange: (value: Value) => void;
};

type SelectMultipleProps<Value extends string = string> = SelectBaseProps<Value> & {
    readonly multiple: true;
    readonly value?: readonly Value[];
    readonly onChange: (value: Value[]) => void;
};

export type SelectProps<Value extends string = string> =
    | SelectSingleProps<Value>
    | SelectMultipleProps<Value>;

export function Select<Value extends string = string>({
    label,
    value,
    options,
    onChange,
    multiple = false,
    placeholder,
    error,
    disabled,
    loading,
    emptyMessage,
    footerRow,
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
    renderTrigger,
    dropdownAlign = 'left',
    dropdownClassName,
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
        () => multiple ? '' : options.find((option) => option.value === value)?.label ?? '',
        [multiple, options, value]
    );

    const selectedCount = useMemo(() => {
        if (!multiple) return 0;
        return (value as Value[] | undefined)?.length ?? 0;
    }, [multiple, value]);

    const triggerLabel = useMemo(() => {
        if (!multiple || !label) return label;
        return `${label}${selectedCount > 0 ? ` ${selectedCount}` : ''}`;
    }, [label, multiple, selectedCount]);

    const isSelected = useCallback(
        (optionValue: Value) => {
            if (multiple) {
                return ((value as Value[] | undefined) ?? []).includes(optionValue);
            }
            return value === optionValue;
        },
        [multiple, value]
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

    const handleSelect = useCallback((selectedValue: Value) => {
        if (multiple) {
            const currentValues = (value as Value[] | undefined) ?? [];
            const nextValues = currentValues.includes(selectedValue)
                ? currentValues.filter((v) => v !== selectedValue)
                : [...currentValues, selectedValue];
            (onChange as (value: Value[]) => void)(nextValues);
        } else {
            (onChange as (value: Value) => void)(selectedValue);
            setIsOpen(false);
        }
    }, [multiple, onChange, value, setIsOpen]);

    const showMessage = (loading || emptyMessage) && options.length === 0;

    const canShowDropdown = searchable
        ? (inputValue?.trim().length ?? 0) >= (minQueryLength ?? 0)
        : true;

    return (
        <div className={styles.root} ref={wrapperRef}>
            {renderTrigger ? (
                renderTrigger({ isOpen, onClick: () => setIsOpen(!isOpen) })
            ) : searchable ? (
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
                    <span className={styles.label}>
                        {triggerLabel}
                        {required && <span className={styles.required}>*</span>}
                    </span>
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
                <div
                    className={clsx(
                        styles.dropdown,
                        dropdownAlign === 'right' ? styles.dropdownAlignRight : styles.dropdownAlignLeft,
                        dropdownClassName
                    )}
                >
                    {showMessage ? (
                        <div className={styles.message}>
                            {loading ? 'Загрузка...' : emptyMessage}
                        </div>
                    ) : (
                        <ul
                            className={styles.list}
                            role="listbox"
                            aria-label={label}
                            aria-multiselectable={multiple || undefined}
                        >
                            {options.map((option, index) => (
                                <li
                                    key={option.value}
                                    className={clsx(
                                        styles.listItem,
                                        isSelected(option.value) && styles.listItemSelected,
                                        activeIndex === index && styles.listItemActive
                                    )}
                                    role="option"
                                    aria-selected={isSelected(option.value)}
                                    data-active={activeIndex === index}
                                    onClick={() => handleSelect(option.value)}
                                >
                                    {option.label}
                                </li>
                            ))}
                            {footerRow}
                        </ul>
                    )}
                </div>
            )}
        </div>
    );
}
