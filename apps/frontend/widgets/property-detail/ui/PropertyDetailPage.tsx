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
import {useRentals, currentRentalOf} from '@/features/rentals';
import {
  defaultOperationsPeriod,
  usePayments,
  usePropertyOperationsSummary,
  usePropertyOverdueOperations,
} from '@/features/payments';
import {useContacts} from '@/features/contacts';
import {useActiveTasks} from '@/features/tasks';
import {clientTodayIso} from '@/entities/payment';
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
import {propertyPaymentGroups} from '../lib/payments-strip';
import {operationsSectionTitle} from '../lib/operations-section';
import {propertyTasksSummary} from '../lib/tasks-summary';
import {propertyApartmentSummaryRows} from '../lib/apartment-summary';
import {TopNav, TopNavBackButton, TopNavTitle, IconButton, PageContent, ConfirmDialog, Skeleton} from '@/shared/ui/design';
import {StarOutline} from '@/shared/assets/icons';
import {PropertyMediaBlock} from './PropertyMediaBlock';
import {
  PropertyRentalBlock,
} from './PropertyRentalBlock';
import {PropertyPaymentsIconsBlock} from './PropertyPaymentsIconsBlock';
import {PropertyOperationsBlock} from './PropertyOperationsBlock';
import {PropertyContactsBlock} from './PropertyContactsBlock';
import {PropertyTasksBlock} from './PropertyTasksBlock';
import {PropertyApartmentBlock} from './PropertyApartmentBlock';
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

/** Плейсхолдер строки секции, пока данные секции едут (§7: скелетон
 * приглушён внутри серой карточки). */
