'use client';

import { useMemo, type JSX } from 'react';
import {
    incomeCategories,
    expenseCategories,
    type OperationType,
    type OperationCategory,
} from '@/entities/operation/model/types';
import { Select } from '@/shared/ui/select';

export type CategorySelectProps = {
    readonly type: OperationType;
    readonly value?: OperationCategory;
    readonly onChange: (category: OperationCategory) => void;
    readonly error?: string;
    readonly disabled?: boolean;
};

export function CategorySelect({
    type,
    value,
    onChange,
    error,
    disabled,
}: CategorySelectProps): JSX.Element {
    const options = useMemo(
        () => (type === 'income' ? incomeCategories : expenseCategories),
        [type]
    );

    const label = type === 'income' ? 'Категория дохода' : 'Категория расхода';

    return (
        <Select
            label={label}
            value={value}
            options={options}
            onChange={onChange}
            error={error}
            disabled={disabled}
            required
            placeholder="Выберите категорию"
        />
    );
}
