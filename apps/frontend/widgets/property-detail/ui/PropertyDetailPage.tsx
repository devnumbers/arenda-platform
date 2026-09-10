'use client';

import {useParams, useRouter} from 'next/navigation';
import type {JSX} from 'react';
import React, {useCallback} from 'react';
import {notify} from '@/shared/lib/notifications';
import {goBack} from '@/shared/lib/navigation';
import {ROUTES} from '@/shared/config/routes';
import {
    type DeletePropertyMode,
    useArchiveProperty,
    useDeleteProperty,
    useProperty,
    useUnarchiveProperty,
    useUpdateProperty
} from '@/features/properties';
import type {ApiError} from '@/shared/api/errors';
import {resolvePropertyDetailError} from '../lib/resolve-property-detail-error';
import {SubScreenShell} from '@/shared/ui/design';
import {PropertyGallery} from './PropertyGallery';
import {PropertyStatusSection} from './PropertyStatusSection';
import {PropertyInfoCard} from './PropertyInfoCard';
import {PropertyAttributesSection} from './PropertyAttributesSection';
import {PropertyPaymentsSection} from './PropertyPaymentsSection';
import {PropertyRentalsSection} from './PropertyRentalsSection';
import {PropertyContactsSection} from './PropertyContactsSection';
import {PropertyActionMenu} from './PropertyActionMenu';
import {PropertyArchiveModal} from './PropertyArchiveModal';
import {PropertyDeleteModal} from './PropertyDeleteModal';
import {PropertySharingModal} from './PropertySharingModal';
import {PropertySharedBanner} from './PropertySharedBanner';
import {PropertyDetailLoading} from './PropertyDetailLoading';
import {PropertyDetailError} from './PropertyDetailError';
import {PropertyNotFoundScreen} from './PropertyNotFoundScreen';
import {PropertySuspendedScreen} from './PropertySuspendedScreen';
import styles from './PropertyDetailPage.module.css';

function showMutationError(error: ApiError): void {
    notify.scenarios.property.saveError({description: error.detail});
}

export function PropertyDetailPage(): JSX.Element {
    const params = useParams<{ id: string }>();
    const id = params.id;
    const router = useRouter();

    const propertyQuery = useProperty(id);

    const updateProperty = useUpdateProperty();
    const archiveProperty = useArchiveProperty();
    const unarchiveProperty = useUnarchiveProperty();
    const deleteProperty = useDeleteProperty();

    const [archiveOpen, setArchiveOpen] = React.useState(false);
    const [deleteOpen, setDeleteOpen] = React.useState(false);
    const [sharingOpen, setSharingOpen] = React.useState(false);

    const property = propertyQuery.data;

    const handleToggleMaintenance = useCallback(() => {
        if (!property) return;
        if (property.status === 'active') {
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
    }, [property, id, updateProperty]);

    const handleToggleArchive = useCallback(() => {
        if (!property) return;
        if (property.status === 'archived') {
            unarchiveProperty.mutate(id, {
                onSuccess: () => notify.scenarios.property.returnedFromArchive(),
                onError: showMutationError,
            });
        } else {
            setArchiveOpen(true);
        }
    }, [property, id, unarchiveProperty]);

    const handleArchive = useCallback(() => {
        archiveProperty.mutate(id, {
            onSuccess: () => {
                setArchiveOpen(false);
                notify.scenarios.property.movedToArchive();
            },
            onError: showMutationError,
        });
    }, [archiveProperty, id]);

    const handleEdit = useCallback(() => {
        router.push(ROUTES.propertyEdit(id));
    }, [id, router]);

    const handleAccess = useCallback(() => {
        setSharingOpen(true);
    }, []);

    const handleDelete = useCallback((mode: DeletePropertyMode) => {
        deleteProperty.mutate(
            {id, mode},
            {
                onSuccess: () => {
                    setDeleteOpen(false);
                    notify.scenarios.property.deleted();
                    goBack(router, ROUTES.properties);
                },
                onError: (error) => {
                    notify.scenarios.property.deleteError({description: error.detail});
                },
            },
        );
    }, [deleteProperty, id, router]);

    // Разводим только ошибку основного запроса объекта: 404 (нет объекта
    // или нет доступа) и 403 membership_suspended (лимит тарифа) получают
    // свои экраны.
    const propertyErrorKind = propertyQuery.isError
        ? resolvePropertyDetailError(propertyQuery.error)
        : null;

    const isLoading = propertyQuery.isPending;

    return (
        <>
            {/* Единый хром подэкрана (карта #556): каркас SubScreenShell,
                меню действий — в слоте trailing, «Назад» — на список. */}
            <SubScreenShell
                title="Мой объект"
                fallbackHref={ROUTES.properties}
                trailing={
                    <PropertyActionMenu
                        status={property?.status}
                        disabled={isLoading || propertyQuery.isError || !property}
                        onEdit={handleEdit}
                        onAccess={handleAccess}
                        onToggleMaintenance={handleToggleMaintenance}
                        onToggleArchive={handleToggleArchive}
                        onDelete={() => setDeleteOpen(true)}
                    />
                }
            >
                <div className={styles.root}>
                    {property?.access && property.access.role !== 'owner' && (
                        <PropertySharedBanner access={property.access}/>
                    )}

                    {isLoading && <PropertyDetailLoading/>}

                    {!isLoading && propertyErrorKind === 'not_found' && (
                        <PropertyNotFoundScreen/>
                    )}

                    {!isLoading && propertyErrorKind === 'suspended' && (
                        <PropertySuspendedScreen/>
                    )}

                    {!isLoading && (propertyErrorKind === null || propertyErrorKind === 'generic') &&
                        (propertyQuery.isError || !property) && (
                        <PropertyDetailError
                            onRetry={() => {
                                void propertyQuery.refetch();
                            }}
                            isLoading={propertyQuery.isFetching}
                        />
                    )}

                    {!isLoading && !propertyQuery.isError && property && (
                        <>
                            <PropertyGallery/>

                            <PropertyStatusSection property={property}/>

                            <PropertyInfoCard
                                description={property.description}
                                propertyId={id}
                                isArchived={property.status === 'archived'}
                            />

                            <PropertyAttributesSection
                                type={property.type}
                                attributes={property.attributes}
                                propertyId={id}
                                isArchived={property.status === 'archived'}
                            />

                            <PropertyRentalsSection propertyId={id}/>

                            <PropertyPaymentsSection propertyId={id}/>

                            <PropertyContactsSection propertyId={id}/>
                        </>
                    )}
                </div>
            </SubScreenShell>

            <PropertyArchiveModal
                isOpen={archiveOpen}
                onClose={() => setArchiveOpen(false)}
                onArchive={handleArchive}
                membersCount={property?.members_count ?? 0}
                isArchiving={archiveProperty.isPending}
            />

            <PropertyDeleteModal
                isOpen={deleteOpen}
                onClose={() => setDeleteOpen(false)}
                onDelete={handleDelete}
                membersCount={property?.members_count ?? 0}
                deletingMode={
                    deleteProperty.isPending
                        ? deleteProperty.variables.mode
                        : null
                }
            />

            <PropertySharingModal
                propertyId={id}
                isOpen={sharingOpen}
                onClose={() => setSharingOpen(false)}
                isArchived={property?.status === 'archived'}
            />
        </>
    );
}