function PropertySectionSkeleton(): JSX.Element {
    return (
        <div className="flex flex-col gap-3 px-6 pb-6 pt-4" aria-hidden>
            <Skeleton className="h-11 w-full bg-surface-muted-hover"/>
        </div>
    );
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
 * Занятость (Начать ↔ Завершить аренду) приходит из аренд объекта.
 *
 * Наполнение секций живыми данными (#589; Figma 1185:40820 — активная
 * аренда, 1581:53905 — срок подошёл к концу, 1193:48779 — без аренды
 * с платежами, 1193:49273 — нули): «Аренда» — прогресс платежей и
 * кнопки Продлить/Завершить; «Регулярные платежи» — группы иконок
 * категорий с точками просрочки; «Операции в <месяц>» — сводка месяца;
 * «Контакты» — арендатор + контакты объекта; «Задачи» — сводка
 * (макетом не покрыта — сверить на приёмке); «<Тип>» — характеристики.
 * Пустые состояния секций — по фактическим данным секции (#588 задавал
 * каркас), набор копирайта — по числу объектов.
 */
export function PropertyDetailPage(): JSX.Element {
    const params = useParams<{ id: string }>();
    const id = params.id;
    const router = useRouter();

    const propertyQuery = useProperty(id);

    // Количество активных объектов живёт только в списочном ответе (#584) —
    // читаем из общего кэша хаба/лендинга ради выбора набора пустых.
    const listQuery = usePropertiesWithMeta();
    // Текущая (незавершённая) аренда — источник блока «Аренда» (#589) и
    // арендатора в «Контактах»; точный источник — аренды самого объекта.
    const rentalsQuery = useRentals(id);
    const subscriptionQuery = useSubscription();

    // Данные секций (#589): правила платежей с точками просрочки, сводка
    // оплаченных операций за текущий месяц (+ all-time — выбор между
    // сводкой и «Операций еще не было», как на экране операций), контакты
    // объекта и активные задачи.
    const today = clientTodayIso();
    const paymentsQuery = usePayments(id);
    const overdueQuery = usePropertyOverdueOperations(id);
    const operationsPeriod = defaultOperationsPeriod(today);
    const operationsQuery = usePropertyOperationsSummary(id, {
        status: 'paid',
        order: 'desc',
        dateFrom: operationsPeriod.from,
        dateTo: operationsPeriod.to,
    });
    const operationsEverQuery = usePropertyOperationsSummary(id, {
        status: 'paid',
        order: 'desc',
    });
    const contactsQuery = useContacts(id);
    const tasksQuery = useActiveTasks(id);

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

    const currentRental = rentalsQuery.data
        ? currentRentalOf(rentalsQuery.data)
        : undefined;
    const hasRental = currentRental !== undefined;

    const payments = paymentsQuery.data ?? [];
    // Платежи с накопленной просрочкой — красная точка на иконке
    // (как на «Платежах объекта»: просрочки приходят порциями).
    const overduePaymentIds = new Set(
        overdueQuery.data?.flatMap((operation) =>
            operation.paymentId !== null ? [operation.paymentId] : [],
        ) ?? [],
    );
    const paymentGroups = propertyPaymentGroups(payments, overduePaymentIds, today);
    const contacts = contactsQuery.data ?? [];
    const tenant = currentRental?.tenant ?? null;
    // Арендатор уже показан отдельной строкой с бейджем роли — из книги
    // его исключаем, чтобы человек не дублировался (макет 1185:40820).
    const propertyContacts = tenant
        ? contacts.filter((contact) => contact.id !== tenant.contactId)
        : contacts;
    const tasksSummary = tasksQuery.data ? propertyTasksSummary(tasksQuery.data) : null;
    const apartmentRows = property
        ? propertyApartmentSummaryRows(property.type, property.attributes)
        : [];
    // «Операций еще не было» — только когда оплаченных операций нет
    // вовсе (all-time сводка), иначе блок месяца с нулями.
    const operationsNever =
        operationsEverQuery.isSuccess &&
        operationsEverQuery.data.incomeTotalKopecks === 0 &&
        operationsEverQuery.data.expenseTotalKopecks === 0;

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
    }, [archiveProperty, id, setArchiveOpen]);

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
    }, [id, router, setPin, unarchiveProperty, updateProperty, setStatusSheetOpen, setArchiveOpen, setDeleteOpen, setSharingOpen]);

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
    }, [deleteProperty, id, router, setDeleteOpen]);

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
                            {rentalsQuery.isPending ? (
                                <PropertySectionSkeleton/>
                            ) : currentRental !== undefined ? (
                                <PropertyRentalBlock
                                    rental={currentRental}
                                    onExtend={() => router.push(ROUTES.propertyRentalExtend(id))}
                                    onComplete={() => router.push(ROUTES.propertyRentalComplete(id))}
                                />
                            ) : (
                                <PropertySectionEmpty
                                    imageSrc={propertySectionImages.rental}
                                    copy={resolvePropertySectionEmpty('rental', emptySet, property.status)}
                                    onCta={sectionCTAs.rental}
                                    ctaDisabled={!canMutate}
                                />
                            )}
                        </PropertySectionCard>

                        <PropertySectionCard
                            title="Регулярные платежи"
                            href={ROUTES.propertyPayments(id)}
                        >
                            {paymentsQuery.isPending ? (
                                <PropertySectionSkeleton/>
                            ) : payments.length > 0 ? (
                                <PropertyPaymentsIconsBlock groups={paymentGroups}/>
                            ) : (
                                <PropertySectionEmpty
                                    imageSrc={propertySectionImages.payments}
                                    copy={resolvePropertySectionEmpty('payments', emptySet, property.status)}
                                    onCta={sectionCTAs.payments}
                                    ctaDisabled={!canMutate}
                                />
                            )}
                        </PropertySectionCard>

                        <PropertySectionCard
                            title={operationsSectionTitle(today)}
                            href={ROUTES.propertyOperations(id)}
                        >
                            {operationsNever ? (
                                <PropertySectionEmpty
                                    imageSrc={propertySectionImages.operations}
                                    copy={resolvePropertySectionEmpty('operations', emptySet, property.status)}
                                />
                            ) : (
                                <PropertyOperationsBlock
                                    summary={operationsQuery.data}
                                    isLoading={operationsQuery.isPending}
                                />
                            )}
                        </PropertySectionCard>

                        <PropertySectionCard
                            title="Контакты"
                            href={ROUTES.propertyContacts(id)}
                        >
                            {contactsQuery.isPending ? (
                                <PropertySectionSkeleton/>
                            ) : tenant !== null || propertyContacts.length > 0 ? (
                                <PropertyContactsBlock
                                    tenant={tenant}
                                    contacts={propertyContacts}
                                    onOpenContact={(contactId) =>
                                        router.push(ROUTES.propertyContact(id, contactId))
                                    }
                                />
                            ) : (
                                <PropertySectionEmpty
                                    imageSrc={propertySectionImages.contacts}
                                    copy={resolvePropertySectionEmpty('contacts', emptySet, property.status)}
                                    onCta={sectionCTAs.contacts}
                                    ctaDisabled={!canMutate}
                                />
                            )}
                        </PropertySectionCard>

                        <PropertySectionCard
                            title="Задачи"
                            href={ROUTES.propertyTasks(id)}
                        >
                            {tasksQuery.isPending ? (
                                <PropertySectionSkeleton/>
                            ) : tasksSummary !== null ? (
                                <PropertyTasksBlock
                                    summary={tasksSummary}
                                    onSelect={() => router.push(ROUTES.propertyTasks(id))}
                                />
                            ) : (
                                <PropertySectionEmpty
                                    imageSrc={propertySectionImages.tasks}
                                    copy={resolvePropertySectionEmpty('tasks', emptySet, property.status)}
                                    onCta={sectionCTAs.tasks}
                                    ctaDisabled={!canMutate}
                                />
                            )}
                        </PropertySectionCard>

                        <PropertySectionCard
                            title={propertyTypeLabels[property.type]}
                            href={ROUTES.propertyAbout(id)}
                        >
                            {apartmentRows.length > 0 ? (
                                <PropertyApartmentBlock rows={apartmentRows}/>
                            ) : (
                                <PropertySectionEmpty
                                    imageSrc={propertySectionImages.about}
                                    copy={resolvePropertySectionEmpty('about', emptySet, property.status)}
                                    onCta={sectionCTAs.about}
                                    ctaDisabled={!canMutate}
                                />
                            )}
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
