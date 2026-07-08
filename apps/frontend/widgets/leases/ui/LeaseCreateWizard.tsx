'use client';

import {type JSX, useEffect, useState} from 'react';
import {useRouter} from 'next/navigation';
import {useQueryClient} from '@tanstack/react-query';
import {propertyKeys, useProperty} from '@/features/properties/api';

import {useCreateLease, usePropertyLeases} from '@/features/leases/api';
import {isOpenLeaseStatus} from '@/entities/lease/lib/status';
import {ApiError} from '@/shared/api/errors';
import {ROUTES} from '@/shared/config/routes';
import {Button} from '@/shared/ui/button';
import {LinkButton} from '@/shared/ui/link-button';
import {type LeaseCreateStep, useLeaseCreateDraft} from '../lib/use-lease-create-draft';
import {WizardHeader} from '@/shared/ui/wizard-header';
import {LeasePriceStep} from './LeasePriceStep';
import {LeaseDatesStep} from './LeaseDatesStep';
import {LeaseSuccessStep} from './LeaseSuccessStep';
import styles from './LeaseCreateWizard.module.css';

const UUID_REGEX = /^[0-9a-fA-F-]{36}$/;

function formatErrorMessage(error: unknown): string {
    if (error instanceof ApiError) {
        if (error.code === 'ErrOpenLeaseExists') {
            return 'У этого объекта уже есть активная аренда. Завершите текущую аренду перед созданием новой.';
        }
        return error.detail;
    }
    if (error instanceof Error) {
        return error.message;
    }
    return 'Не удалось создать аренду. Попробуйте ещё раз.';
}

export type LeaseCreateWizardProps = {
    readonly propertyId?: string;
};

