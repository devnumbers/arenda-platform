import { type JSX } from 'react';
import { Select } from '@/shared/ui/select';
import { propertyTypeOptions } from '@/features/properties/lib/property-types';
import type { PropertyType } from '@/entities/property/model/types';

export type PropertyTypeSelectProps = {
    readonly value?: PropertyType;
    readonly onChange: (value: PropertyType) => void;
    readonly error?: string;
};

export function PropertyTypeSelect({ value, onChange, error }: PropertyTypeSelectProps): JSX.Element {
    return (
        <Select
            label="Тип"
            value={value}
            options={propertyTypeOptions}
            onChange={onChange}
            error={error}
        />
    );
}
