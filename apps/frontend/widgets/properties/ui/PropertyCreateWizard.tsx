'use client';

import { useRouter } from 'next/navigation';
import type { JSX } from 'react';
import { ROUTES } from '@/shared/config/routes';
import { usePropertyCreateDraft, type CreateStep } from '@/widgets/properties/lib/use-property-create-draft';
import { PropertyCreateHeader } from './PropertyCreateHeader';
import { PropertyAddressStep } from './PropertyAddressStep';
import { PropertyInfoStep } from './PropertyInfoStep';
import { PropertySuccessStep } from './PropertySuccessStep';
import { PropertyTypeStep } from './PropertyTypeStep';
import styles from './PropertyCreateWizard.module.css';

export function PropertyCreateWizard(): JSX.Element {
  const router = useRouter();
  const { draft, setDraft } = usePropertyCreateDraft();

  const handleCancel = () => {
    router.push(ROUTES.properties);
  };

  const handleBack = () => {
    if (draft.step === 1) {
      router.push(ROUTES.properties);
      return;
    }

    setDraft((prev) => ({ ...prev, step: ((prev.step - 1) as CreateStep) }));
  };

  const handleNext = () => {
    setDraft((prev) => ({ ...prev, step: ((prev.step + 1) as CreateStep) }));
  };

  if (draft.step === 4) {
    return (
      <div className={styles.root}>
        <PropertySuccessStep onNext={handleNext} onBack={handleBack} />
      </div>
    );
  }

  return (
    <div className={styles.root}>
      <PropertyCreateHeader step={draft.step} onBack={handleBack} onCancel={handleCancel} />
      <div className={styles.content}>
        {draft.step === 1 && <PropertyTypeStep onNext={handleNext} onBack={handleBack} />}
        {draft.step === 2 && <PropertyAddressStep onNext={handleNext} onBack={handleBack} />}
        {draft.step === 3 && <PropertyInfoStep onNext={handleNext} onBack={handleBack} />}
      </div>
    </div>
  );
}
