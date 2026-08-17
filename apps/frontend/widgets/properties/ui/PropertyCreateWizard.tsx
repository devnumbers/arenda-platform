'use client';

import {type JSX, useEffect, useState} from 'react';
import {useRouter} from 'next/navigation';
import {ROUTES} from '@/shared/config/routes';
import {buildReturnUrl, goBack} from '@/shared/lib/navigation';
import {useCreateProperty} from '@/features/properties';
import {filterByType} from '@/features/property-attributes';
import {
    clearPropertyCreateDraft,
    type CreateStep,
    usePropertyCreateDraft,
} from '@/widgets/properties/lib/use-property-create-draft';
import {WizardHeader} from '@/shared/ui/wizard-header';
import {PropertyAddressStep} from './PropertyAddressStep';
import {PropertyAttributesStep} from './PropertyAttributesStep';
import {PropertyInfoStep} from './PropertyInfoStep';
import {PropertySuccessStep} from './PropertySuccessStep';
import {PropertyTypeStep} from './PropertyTypeStep';
import styles from './PropertyCreateWizard.module.css';

export type PropertyCreateWizardProps = {
    readonly returnTo?: string;
};

export function PropertyCreateWizard({returnTo}: PropertyCreateWizardProps): JSX.Element {
    const router = useRouter();
    const {draft, setDraft} = usePropertyCreateDraft();
    const [isSubmitting, setIsSubmitting] = useState(false);
    const createProperty = useCreateProperty();

    useEffect(() => {
        if (draft.step !== 3 && draft.step !== 4) return;

        if (!draft.type) {
            setDraft((prev) => ({...prev, step: 1}));
        } else if (!draft.address) {
            setDraft((prev) => ({...prev, step: 2}));
        }
    }, [draft.step, draft.type, draft.address, setDraft]);

    const handleCancel = () => {
        goBack(router, ROUTES.properties);
    };

    const handleBack = () => {
        if (draft.step === 1) {
            goBack(router, ROUTES.properties);
            return;
        }

        setDraft((prev) => ({...prev, step: ((prev.step - 1) as CreateStep)}));
    };

    const handleNext = () => {
        setDraft((prev) => ({...prev, step: ((prev.step + 1) as CreateStep)}));
    };

    const handleCreate = async () => {
        const {name, type, address, description, attributes} = draft;

        if (!type || !address) {
            router.replace(ROUTES.properties);
            return;
        }

        // Drop attribute keys that do not belong to the selected type's catalog:
        // the user may have filled characteristics for another type on step 1
        // before switching. Lossless on the client side — we just don't send
        // foreign keys (the backend is the source of truth).
        const filteredAttributes = attributes ? filterByType(type, attributes) : undefined;

        setIsSubmitting(true);
        try {
            const created = await createProperty.mutateAsync({
                name: name ?? '',
                type,
                address,
                description,
                ...(filteredAttributes !== undefined && {attributes: filteredAttributes}),
            });

            if (returnTo) {
                clearPropertyCreateDraft();
                router.replace(buildReturnUrl(returnTo, {propertyId: String(created.id)}));
                return;
            }

            handleNext();
        } catch (error: unknown) {
            console.error('Failed to create property', error);
        } finally {
            setIsSubmitting(false);
        }
    };

    if (draft.step === 5) {
        return (
            <div className={styles.root}>
                <PropertySuccessStep
                    onAddLater={() => goBack(router, ROUTES.properties)}
                    onCreateLease={() => router.replace(ROUTES.tenants)}
                />
            </div>
        );
    }

    return (
        <div className={styles.root}>
            <WizardHeader
                title="Создание объекта"
                step={draft.step}
                totalSteps={4}
                onBack={handleBack}
                onCancel={handleCancel}
            />
            <div className={styles.content}>
                {draft.step === 1 && (
                    <PropertyTypeStep
                        value={draft.type}
                        onChange={(type) => setDraft((prev) => ({...prev, type}))}
                        onNext={handleNext}
                    />
                )}
                {draft.step === 2 && (
                    <PropertyAddressStep
                        value={draft.address}
                        onChange={(address) => setDraft((prev) => ({...prev, address}))}
                        onNext={handleNext}
                        onBack={handleBack}
                    />
                )}
                {draft.step === 3 && draft.type && (
                    <PropertyAttributesStep
                        type={draft.type}
                        value={draft.attributes ?? {}}
                        onChange={(attributes) => setDraft((prev) => ({...prev, attributes}))}
                        onNext={handleNext}
                        onBack={handleBack}
                    />
                )}
                {draft.step === 4 && (
                    <PropertyInfoStep
                        name={draft.name}
                        description={draft.description}
                        onNameChange={(name) => setDraft((prev) => ({...prev, name}))}
                        onDescriptionChange={(description) =>
                            setDraft((prev) => ({...prev, description}))
                        }
                        onSubmit={handleCreate}
                        isLoading={isSubmitting}
                    />
                )}
            </div>
        </div>
    );
}
