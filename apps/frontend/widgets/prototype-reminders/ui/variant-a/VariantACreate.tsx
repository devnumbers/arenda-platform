// PROTOTYPE — throwaway, issue #105
'use client';

import type { JSX } from 'react';
import { PageHeader } from '@/shared/ui/page-header';
import { prototypeHref, PROTOTYPE_ROUTES, type PrototypeVariant } from '../../model/variant';
import type { PrototypeReminderDraft } from '../../model/mocks';
import { VariantAReminderForm } from './VariantAReminderForm';
import styles from './VariantACreate.module.css';

export const variantName = 'Одноэкранная форма';

export type VariantACreateProps = {
  readonly variant: PrototypeVariant;
  readonly preselectedObjectId?: string;
};

export function VariantACreate({
  variant,
  preselectedObjectId,
}: VariantACreateProps): JSX.Element {
  const handleSubmit = (draft: PrototypeReminderDraft) => {
    console.log('[prototype#105] variant A — создать напоминание', draft);
  };

  return (
    <div className={styles.root}>
      <PageHeader
        title="Новое напоминание"
        backHref={prototypeHref(PROTOTYPE_ROUTES.objectBlock, variant)}
      />
      <VariantAReminderForm
        preselectedObjectId={preselectedObjectId}
        submitLabel="Создать напоминание"
        onSubmit={handleSubmit}
      />
    </div>
  );
}
