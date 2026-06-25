'use client';

import { useEffect, useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ROUTES } from '@/shared/config/routes';
import { useCreateProperty, useUploadPropertyPhoto } from '@/features/properties/api';
import {
  usePropertyCreateDraft,
  type CreateStep,
} from '@/widgets/properties/lib/use-property-create-draft';
import { PropertyCreateHeader } from './PropertyCreateHeader';
import { PropertyAddressStep } from './PropertyAddressStep';
import { PropertyInfoStep } from './PropertyInfoStep';
import { PropertySuccessStep } from './PropertySuccessStep';
import { PropertyTypeStep } from './PropertyTypeStep';
import styles from './PropertyCreateWizard.module.css';

export function PropertyCreateWizard(): JSX.Element {
  const router = useRouter();
  const { draft, setDraft } = usePropertyCreateDraft();
  const [photos, setPhotos] = useState<File[]>([]);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const createProperty = useCreateProperty();
  const uploadPhoto = useUploadPropertyPhoto();

  useEffect(() => {
    if (draft.step !== 3) return;

    if (!draft.type) {
      setDraft((prev) => ({ ...prev, step: 1 }));
    } else if (!draft.address) {
      setDraft((prev) => ({ ...prev, step: 2 }));
    }
  }, [draft.step, draft.type, draft.address, setDraft]);

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

  const handleCreate = async () => {
    const { name, type, address, description } = draft;

    if (!type || !address) {
      router.push(ROUTES.properties);
      return;
    }

    setIsSubmitting(true);
    try {
      const property = await createProperty.mutateAsync({
        name: name ?? '',
        type,
        address,
        description,
      });

      if (photos.length > 0) {
        await Promise.all(
          photos.map((file) =>
            uploadPhoto
              .mutateAsync({ propertyId: property.id, file })
              .catch((error: unknown) => {
                console.error('Failed to upload property photo', error);
              }),
          ),
        );
      }

      handleNext();
    } catch (error: unknown) {
      console.error('Failed to create property', error);
    } finally {
      setIsSubmitting(false);
    }
  };

  if (draft.step === 4) {
    return (
      <div className={styles.root}>
        <PropertySuccessStep
          onAddLater={() => router.push(ROUTES.properties)}
          onCreateLease={() => router.push(ROUTES.tenants)}
        />
      </div>
    );
  }

  return (
    <div className={styles.root}>
      <PropertyCreateHeader
        step={draft.step}
        onBack={handleBack}
        onCancel={handleCancel}
      />
      <div className={styles.content}>
        {draft.step === 1 && (
          <PropertyTypeStep
            value={draft.type}
            onChange={(type) => setDraft((prev) => ({ ...prev, type }))}
            onNext={handleNext}
          />
        )}
        {draft.step === 2 && (
          <PropertyAddressStep
            value={draft.address}
            onChange={(address) => setDraft((prev) => ({ ...prev, address }))}
            onNext={handleNext}
            onBack={handleBack}
          />
        )}
        {draft.step === 3 && (
          <PropertyInfoStep
            name={draft.name}
            description={draft.description}
            onNameChange={(name) => setDraft((prev) => ({ ...prev, name }))}
            onDescriptionChange={(description) =>
              setDraft((prev) => ({ ...prev, description }))
            }
            draftType={draft.type}
            draftAddress={draft.address}
            photos={photos}
            onPhotosChange={setPhotos}
            onSubmit={handleCreate}
            onBack={handleBack}
            isLoading={isSubmitting}
          />
        )}
      </div>
    </div>
  );
}
