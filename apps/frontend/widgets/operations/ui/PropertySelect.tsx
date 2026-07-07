'use client';

import { useMemo, type JSX } from 'react';
import { useProperties } from '@/features/properties/api';
import { Select } from '@/shared/ui/select';

export type PropertySelectProps = {
    readonly value?: string;
    readonly onChange: (propertyId: string) => void;
    readonly error?: string;
    readonly disabled?: boolean;
};

export function PropertySelect({
    value,
    onChange,
    error,
    disabled,
}: PropertySelectProps): JSX.Element {
    const { data: properties, isLoading } = useProperties();

    const options = useMemo(
        () => properties?.map((property) => ({ value: property.id, label: property.name })) ?? [],
        [properties]
    );

    const isEmpty = !isLoading && properties?.length === 0;
    const isDisabled = isLoading || isEmpty || disabled;
    const placeholder = isLoading ? 'Загрузка объектов...' : 'Выберите объект';

    return (
        <Select
            label="Объект"
            value={value}
            options={options}
            onChange={onChange}
            error={error}
            disabled={isDisabled}
            loading={isLoading}
            emptyMessage="Сначала добавьте объект"
            placeholder={placeholder}
        />
    );
}
