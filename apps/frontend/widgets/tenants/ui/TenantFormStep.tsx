'use client';

import type { JSX } from 'react';
import { TenantForm } from './TenantForm';
import type { TenantContactFormData } from './TenantForm';
import styles from './TenantFormStep.module.css';

export interface TenantFormStepProps {
  readonly initialData?: Partial<TenantContactFormData>;
  readonly onSubmit: (data: TenantContactFormData) => void;
  readonly isLoading: boolean;
  readonly error?: string;
}

export function TenantFormStep({
  initialData,
  onSubmit,
  isLoading,
  error,
}: TenantFormStepProps): JSX.Element {
  return (
    <div className={styles.root}>
      <h2 className={styles.heading}>Добавление арендатора</h2>
      <TenantForm
        initialData={initialData}
        submitLabel="Добавить арендатора"
        isLoading={isLoading}
        error={error}
        onSubmit={onSubmit}
        backHref="/tenants"
      />
    </div>
  );
}
