'use client';

import { useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { useCreateTenantContact } from '@/features/tenant-contacts/api';
import { ApiError } from '@/shared/api/errors';
import { ROUTES } from '@/shared/config/routes';
import { useTenantCreateDraft, type TenantCreateDraft } from '../lib/use-tenant-create-draft';
import { TenantCreateHeader } from './TenantCreateHeader';
import { TenantFormStep } from './TenantFormStep';
import { TenantSuccessStep } from './TenantSuccessStep';
import styles from './TenantCreateWizard.module.css';

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
  const { draft, setDraft, clearDraft } = useTenantCreateDraft();
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | undefined>(undefined);

  const createTenantContact = useCreateTenantContact();

  const handleClose = () => {
    clearDraft();
    router.push(ROUTES.tenants);
  };

  const handleFieldChange = <K extends keyof Omit<TenantCreateDraft, 'step'>>(
    field: K,
  ) =>
    (value: TenantCreateDraft[K]) => {
      setDraft((prev) => ({ ...prev, [field]: value }));
    };

  const handleSubmit = async () => {
    if (draft.name.trim() === '') return;

    setIsSubmitting(true);
    setSubmitError(undefined);

    try {
      await createTenantContact.mutateAsync({
        name: draft.name.trim(),
        surname: draft.surname.trim() || undefined,
        patronymic: draft.patronymic.trim() || undefined,
        phone: draft.phone.trim() || undefined,
        comment: draft.comment.trim() || undefined,
      });

      setDraft((prev) => ({ ...prev, step: 'success' }));
    } catch (error: unknown) {
      console.error('Failed to create tenant contact', error);
      setSubmitError(formatErrorMessage(error));
    } finally {
      setIsSubmitting(false);
    }
  };

  if (draft.step === 'success') {
    return (
      <div className={styles.root}>
        <TenantSuccessStep
          onAddLater={() => router.push(ROUTES.tenants)}
          onAddPayments={() => router.push(ROUTES.finance)}
        />
      </div>
    );
  }

  return (
    <div className={styles.root}>
      <TenantCreateHeader onClose={handleClose} />
      <div className={styles.content}>
        <TenantFormStep
          name={draft.name}
          surname={draft.surname}
          patronymic={draft.patronymic}
          phone={draft.phone}
          comment={draft.comment}
          isLoading={isSubmitting}
          error={submitError}
          onNameChange={handleFieldChange('name')}
          onSurnameChange={handleFieldChange('surname')}
          onPatronymicChange={handleFieldChange('patronymic')}
          onPhoneChange={handleFieldChange('phone')}
          onCommentChange={handleFieldChange('comment')}
          onSubmit={handleSubmit}
        />
      </div>
    </div>
  );
}
