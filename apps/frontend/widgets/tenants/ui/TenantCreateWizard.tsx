'use client';

import {type JSX, useCallback, useState} from 'react';
import {useRouter} from 'next/navigation';
import {useCreateTenantContact} from '@/features/tenant-contacts/api';
import {notify} from '@/shared/lib/notifications';
import {ROUTES} from '@/shared/config/routes';
import {buildReturnUrl, goBack} from '@/shared/lib/navigation';
import {WizardHeader} from '@/shared/ui/wizard-header';
import {useTenantCreateDraft} from '../lib/use-tenant-create-draft';
import {TenantFormStep} from './TenantFormStep';
import {TenantSuccessStep} from './TenantSuccessStep';
import {TenantCreateWizardLoading} from './TenantCreateWizardLoading';
import styles from './TenantCreateWizard.module.css';
import type {TenantContactFormData} from './TenantForm';

export type TenantCreateWizardProps = {
    readonly returnTo?: string;
};

export function TenantCreateWizard({returnTo}: TenantCreateWizardProps): JSX.Element {
    const router = useRouter();
    const {draft, isLoaded, setDraft, clearDraft} = useTenantCreateDraft();
    const [isSubmitting, setIsSubmitting] = useState(false);

    const createTenantContact = useCreateTenantContact();

    const handleClose = () => {
        clearDraft();
        goBack(router, ROUTES.tenants);
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

        try {
            const created = await createTenantContact.mutateAsync({
                name: data.name.trim(),
                surname: data.surname.trim() || undefined,
                patronymic: data.patronymic.trim() || undefined,
                phone: data.phone.trim() || undefined,
                email: data.email.trim() || undefined,
                comment: data.comment.trim() || undefined,
            });

            if (returnTo) {
                clearDraft();
                router.replace(buildReturnUrl(returnTo, {tenantContactId: created.id}));
                return;
            }

            setDraft((prev) => ({...prev, step: 'success'}));
        } catch (error: unknown) {
            console.error('Failed to create tenant contact', error);
            notify.scenarios.tenants.tenantCreateError(error);
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
                    onBack={handleClose}
                    onCancel={handleClose}
                />
                <TenantCreateWizardLoading/>
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
                onBack={handleClose}
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
                    onSubmit={handleSubmit}
                    onChange={handleChange}
                />
            </div>
        </div>
    );
}
