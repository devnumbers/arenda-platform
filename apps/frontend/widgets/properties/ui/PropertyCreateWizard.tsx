'use client';

import {type JSX, useEffect, useState} from 'react';
import {useRouter} from 'next/navigation';
import {ROUTES} from '@/shared/config/routes';
import {goBack} from '@/shared/lib/navigation';
import {useCreateProperty} from '@/features/properties/api';
import {type CreateStep, usePropertyCreateDraft,} from '@/widgets/properties/lib/use-property-create-draft';
import {WizardHeader} from '@/shared/ui/wizard-header';
import {PropertyAddressStep} from './PropertyAddressStep';
import {PropertyInfoStep} from './PropertyInfoStep';
import {PropertySuccessStep} from './PropertySuccessStep';
import {PropertyTypeStep} from './PropertyTypeStep';
import styles from './PropertyCreateWizard.module.css';

export function PropertyCreateWizard(): JSX.Element {
    const router = useRouter();
    const {draft, setDraft} = usePropertyCreateDraft();
    const [isSubmitting, setIsSubmitting] = useState(false);
    const createProperty = useCreateProperty();

    useEffect(() => {
        if (draft.step !== 3) return;

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
        const {name, type, address, description} = draft;

        if (!type || !address) {
            router.push(ROUTES.properties);
            return;
        }

        setIsSubmitting(true);
        try {
            await createProperty.mutateAsync({
                name: name ?? '',
                type,
                address,
                description,
            });

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
            <WizardHeader
                title="Создание объекта"
                step={draft.step}
                totalSteps={3}
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
                {draft.step === 3 && (
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
