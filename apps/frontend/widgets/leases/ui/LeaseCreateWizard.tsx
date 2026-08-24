'use client';

import {type JSX, useEffect, useState} from 'react';
import {useRouter} from 'next/navigation';
import {useQueryClient} from '@tanstack/react-query';
import {propertyKeys, useProperty} from '@/features/properties';

import {useCreateLease, useLeases} from '@/features/leases';
import {useTenantContacts} from '@/features/tenant-contacts';
import {isOpenLeaseStatus} from '@/entities/lease';
import {notify} from '@/shared/lib/notifications';
import {parseRublesToKopecks} from '@/shared/lib/format-money';
import {ROUTES} from '@/shared/config/routes';
import {goBack, RETURN_TO_PARAM} from '@/shared/lib/navigation';
import {Button} from '@/shared/ui/button';
import {LinkButton} from '@/shared/ui/link-button';
import {type LeaseCreateStep, useLeaseCreateDraft} from '../lib/use-lease-create-draft';
import {WizardHeader} from '@/shared/ui/wizard-header';
import {LeasePriceStep} from './LeasePriceStep';
import {LeaseDatesStep} from './LeaseDatesStep';
import {LeaseSuccessStep} from './LeaseSuccessStep';
import styles from './LeaseCreateWizard.module.css';

const UUID_REGEX = /^[0-9a-fA-F-]{36}$/;

export type LeaseCreateWizardProps = {
    readonly propertyId?: string;
    readonly preselectedTenantContactId?: string;
};

export function LeaseCreateWizard({propertyId, preselectedTenantContactId}: LeaseCreateWizardProps): JSX.Element {
    const router = useRouter();
    const queryClient = useQueryClient();
    const {draft, setDraft} = useLeaseCreateDraft();
    const [isSubmitting, setIsSubmitting] = useState(false);
    const [createdLeaseId, setCreatedLeaseId] = useState<string | undefined>(undefined);

    const propertyQuery = useProperty(propertyId ?? '');
    const allLeasesQuery = useLeases();
    const propertyLeasesQuery = {
        ...allLeasesQuery,
        data: allLeasesQuery.data?.filter((lease) => lease.propertyId === propertyId),
    };
    const createLease = useCreateLease();

    useEffect(() => {
        if (!propertyId || !UUID_REGEX.test(propertyId)) {
            router.replace(ROUTES.properties);
            return;
        }
    }, [propertyId, router]);

    useEffect(() => {
        if (!preselectedTenantContactId) return;
        // An explicit tenantContactId from the URL wins over the persisted draft.
        // Declared after useLeaseCreateDraft so it applies on top of the loaded draft.
        setDraft((prev) => ({...prev, tenantContactId: preselectedTenantContactId}));
    }, [preselectedTenantContactId, setDraft]);

    const {data: tenantContacts, isLoading: isTenantContactsLoading} = useTenantContacts();
    const isTenantListEmpty = !isTenantContactsLoading && tenantContacts?.length === 0;

    const currentUrl = propertyId
        ? `${ROUTES.leaseNew}?propertyId=${propertyId}`
        : ROUTES.leaseNew;
    const createTenantHref = `${ROUTES.tenantNew}?${RETURN_TO_PARAM}=${encodeURIComponent(currentUrl)}`;

    const openLease = propertyLeasesQuery.data?.find((lease) =>
        isOpenLeaseStatus(lease.status),
    );
    const isPropertyBlocked =
        propertyQuery.data !== undefined && propertyQuery.data.status !== 'active';
    const isLeaseBlocked = openLease !== undefined;

    const handleCancel = () => {
        goBack(router, ROUTES.properties);
    };

    const handleBack = () => {
        if (draft.step === 1) {
            goBack(router, ROUTES.properties);
            return;
        }
        setDraft((prev) => ({...prev, step: ((prev.step - 1) as LeaseCreateStep)}));
    };

    const handleNext = () => {
        setDraft((prev) => ({...prev, step: ((prev.step + 1) as LeaseCreateStep)}));
    };

    const handleSubmit = async () => {
        if (!draft.rentAmount || draft.paymentDay === undefined || !draft.startDate) return;
        if (!propertyId) return;

        const rentKopecks = parseRublesToKopecks(draft.rentAmount, {positive: true});
        const depositKopecks = parseRublesToKopecks(draft.depositAmount?.length ? draft.depositAmount : '0');
        if (rentKopecks === undefined || depositKopecks === undefined) return;

        setIsSubmitting(true);

        try {
            const lease = await createLease.mutateAsync({
                propertyId,
                rentKopecks,
                depositKopecks,
                paymentDay: draft.paymentDay,
                startDate: draft.startDate,
                endDate: draft.endDate?.length ? draft.endDate : undefined,
                tenantContactId: draft.tenantContactId?.length ? draft.tenantContactId : undefined,
            });

            setCreatedLeaseId(lease.id);

            void queryClient.invalidateQueries({queryKey: propertyKeys.all});
            void queryClient.invalidateQueries({queryKey: propertyKeys.list});
            void queryClient.invalidateQueries({queryKey: propertyKeys.detail(propertyId)});

            handleNext();
        } catch (error: unknown) {
            console.error('Failed to create lease', error);
            notify.scenarios.leases.leaseCreateError(error);
        } finally {
            setIsSubmitting(false);
        }
    };

    if (draft.step === 3) {
        return (
            <div className={styles.root}>
                <LeaseSuccessStep
                    onAddLater={() => goBack(router, ROUTES.properties)}
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

    if (propertyQuery.isError || propertyLeasesQuery.isError) {
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
                        tenantContactId={draft.tenantContactId ?? ''}
                        onTenantChange={(tenantContactId) =>
                            setDraft((prev) => ({...prev, tenantContactId}))
                        }
                        tenantContacts={tenantContacts}
                        isTenantListEmpty={isTenantListEmpty}
                        isTenantContactsLoading={isTenantContactsLoading}
                        createTenantHref={createTenantHref}
                        paymentDay={draft.paymentDay}
                        startDate={draft.startDate}
                        endDate={draft.endDate}
                        onPaymentDayChange={(paymentDay) =>
                            setDraft((prev) => ({...prev, paymentDay}))
                        }
                        onStartDateChange={(startDate) => setDraft((prev) => ({...prev, startDate}))}
                        onEndDateChange={(endDate) => setDraft((prev) => ({...prev, endDate}))}
                        onSubmit={() => void handleSubmit()}
                        isLoading={isSubmitting}
                    />
                )}
            </div>
        </div>
    );
}
