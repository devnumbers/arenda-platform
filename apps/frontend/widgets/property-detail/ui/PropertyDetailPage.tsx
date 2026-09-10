'use client';

import {useParams, useRouter} from 'next/navigation';
import type {JSX} from 'react';
import React, {useCallback} from 'react';
import {notify} from '@/shared/lib/notifications';
import {goBack} from '@/shared/lib/navigation';
import {ROUTES} from '@/shared/config/routes';
import {
  canMutateProperty,
  propertyTypeLabels,
  useArchiveProperty,
  useDeleteProperty,
  usePropertiesWithMeta,
  useProperty,
  useSetPropertyPin,
  useUnarchiveProperty,
  useUpdateProperty,
  type DeletePropertyMode,
} from '@/features/properties';
import {useRentals} from '@/features/rentals';
import {useSubscription} from '@/features/subscription';
import {isPaidTariff} from '@/entities/user';
import type {ApiError} from '@/shared/api/errors';
import {resolvePropertyDetailError} from '../lib/resolve-property-detail-error';
import {
  buildPropertyManageActions,
  buildPropertyStatusSheetItems,
  propertyStatusSubtitle,
  type PropertyDetailActionKey,
} from '../lib/property-detail-status';
import {
  propertySectionImages,
  resolvePropertyDetailEmptySet,
  resolvePropertySectionEmpty,
  type PropertyDetailSectionKey,
} from '../lib/property-sections';
import {TopNav, TopNavBackButton, TopNavTitle, IconButton, PageContent, ConfirmDialog} from '@/shared/ui/design';
import {StarOutline} from '@/shared/assets/icons';
import {PropertyMediaBlock} from './PropertyMediaBlock';
import {
  PropertySectionCard,
  PropertySectionEmpty,
} from './PropertySectionCard';
import {
  PropertyManageSection,
  PropertyStatusSheet,
} from './PropertyDetailActions';
import {PropertyDetailKebab} from './PropertyDetailKebab';
import {PropertyDeleteModal} from './PropertyDeleteModal';
import {PropertySharingModal} from './PropertySharingModal';
import {PropertySharedBanner} from './PropertySharedBanner';
import {PropertyDetailLoading} from './PropertyDetailLoading';
import {PropertyDetailError} from './PropertyDetailError';
import {PropertyNotFoundScreen} from './PropertyNotFoundScreen';
import {PropertySuspendedScreen} from './PropertySuspendedScreen';

function showMutationError(error: ApiError): void {
    notify.scenarios.property.saveError({description: error.detail});
}

/**
 * Детализация объекта — новый каркас карты #583 (тикет #588; Figma
 * 1554:98469 — приветственные пустые, 1554:100751 — обычные, 1186:44996 —
 * ПК, 1186:44992 — планшет): шапка «Объект» со звездой базового тарифа
 * или «Назад», кебаб справа (1186:44996); медиа-блок (плейсхолдер-круг —
 * рендер фото решается на приёмке); секции-карточки с пустыми
 * состояниями — наполнение в #589; «Управление» — контекстные действия;
 * шиты смены статуса и подтверждение архивации (1581:55389); тосты
 * результатов (1581:55564 / 1581:54666 / 1581:55041). «Объект» = кебаб
 * «Изменить статус» — секция «Управление» дублирует те же действия.
 * Занятость (Начать ↔ Завершить аренду) приходит из списочного кэша
 * (occupancy — резолюция #584, на детали её нет).
 */
