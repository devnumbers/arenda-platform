'use client';

import {useParams, useRouter} from 'next/navigation';
import type {JSX} from 'react';
import {useCallback, useMemo, useState} from 'react';
import {notify} from '@/shared/lib/toast';
import {ROUTES} from '@/shared/config/routes';
import {
    useArchiveProperty,
    useProperty,
    useUnarchiveProperty,
    useUpdateProperty
} from '@/features/properties/api/hooks';
import {useCompleteLease, usePropertyLeases, useReturnDeposit,} from '@/features/leases/api/hooks';
import {
    type OperationsFilters,
    useOperations,
    useOperationsByProperty,
    usePropertyOperationsSummary,
} from '@/features/operations/api/hooks';
import {ApiError} from '@/shared/api/errors';
import {findCurrentLease, getPropertyPageStatus,} from '../lib/get-property-page-status';
import {PropertyDetailHeader} from './PropertyDetailHeader';
import {PropertyGallery} from './PropertyGallery';
import {PropertyStatusSection} from './PropertyStatusSection';
import {PropertyLeaseCard} from './PropertyLeaseCard';
import {PropertyTenantCard} from './PropertyTenantCard';
import {PropertyPaymentsCard} from './PropertyPaymentsCard';
import {PropertyOverdueOperationsCard} from './PropertyOverdueOperationsCard';
import {PropertyOperationsCard} from './PropertyOperationsCard';
import {PropertyInfoCard} from './PropertyInfoCard';
import {PropertyActionMenu} from './PropertyActionMenu';
import {PropertyBlockedModal} from './PropertyBlockedModal';
import {PropertyEndLeaseModal} from './PropertyEndLeaseModal';
import {PropertyDepositReturnModal} from './PropertyDepositReturnModal';
import {PropertySuccessBanner} from './PropertySuccessBanner';
import {PropertyDetailLoading} from './PropertyDetailLoading';
import {PropertyDetailError} from './PropertyDetailError';
import styles from './PropertyDetailPage.module.css';

function showMutationError(error: ApiError): void {
    notify.error(error);
}

