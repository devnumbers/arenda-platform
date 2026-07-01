'use client';

import type { JSX } from 'react';
import { ROUTES } from '@/shared/config/routes';
import { TenantForm } from './TenantForm';
import type { TenantContactFormData } from './TenantForm';

export interface TenantFormStepProps {
  readonly initialData?: Partial<TenantContactFormData>;
  readonly onSubmit: (data: TenantContactFormData) => void;
  readonly onChange?: (data: TenantContactFormData) => void;
  readonly isLoading: boolean;
  readonly error?: string;
}

export function TenantFormStep({
  initialData,
  onSubmit,
  onChange,
  isLoading,
  error,
}: TenantFormStepProps): JSX.Element {
  return (
    <div>
      <TenantForm
        initialData={initialData}
        submitLabel="Добавить арендатора"
        isLoading={isLoading}
        error={error}
        onSubmit={onSubmit}
        onChange={onChange}
        backHref={ROUTES.tenants}
      />
    </div>
  );
}
