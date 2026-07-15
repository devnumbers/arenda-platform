'use client';

import type {JSX} from 'react';
import {ROUTES} from '@/shared/config/routes';
import type {TenantContactFormData} from './TenantForm';
import {TenantForm} from './TenantForm';

export interface TenantFormStepProps {
    readonly initialData?: Partial<TenantContactFormData>;
    readonly onSubmit: (data: TenantContactFormData) => void;
    readonly onChange?: (data: TenantContactFormData) => void;
    readonly isLoading: boolean;
}

export function TenantFormStep({
                                   initialData,
                                   onSubmit,
                                   onChange,
                                   isLoading,
                               }: TenantFormStepProps): JSX.Element {
    return (
        <div>
            <TenantForm
                initialData={initialData}
                submitLabel="Добавить арендатора"
                isLoading={isLoading}
                onSubmit={onSubmit}
                onChange={onChange}
                backHref={ROUTES.tenants}
            />
        </div>
    );
}
