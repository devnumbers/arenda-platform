'use client';

import {useMemo, type JSX} from 'react';
import {Button} from '@/shared/ui/button';
import {Select} from '@/shared/ui/select';
import styles from './PropertyFilter.module.css';

export type PropertyFilterProps = {
    readonly value?: string;
    readonly onChange: (value?: string) => void;
    readonly options: Array<{value: string; label: string}>;
    readonly loading?: boolean;
    readonly disabled?: boolean;
};

const ALL_VALUE = '';

export function PropertyFilter({
    value,
    onChange,
    options,
    loading,
    disabled,
}: PropertyFilterProps): JSX.Element {
    const hasOptions = options.length > 0;

    const selectOptions = useMemo(
        () => (hasOptions ? [{value: ALL_VALUE, label: 'Все объекты'}, ...options] : []),
        [hasOptions, options],
    );

    const selectedOption = useMemo(
        () => options.find((option) => option.value === value),
        [options, value],
    );

    const handleChange = (next: string) => {
        if (next === ALL_VALUE) {
            onChange(undefined);
            return;
        }
        onChange(next);
    };

    const selectValue = value ?? (hasOptions ? ALL_VALUE : undefined);
    const triggerLabel = value && selectedOption ? selectedOption.label : 'Все объекты';

    return (
        <Select
            value={selectValue}
            options={selectOptions}
            onChange={handleChange}
            emptyMessage="Сначала добавьте объект"
            loading={loading}
            disabled={disabled}
            renderTrigger={({isOpen, onClick}) => (
                <Button
                    variant="secondary"
                    size="small"
                    aria-label="Фильтр по объекту"
                    aria-expanded={isOpen}
                    onClick={onClick}
                    disabled={disabled}
                >
                    <span className={styles.triggerLabel}>{triggerLabel}</span>
                </Button>
            )}
        />
    );
}