export function PropertyDetailPage(): JSX.Element {
    const params = useParams<{ id: string }>();
    const id = params.id;
    const router = useRouter();

    const propertyQuery = useProperty(id);

    // Количество активных объектов живёт только в списочном ответе (#584) —
    // читаем из общего кэша хаба/лендинга ради выбора набора пустых.
    const listQuery = usePropertiesWithMeta();
    // Незавершённая аренда для свитча «Начать ↔ Завершить» — из аренд
    // самого объекта: точный источник, не зависящий от кэша списка.
    const rentalsQuery = useRentals(id);
    const subscriptionQuery = useSubscription();

    const updateProperty = useUpdateProperty();
    const archiveProperty = useArchiveProperty();
    const unarchiveProperty = useUnarchiveProperty();
    const setPin = useSetPropertyPin();
    const deleteProperty = useDeleteProperty();

    const [statusSheetOpen, setStatusSheetOpen] = React.useState(false);
    const [archiveOpen, setArchiveOpen] = React.useState(false);
    const [deleteOpen, setDeleteOpen] = React.useState(false);
    const [sharingOpen, setSharingOpen] = React.useState(false);

    const property = propertyQuery.data;

    const hasRental =
        rentalsQuery.data?.some((rental) => rental.status !== 'completed') ?? false;
    // Пока список не загружен, считаем объект единственным — приветственный
    // набор (первый объект); правило переключения наборов — на приёмке.
    const emptySet = resolvePropertyDetailEmptySet(listQuery.data?.items.length ?? 1);

    const canMutate = canMutateProperty(propertyQuery.isSuccess ? property : undefined);
    // Для «Управления» роль важна без статуса: архивный владелец видит
    // «Вернуть из архива» и удаление (canMutateProperty гасит и архив —
    // его смысл для CTA-кнопок секций, не для этого списка).
    const roleCanMutate =
        property?.access !== undefined && property.access.role !== 'viewer';
    // «Основной объект» — платная возможность: базовому тарифу в шапке
    // звезда апселла, строк пина в «Управлении» нет.
    const isPaid = subscriptionQuery.data
        ? isPaidTariff(subscriptionQuery.data.tariff.name)
        : true;
    const isPinned = property?.pinned_at != null;

    const manageActions = property
        ? buildPropertyManageActions({
            status: property.status,
            hasRental,
            isPinned,
            canMutate: roleCanMutate,
            canPin: isPaid && roleCanMutate,
        })
        : [];
    const statusSheetItems = property
        ? buildPropertyStatusSheetItems(property.status, hasRental)
        : [];

    const handleArchive = useCallback(() => {
        archiveProperty.mutate(id, {
            onSuccess: () => {
                setArchiveOpen(false);
                notify.scenarios.property.movedToArchive();
            },
            onError: showMutationError,
        });
    }, [archiveProperty, id]);

    const handleAction = useCallback((key: PropertyDetailActionKey) => {
        switch (key) {
            case 'about':
                router.push(ROUTES.propertyAbout(id));
                break;
            case 'change-status':
                setStatusSheetOpen(true);
                break;
            case 'edit':
                router.push(ROUTES.propertyEdit(id));
                break;
            case 'pin':
            case 'unpin':
                setPin.mutate(
                    {id, pinned: key === 'pin'},
                    {onError: showMutationError},
                );
                break;
            case 'start-rental':
                router.push(ROUTES.propertyRentalNew(id));
                break;
            case 'complete-rental':
                router.push(ROUTES.propertyRentalComplete(id));
                break;
            case 'start-maintenance':
                updateProperty.mutate(
                    {id, data: {status: 'maintenance'}},
                    {
                        onSuccess: () => notify.scenarios.property.movedToMaintenance(),
                        onError: showMutationError,
                    },
                );
                break;
            case 'finish-maintenance':
                updateProperty.mutate(
                    {id, data: {status: 'active'}},
                    {
                        onSuccess: () => notify.scenarios.property.maintenanceFinished(),
                        onError: showMutationError,
                    },
                );
                break;
            case 'archive':
                setArchiveOpen(true);
                break;
            case 'unarchive':
                unarchiveProperty.mutate(id, {
                    onSuccess: () => notify.scenarios.property.returnedFromArchive(),
                    onError: showMutationError,
                });
                break;
            case 'access':
                setSharingOpen(true);
                break;
            case 'delete':
                setDeleteOpen(true);
                break;
        }
    }, [id, router, setPin, unarchiveProperty, updateProperty]);

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
    const status = property?.status;

    const sectionCTAs: Record<PropertyDetailSectionKey, (() => void) | undefined> = {
        rental: () => router.push(ROUTES.propertyRentalNew(id)),
        payments: () => router.push(ROUTES.propertyPaymentNew(id, 'payment')),
        operations: undefined,
        contacts: () => router.push(ROUTES.propertyContactNew(id)),
        tasks: () => router.push(ROUTES.propertyTaskCreate(id)),
        about: () => router.push(ROUTES.propertyEdit(id)),
    };

    return (
        <>
            {/* Особая анатомия шапки (DESIGN.md §2): слот ведущей кнопки
             * зависит от тарифа — звезда апселла у базового (тап → смена
             * тарифа), «Назад» у платных, включая режим «деталь = лендинг
             * таба» (решение владельца 10.09). Кебаб — в trailing. */}
            <TopNav
                leading={
                    isPaid ? (
                        <TopNavBackButton fallbackHref={ROUTES.properties}/>
                    ) : (
                        <IconButton
                            icon={<StarOutline className="h-6 w-6"/>}
                            label="Сменить тариф"
                            onClick={() => router.push(ROUTES.profileTariffChange)}
                        />
                    )
                }
                trailing={
                    status !== undefined && propertyErrorKind === null ? (
                        <PropertyDetailKebab
                            status={status}
                            canMutate={roleCanMutate}
                            onAction={handleAction}
                        />
                    ) : undefined
                }
            >
                <TopNavTitle
                    title="Объект"
                    subtitle={property ? propertyStatusSubtitle(property.status) : undefined}
                />
            </TopNav>

            <PageContent className="px-6">
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
                        <PropertyMediaBlock name={property.name} address={property.address}/>

                        <PropertySectionCard
                            title="Аренда"
                            href={ROUTES.propertyRental(id)}
                            className="mt-20"
                        >
                            <PropertySectionEmpty
                                imageSrc={propertySectionImages.rental}
                                copy={resolvePropertySectionEmpty('rental', emptySet, property.status)}
                                onCta={sectionCTAs.rental}
                                ctaDisabled={!canMutate}
                            />
                        </PropertySectionCard>

                        <PropertySectionCard
                            title="Регулярные платежи"
                            href={ROUTES.propertyPayments(id)}
                        >
                            <PropertySectionEmpty
                                imageSrc={propertySectionImages.payments}
                                copy={resolvePropertySectionEmpty('payments', emptySet, property.status)}
                                onCta={sectionCTAs.payments}
                                ctaDisabled={!canMutate}
                            />
                        </PropertySectionCard>

                        <PropertySectionCard
                            title="Операции"
                            href={ROUTES.propertyOperations(id)}
                        >
                            <PropertySectionEmpty
                                imageSrc={propertySectionImages.operations}
                                copy={resolvePropertySectionEmpty('operations', emptySet, property.status)}
                            />
                        </PropertySectionCard>

                        <PropertySectionCard
                            title="Контакты"
                            href={ROUTES.propertyContacts(id)}
                        >
                            <PropertySectionEmpty
                                imageSrc={propertySectionImages.contacts}
                                copy={resolvePropertySectionEmpty('contacts', emptySet, property.status)}
                                onCta={sectionCTAs.contacts}
                                ctaDisabled={!canMutate}
                            />
                        </PropertySectionCard>

                        <PropertySectionCard
                            title="Задачи"
                            href={ROUTES.propertyTasks(id)}
                        >
                            <PropertySectionEmpty
                                imageSrc={propertySectionImages.tasks}
                                copy={resolvePropertySectionEmpty('tasks', emptySet, property.status)}
                                onCta={sectionCTAs.tasks}
                                ctaDisabled={!canMutate}
                            />
                        </PropertySectionCard>

                        <PropertySectionCard
                            title={propertyTypeLabels[property.type]}
                            href={ROUTES.propertyAbout(id)}
                        >
                            <PropertySectionEmpty
                                imageSrc={propertySectionImages.about}
                                copy={resolvePropertySectionEmpty('about', emptySet, property.status)}
                                onCta={sectionCTAs.about}
                                ctaDisabled={!canMutate}
                            />
                        </PropertySectionCard>

                        <PropertySectionCard title="Управление">
                            <PropertyManageSection
                                items={manageActions}
                                onAction={handleAction}
                            />
                        </PropertySectionCard>
                    </>
                )}
            </PageContent>

            <PropertyStatusSheet
                open={statusSheetOpen}
                onOpenChange={setStatusSheetOpen}
                items={statusSheetItems}
                onAction={handleAction}
            />

            <ConfirmDialog
                open={archiveOpen}
                onOpenChange={setArchiveOpen}
                title="Перевести объект в архив?"
                description="Объектом нельзя будет управлять, он будет доступен только для просмотра. Все данные будут сохранены. Вы сможете вернуть объект в работу в любой момент"
                confirmLabel="Архивировать"
                pending={archiveProperty.isPending}
                onConfirm={handleArchive}
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
