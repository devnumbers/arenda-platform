'use client';

import {useParams, useRouter} from 'next/navigation';
import type {JSX} from 'react';
import {useCallback, useMemo, useState} from 'react';
import {notify} from '@/shared/lib/notifications';
import {ROUTES} from '@/shared/config/routes';
import {
    useArchiveProperty,
    useProperty,
    useUnarchiveProperty,
    useUpdateProperty
} from '@/features/properties/api/hooks';
import {useCompleteLease, usePropertyLeases,} from '@/features/leases/api/hooks';
import {
    type OperationsFilters,
    useOperations,
    usePropertyOperationsSummary,
} from '@/features/operations/api/hooks';
import {useOperationCategories} from '@/features/operation-categories/api';
import {formatDateForApi} from '@/entities/operation/lib/dates';
import {ApiError} from '@/shared/api/errors';
import {findCurrentLease, getPropertyPageStatus,} from '../lib/get-property-page-status';
import {PropertyDetailHeader} from './PropertyDetailHeader';
import {PropertyGallery} from './PropertyGallery';
import {PropertyStatusSection} from './PropertyStatusSection';
import {PropertyLeaseCard} from './PropertyLeaseCard';
import {PropertyTenantCard} from './PropertyTenantCard';
import {PropertyOperationsSection} from './PropertyOperationsSection';
import {PropertyOperationsActions} from './PropertyOperationsActions';
import {PropertyOperationsCard} from './PropertyOperationsCard';
import {PropertyInfoCard} from './PropertyInfoCard';
import {PropertyActionMenu} from './PropertyActionMenu';
import {PropertyBlockedModal} from './PropertyBlockedModal';
import {PropertyEndLeaseModal} from './PropertyEndLeaseModal';
import {PropertySuccessBanner} from './PropertySuccessBanner';
import {PropertyDetailLoading} from './PropertyDetailLoading';
import {PropertyDetailError} from './PropertyDetailError';
import styles from './PropertyDetailPage.module.css';

function showMutationError(error: ApiError): void {
    notify.scenarios.property.saveError({description: error.detail});
}

export function PropertyDetailPage(): JSX.Element {
    const params = useParams<{ id: string }>();
    const id = params.id ?? '';
    const router = useRouter();

    const propertyQuery = useProperty(id);
    const leasesQuery = usePropertyLeases(id);
    const summaryQuery = usePropertyOperationsSummary(id);

    const updateProperty = useUpdateProperty();
    const archiveProperty = useArchiveProperty();
    const unarchiveProperty = useUnarchiveProperty();
    const completeLease = useCompleteLease();

    const [blockedOpen, setBlockedOpen] = useState(false);
    const [endLeaseOpen, setEndLeaseOpen] = useState(false);
    const [successBannerOpen, setSuccessBannerOpen] = useState(false);
    const [selectedLeaseId, setSelectedLeaseId] = useState<string>('');

    const property = propertyQuery.data;
    const leases = useMemo(() => leasesQuery.data?.items ?? [], [leasesQuery.data]);

    const pageStatus = useMemo(
        () => (property ? getPropertyPageStatus(property.status, leases) : 'free'),
        [property, leases],
    );
    const currentLease = useMemo(() => findCurrentLease(leases), [leases]);
    const categoriesQuery = useOperationCategories('income');
    const rentCategoryId = categoriesQuery.data?.find(
        (category) => category.code === 'rent',
    )?.id;
    const payableRentFilters = useMemo<OperationsFilters>(
        () => ({
            lease_id: currentLease?.id,
            category_id: rentCategoryId ? [rentCategoryId] : undefined,
            status: ['pending', 'overdue'],
            sort: 'operation_date_asc',
            limit: 1,
        }),
        [currentLease?.id, rentCategoryId],
    );
    const payableRentQuery = useOperations(payableRentFilters, {
        enabled: Boolean(currentLease?.id) && Boolean(rentCategoryId),
    });
    const isPayRentLoading = Boolean(currentLease) && payableRentQuery.isFetching;

    const overdueFilters = useMemo<Omit<OperationsFilters, 'property_id'>>(
        () => ({
            status: ['overdue'],
            sort: 'operation_date_asc',
            limit: 3,
        }),
        [],
    );
    const upcomingFilters = useMemo<Omit<OperationsFilters, 'property_id'>>(
        () => ({
            status: ['pending'],
            from: formatDateForApi(new Date()),
            sort: 'operation_date_asc',
            limit: 3,
        }),
        [],
    );

    const handleToggleMaintenance = useCallback(() => {
        if (!property) return;
        if (property.status === 'active') {
            if (currentLease) {
                setBlockedOpen(true);
                return;
            }
            updateProperty.mutate(
                {id, data: {status: 'maintenance'}},
                {
                    onSuccess: () => notify.scenarios.property.movedToMaintenance(),
                    onError: showMutationError,
                },
            );
        } else {
            updateProperty.mutate(
                {id, data: {status: 'active'}},
                {
                    onSuccess: () => notify.scenarios.property.returnedToWork(),
                    onError: showMutationError,
                },
            );
        }
    }, [property, currentLease, id, updateProperty]);

    const handleToggleArchive = useCallback(() => {
        if (!property) return;
        if (property.status === 'archived') {
            unarchiveProperty.mutate(id, {
                onSuccess: () => notify.scenarios.property.returnedFromArchive(),
                onError: showMutationError,
            });
        } else {
            if (currentLease) {
                setBlockedOpen(true);
                return;
            }
            archiveProperty.mutate(id, {
                onSuccess: () => notify.scenarios.property.movedToArchive(),
                onError: showMutationError,
            });
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
        summaryQuery.isError;

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
                    }}
                    isLoading={
                        propertyQuery.isFetching ||
                        leasesQuery.isFetching ||
                        summaryQuery.isFetching
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
                        propertyId={id}
                        onPayRent={handlePayRent}
                        isPayRentLoading={isPayRentLoading}
                        onEndLease={handleEndLease}
                    />

                    <PropertyTenantCard lease={property.activeLease}/>

                    <PropertyOperationsSection
                        propertyId={id}
                        title="Просроченные операции"
                        emptyText="Просроченных операций нет"
                        filters={overdueFilters}
                    />

                    <PropertyOperationsSection
                        propertyId={id}
                        title="Запланированные операции"
                        emptyText="Запланированных операций нет"
                        filters={upcomingFilters}
                    />

                    <PropertyOperationsActions
                        propertyId={id}
                        isArchived={property.status === 'archived'}
                    />

                    <PropertyOperationsCard
                        propertyName={property.name}
                        summary={summaryQuery.data}
                    />

                    <PropertyInfoCard
                        description={property.description}
                        propertyId={id}
                        isArchived={property.status === 'archived'}
                    />
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

            {successBannerOpen && (
                <PropertySuccessBanner
                    onOpenLease={() => router.push(ROUTES.lease(selectedLeaseId))}
                />
            )}
        </div>
    );
}
