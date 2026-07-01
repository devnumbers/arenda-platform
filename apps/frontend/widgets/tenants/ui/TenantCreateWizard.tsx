'use client';

import { useCallback, useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { useCreateTenantContact } from '@/features/tenant-contacts/api';
import { ApiError } from '@/shared/api/errors';
import { ROUTES } from '@/shared/config/routes';
import { WizardHeader } from '@/shared/ui/wizard-header';
import { useTenantCreateDraft } from '../lib/use-tenant-create-draft';
import { TenantFormStep } from './TenantFormStep';
import { TenantSuccessStep } from './TenantSuccessStep';
import styles from './TenantCreateWizard.module.css';
import type { TenantContactFormData } from './TenantForm';

function formatErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    return error.detail;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return 'Не удалось добавить арендатора. Попробуйте ещё раз.';
}

export function TenantCreateWizard(): JSX.Element {
  const router = useRouter();
  const { draft, isLoaded, setDraft, clearDraft } = useTenantCreateDraft();
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | undefined>(undefined);

  const createTenantContact = useCreateTenantContact();

  const handleClose = () => {
    clearDraft();
    router.push(ROUTES.tenants);
  };

  const handleChange = useCallback(
    (data: TenantContactFormData) => {
      setDraft((prev) => ({
        ...prev,
        name: data.name,
        surname: data.surname,
        patronymic: data.patronymic,
        phone: data.phone,
        email: data.email,
        comment: data.comment,
      }));
    },
    [setDraft],
  );

  const handleSubmit = async (data: TenantContactFormData) => {
    if (data.name.trim() === '') return;

    setIsSubmitting(true);
    setSubmitError(undefined);

    try {
      await createTenantContact.mutateAsync({
        name: data.name.trim(),
        surname: data.surname.trim() || undefined,
        patronymic: data.patronymic.trim() || undefined,
        phone: data.phone.trim() || undefined,
        email: data.email.trim() || undefined,
        comment: data.comment.trim() || undefined,
      });

      setDraft((prev) => ({ ...prev, step: 'success' }));
    } catch (error: unknown) {
      console.error('Failed to create tenant contact', error);
      setSubmitError(formatErrorMessage(error));
    } finally {
      setIsSubmitting(false);
    }
  };

  if (!isLoaded) {
    return (
      <div className={styles.root}>
        <WizardHeader
          title="Добавление арендатора"
          step={1}
          totalSteps={1}
          onCancel={handleClose}
        />
        <div className={styles.content}>Загрузка…</div>
      </div>
    );
  }

  if (draft.step === 'success') {
    return (
      <div className={styles.root}>
        <TenantSuccessStep
          onAddLater={() => {
            clearDraft();
            router.push(ROUTES.tenants);
          }}
          onAddOperations={() => {
            clearDraft();
            router.push(ROUTES.finance);
          }}
        />
      </div>
    );
  }

  return (
    <div className={styles.root}>
      <WizardHeader
        title="Добавление арендатора"
        step={1}
        totalSteps={1}
        onCancel={handleClose}
      />
      <div className={styles.content}>
        <TenantFormStep
          initialData={{
            name: draft.name,
            surname: draft.surname,
            patronymic: draft.patronymic,
            phone: draft.phone,
            email: draft.email,
            comment: draft.comment,
          }}
          isLoading={isSubmitting}
          error={submitError}
          onSubmit={handleSubmit}
          onChange={handleChange}
        />
      </div>
    </div>
  );
}