export function LeaseCreateWizard({propertyId}: LeaseCreateWizardProps): JSX.Element {
    const router = useRouter();
    const queryClient = useQueryClient();
    const {draft, setDraft} = useLeaseCreateDraft();
    const [isSubmitting, setIsSubmitting] = useState(false);
    const [submitError, setSubmitError] = useState<string | undefined>(undefined);
    const [createdLeaseId, setCreatedLeaseId] = useState<string | undefined>(undefined);

    const propertyQuery = useProperty(propertyId ?? '');
    const propertyLeasesQuery = usePropertyLeases(propertyId ?? '');
    const createLease = useCreateLease();

    useEffect(() => {
        if (!propertyId || !UUID_REGEX.test(propertyId)) {
            router.replace(ROUTES.properties);
            return;
        }
    }, [propertyId, router]);

    const openLease = propertyLeasesQuery.data?.items.find((lease) =>
        isOpenLeaseStatus(lease.status),
    );
    const isPropertyBlocked =
        propertyQuery.data !== undefined && propertyQuery.data.status !== 'active';
    const isLeaseBlocked = openLease !== undefined;

    const handleCancel = () => {
        router.push(ROUTES.properties);
    };

    const handleBack = () => {
        setSubmitError(undefined);
        if (draft.step === 1) {
            router.push(ROUTES.properties);
            return;
        }
        setDraft((prev) => ({...prev, step: ((prev.step - 1) as LeaseCreateStep)}));
    };

    const handleNext = () => {
        setSubmitError(undefined);
        setDraft((prev) => ({...prev, step: ((prev.step + 1) as LeaseCreateStep)}));
    };

    const handleSubmit = async () => {
        if (!draft.rentAmount || draft.paymentDay === undefined || !draft.startDate) return;
        if (!propertyId) return;

        setIsSubmitting(true);
        setSubmitError(undefined);

        try {
            const lease = await createLease.mutateAsync({
                property_id: propertyId,
                rent_amount_kopecks: Math.round(Number(draft.rentAmount) * 100),
                deposit_amount_kopecks: Math.round(Number(draft.depositAmount || '0') * 100),
                payment_day: draft.paymentDay,
                start_date: draft.startDate,
                end_date: draft.endDate || undefined,
                tenant_contact_id: undefined,
            });

            setCreatedLeaseId(lease.id);

            queryClient.invalidateQueries({queryKey: propertyKeys.all});
            queryClient.invalidateQueries({queryKey: propertyKeys.list});
            queryClient.invalidateQueries({queryKey: propertyKeys.detail(propertyId)});

            handleNext();
        } catch (error: unknown) {
            console.error('Failed to create lease', error);
            setSubmitError(formatErrorMessage(error));
        } finally {
            setIsSubmitting(false);
        }
    };

    if (draft.step === 3) {
        return (
            <div className={styles.root}>
                <LeaseSuccessStep
                    onAddLater={() => router.push(ROUTES.properties)}
                    leaseId={createdLeaseId ?? ''}
                />
            </div>
        );
    }

    if (propertyQuery.isPending || propertyLeasesQuery.isPending) {
        return (
            <div className={styles.root}>
                <WizardHeader
                    title="Создание аренды"
                    step={draft.step}
                    totalSteps={2}
                    onBack={handleBack}
                    onCancel={handleCancel}
                />
            </div>
        );
    }

    if (propertyQuery.isError || propertyLeasesQuery.isError || !propertyQuery.data) {
        return (
            <div className={styles.root}>
                <WizardHeader
                    title="Создание аренды"
                    step={draft.step}
                    totalSteps={2}
                    onBack={handleBack}
                    onCancel={handleCancel}
                />
                <div className={styles.content}>
                    <div className={styles.blocked}>
                        <h2 className={styles.blockedTitle}>Не удалось проверить объект</h2>
                        <p className={styles.blockedText}>
                            Повторите загрузку, чтобы проверить доступность объекта для новой аренды.
                        </p>
                        <Button
                            type="button"
                            variant="primary"
                            size="medium"
                            onClick={() => {
                                void propertyQuery.refetch();
                                void propertyLeasesQuery.refetch();
                            }}
                        >
                            Повторить
                        </Button>
                    </div>
                </div>
            </div>
        );
    }

    if (isPropertyBlocked || isLeaseBlocked) {
        return (
            <div className={styles.root}>
                <WizardHeader
                    title="Создание аренды"
                    step={draft.step}
                    totalSteps={2}
                    onBack={handleBack}
                    onCancel={handleCancel}
                />
                <div className={styles.content}>
                    <div className={styles.blocked}>
                        <h2 className={styles.blockedTitle}>
                            {isLeaseBlocked ? 'У объекта уже есть открытая аренда' : 'Аренда недоступна'}
                        </h2>
                        <p className={styles.blockedText}>
                            {isLeaseBlocked
                                ? 'Завершите текущую аренду перед созданием новой.'
                                : 'Создать аренду можно только для активного объекта.'}
                        </p>
                        <LinkButton
                            href={openLease ? ROUTES.lease(openLease.id) : ROUTES.properties}
                            variant="primary"
                            size="medium"
                        >
                            {openLease ? 'Открыть аренду' : 'К объектам'}
                        </LinkButton>
                    </div>
                </div>
            </div>
        );
    }

    return (
        <div className={styles.root}>
            <WizardHeader title="Создание аренды" step={draft.step} totalSteps={2} onBack={handleBack}
                          onCancel={handleCancel}/>
            <div className={styles.content}>
                {draft.step === 1 && (
                    <LeasePriceStep
                        rentAmount={draft.rentAmount ?? ''}
                        depositAmount={draft.depositAmount ?? ''}
                        onRentChange={(rentAmount) => setDraft((prev) => ({...prev, rentAmount}))}
                        onDepositChange={(depositAmount) =>
                            setDraft((prev) => ({...prev, depositAmount}))
                        }
                        onNext={handleNext}
                    />
                )}
                {draft.step === 2 && (
                    <LeaseDatesStep
                        paymentDay={draft.paymentDay}
                        startDate={draft.startDate}
                        endDate={draft.endDate}
                        onPaymentDayChange={(paymentDay) =>
                            setDraft((prev) => ({...prev, paymentDay}))
                        }
                        onStartDateChange={(startDate) => setDraft((prev) => ({...prev, startDate}))}
                        onEndDateChange={(endDate) => setDraft((prev) => ({...prev, endDate}))}
                        onSubmit={handleSubmit}
                        isLoading={isSubmitting}
                        error={submitError}
                    />
                )}
            </div>
        </div>
    );
}