export function PropertyDetailPage(): JSX.Element {
    const params = useParams<{ id: string }>();
    const id = params.id ?? '';
    const router = useRouter();

    const propertyQuery = useProperty(id);
    const leasesQuery = usePropertyLeases(id);
    const summaryQuery = usePropertyOperationsSummary(id);
    const operationsQuery = useOperationsByProperty(id);

    const updateProperty = useUpdateProperty();
    const archiveProperty = useArchiveProperty();
    const unarchiveProperty = useUnarchiveProperty();
    const completeLease = useCompleteLease();
    const returnDeposit = useReturnDeposit();

    const [blockedOpen, setBlockedOpen] = useState(false);
    const [endLeaseOpen, setEndLeaseOpen] = useState(false);
    const [depositOpen, setDepositOpen] = useState(false);
    const [successBannerOpen, setSuccessBannerOpen] = useState(false);
    const [selectedLeaseId, setSelectedLeaseId] = useState<string>('');

    const property = propertyQuery.data;
    const leases = useMemo(() => leasesQuery.data?.items ?? [], [leasesQuery.data]);

    const pageStatus = useMemo(
        () => (property ? getPropertyPageStatus(property.status, leases) : 'free'),
        [property, leases],
    );
    const currentLease = useMemo(() => findCurrentLease(leases), [leases]);
    const selectedLease = useMemo(
        () => leases.find((lease) => lease.id === selectedLeaseId),
        [leases, selectedLeaseId],
    );
    const payableRentFilters = useMemo<OperationsFilters>(
        () => ({
            lease_id: currentLease?.id,
            category: ['rent'],
            status: ['pending', 'overdue'],
            sort: 'operation_date_asc',
            limit: 1,
        }),
        [currentLease?.id],
    );
    const payableRentQuery = useOperations(payableRentFilters, {
        enabled: Boolean(currentLease?.id),
    });
    const isPayRentLoading = Boolean(currentLease) && payableRentQuery.isFetching;

    const handleToggleMaintenance = useCallback(() => {
        if (!property) return;
        if (property.status === 'active') {
            if (currentLease) {
                setBlockedOpen(true);
                return;
            }
            updateProperty.mutate(
                {id, data: {status: 'maintenance'}},
                {onError: showMutationError},
            );
        } else {
            updateProperty.mutate(
                {id, data: {status: 'active'}},
                {onError: showMutationError},
            );
        }
    }, [property, currentLease, id, updateProperty]);

    const handleToggleArchive = useCallback(() => {
        if (!property) return;
        if (property.status === 'archived') {
            unarchiveProperty.mutate(id, {onError: showMutationError});
        } else {
            if (currentLease) {
                setBlockedOpen(true);
                return;
            }
            archiveProperty.mutate(id, {onError: showMutationError});
        }
    }, [property, currentLease, id, archiveProperty, unarchiveProperty]);

    const handleEndLease = useCallback(() => {
        if (currentLease) {
            setSelectedLeaseId(currentLease.id);
            setEndLeaseOpen(true);
        }
    }, [currentLease]);

    const confirmEndLease = useCallback(() => {
        if (!selectedLeaseId) return;
        completeLease.mutate(selectedLeaseId, {
            onSuccess: () => {
                setEndLeaseOpen(false);
                setSuccessBannerOpen(true);
            },
            onError: showMutationError,
        });
    }, [selectedLeaseId, completeLease]);

    const handleDepositReturn = useCallback(() => {
        const leaseId = selectedLeaseId || currentLease?.id;
        if (leaseId) {
            setSelectedLeaseId(leaseId);
            setDepositOpen(true);
        }
    }, [currentLease?.id, selectedLeaseId]);

    const confirmDepositReturn = useCallback(() => {
        if (!selectedLeaseId) return;
        returnDeposit.mutate(selectedLeaseId, {
            onSuccess: () => setDepositOpen(false),
            onError: showMutationError,
        });
    }, [selectedLeaseId, returnDeposit]);

    const handleEdit = useCallback(() => {
        router.push(ROUTES.propertyEdit(id));
    }, [id, router]);

    const handlePayRent = useCallback(() => {
        if (currentLease) {
            if (payableRentQuery.isFetching) return;
            const nextRentOperation = payableRentQuery.data?.items[0];
            router.push(
                nextRentOperation
                    ? ROUTES.financeOperation(nextRentOperation.id)
                    : ROUTES.lease(currentLease.id),
            );
            return;
        }

        router.push(ROUTES.propertyOperations(id));
    }, [currentLease, id, payableRentQuery.data, payableRentQuery.isFetching, router]);

    const handleBlockedContinue = useCallback(() => {
        setBlockedOpen(false);
        handleEndLease();
    }, [handleEndLease]);

    const hasAnyError =
        propertyQuery.isError ||
        leasesQuery.isError ||
        summaryQuery.isError ||
        operationsQuery.isError;

    const isLoading = propertyQuery.isPending || leasesQuery.isPending;

    return (
        <div className={styles.root}>
            <PropertyDetailHeader
                title="Мой объект"
                actions={
                    <PropertyActionMenu
                        status={property?.status}
                        disabled={isLoading || hasAnyError || !property}
                        onEdit={handleEdit}
                        onToggleMaintenance={handleToggleMaintenance}
                        onToggleArchive={handleToggleArchive}
                    />
                }
            />

            {isLoading && <PropertyDetailLoading/>}

            {!isLoading && (hasAnyError || !property) && (
                <PropertyDetailError
                    onRetry={() => {
                        propertyQuery.refetch();
                        leasesQuery.refetch();
                        summaryQuery.refetch();
                        operationsQuery.refetch();
                    }}
                    isLoading={
                        propertyQuery.isFetching ||
                        leasesQuery.isFetching ||
                        summaryQuery.isFetching ||
                        operationsQuery.isFetching
                    }
                />
            )}

            {!isLoading && !hasAnyError && property && (
                <>
                    <PropertyGallery/>

                    <PropertyStatusSection
                        property={property}
                        leases={leases}
                        summary={summaryQuery.data}
                    />

                    <PropertyLeaseCard
                        lease={currentLease}
                        status={pageStatus}
                        overdueRentCount={summaryQuery.data?.overdue_rent_count ?? 0}
                        propertyId={id}
                        onPayRent={handlePayRent}
                        isPayRentLoading={isPayRentLoading}
                        onEndLease={handleEndLease}
                    />

                    <PropertyTenantCard lease={property.activeLease}/>

                    <PropertyPaymentsCard
                        property={property}
                        operations={operationsQuery.data?.items ?? []}
                        overdueCount={summaryQuery.data?.overdue_total_count ?? 0}
                    />

                    <PropertyOverdueOperationsCard propertyId={id}/>

                    <PropertyOperationsCard
                        propertyName={property.name}
                        summary={summaryQuery.data}
                    />

                    <PropertyInfoCard description={property.description} propertyId={id}/>
                </>
            )}

            <PropertyBlockedModal
                isOpen={blockedOpen}
                onClose={() => setBlockedOpen(false)}
                onContinue={handleBlockedContinue}
            />

            <PropertyEndLeaseModal
                isOpen={endLeaseOpen}
                onClose={() => setEndLeaseOpen(false)}
                onConfirm={confirmEndLease}
            />

            <PropertyDepositReturnModal
                isOpen={depositOpen}
                onClose={() => setDepositOpen(false)}
                onConfirm={confirmDepositReturn}
                depositAmountKopecks={
                    selectedLease?.deposit_amount_kopecks ??
                    currentLease?.deposit_amount_kopecks ??
                    0
                }
            />

            {successBannerOpen && (
                <PropertySuccessBanner
                    onOpenLease={() => router.push(ROUTES.lease(selectedLeaseId))}
                    onDepositReturn={handleDepositReturn}
                />
            )}
        </div>
    );
}
