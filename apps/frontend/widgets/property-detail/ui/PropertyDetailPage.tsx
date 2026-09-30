'use client';

import {useParams, useRouter} from 'next/navigation';
import type {JSX} from 'react';
import React from 'react';
import {notify} from '@/shared/lib/notifications';
import {goBack} from '@/shared/lib/navigation';
import {ROUTES} from '@/shared/config/routes';
import { OPERATIONS_FEED_SORT } from '@/shared/api/query-keys';
import {
  propertyTypeLabels,
  useArchiveProperty,
  useDeleteProperty,
  usePropertiesWithMeta,
  useProperty,
  useSetPropertyPin,
  useUnarchiveProperty,
  useUpdateProperty,
} from '@/features/properties';
import { propertyPermissions } from '@/entities/property';
import { useLeaveProperty } from '@/features/participants';
import {useRentals, currentRentalOf, useCompleteRental} from '@/features/rentals';
import {
  operationsMonthOf,
  operationsMonthRange,
  overduePaymentIdsOf,
  usePayments,
  usePropertyOperationsSummary,
  usePropertyOverdueOperations,
} from '@/features/payments';
import {useContacts} from '@/features/contacts';
import {
  useActiveTasks,
  useCompleteTask,
  useUncompleteTask,
} from '@/features/tasks';
import {dateToIsoLocal} from '@/shared/lib/calendar';
import {useSubscription} from '@/features/subscription';
import {isPaidTariff} from '@/entities/user';
import type {ApiError} from '@/shared/api/errors';
import {resolvePropertyDetailError} from '../lib/resolve-property-detail-error';
import {
  buildPropertyManageActions,
  buildPropertyStatusSheetItems,
  deleteBlockedByRental,
  guardedStatusAction,
  propertyStatusSubtitle,
  type GuardedStatusAction,
  type PropertyDetailActionKey,
} from '../lib/property-detail-status';
import {
  propertySectionImages,
  resolvePropertyDetailEmptySet,
  resolvePropertySectionEmpty,
  type PropertyDetailSectionKey,
} from '../lib/property-sections';
import {propertyPaymentGroups} from '../lib/payments-strip';
import {propertySectionCta} from '../lib/property-section-cta';
import {operationsSectionTitle} from '../lib/operations-section';
import {propertyDetailTasks} from '../lib/detail-tasks';
import {propertyApartmentSummaryRows} from '../lib/apartment-summary';
import {TopNav, TopNavBackButton, TopNavTitle, IconButton, PageContent, ConfirmDialog} from '@/shared/ui/design';
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
import {PropertyAccessPill} from './PropertyAccessPill';
import {PropertyArchivedPill} from './PropertyArchivedPill';
import {propertyHeaderPills} from '../lib/property-header-pills';
import {PropertyOwnerSection} from './PropertyOwnerSection';
import {PropertyDetailLoading, PropertySectionEmptySkeleton, PropertyPaymentsStripSkeleton} from './PropertyDetailLoading';
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
 * Занятость (Начать ↔ Завершить аренду) приходит из аренд объекта.
 *
 * Быстрое завершение аренды (#627; Figma 1583:56380): «Завершить аренду» —
 * из шита статуса, «Управления» и кнопки блока «Аренда» — открывает
 * канон-подтверждение «Завершить аренду?»; подтверждение завершает аренду
 * сегодняшней датой без возврата залога, тост результата, деталь
 * обновляется (секция «Аренда» — обычное пустое). Полный мастер с датой
 * и залогом остаётся на детализации аренды (#534).
 *
 * Guard смены статуса (#628; Figma 1583:55882): у арендованного объекта
 * «Объект на ремонте» и «Перевести в архив» открывают шит «Нельзя изменить
 * статус, пока объект арендован». «Завершить» — составное действие
 * (решение владельца 12.09, против двухшаговой аннотации макета):
 * завершает аренду (#627, сегодняшней датой) и тут же применяет
 * выбранный статус, один объединённый тост; отказ смены статуса после
 * завершения — тост ошибки, аренда остаётся завершённой. Без аренды —
 * прежнее поведение (мутация / архивный конфирм).
 *
 * Шит перед пином (#630; Figma 1583:57452): «Сделать основным» из
 * «Управления» не мутирует сразу — открывает шит-объяснение
 * «Этот объект будет открываться первым…» с кнопками «Сделать объект
 * основным» / «Понятно»; «Убрать из основных» остаётся прямым действием
 * (шита в макетах нет). Только платный тариф — у базового строк пина
 * нет (canPin), звезда-апселл не тронута.
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
    // Гейт #769: property-scoped запросы секций стартуют только после
    // успеха детали — на упавшей детали (404 / 403 membership_suspended)
    // гард deep-link показывается сразу, и зависимые запросы не
    // простреливают 404-волнами по нечитаемому объекту.
    const propertyLoaded = propertyQuery.isSuccess;
    // Текущая (незавершённая) аренда — источник блока «Аренда» (#589) и
    // арендатора в «Контактах»; точный источник — аренды самого объекта.
    const rentalsQuery = useRentals(id, { enabled: propertyLoaded });
    const subscriptionQuery = useSubscription();

    // Данные секций (#589): правила платежей с точками просрочки, сводка
    // оплаченных операций за текущий месяц (+ all-time — выбор между
    // сводкой и «Операций еще не было», как на экране операций), контакты
    // объекта и активные задачи.
    const today = dateToIsoLocal(new Date());
    const paymentsQuery = usePayments(id, '', { enabled: propertyLoaded });
    const overdueQuery = usePropertyOverdueOperations(id, '', { enabled: propertyLoaded });
    // Сводка секции «Операции в <месяц>» — за текущий календарный месяц
    // (карта #669: сводка объекта месячная и после дефолта «весь период»
    // на лентах); границы берутся напрямую из модели месяца. Месяц —
    // по факту оплаты, в согласии с лентой операций (решение #933/#994).
    const operationsPeriod = operationsMonthRange(operationsMonthOf(today));
    const operationsQuery = usePropertyOperationsSummary(id, {
        status: 'paid',
        order: 'desc',
        sort: OPERATIONS_FEED_SORT,
        dateFrom: operationsPeriod.from,
        dateTo: operationsPeriod.to,
    }, { enabled: propertyLoaded });
    const operationsEverQuery = usePropertyOperationsSummary(id, {
        status: 'paid',
        order: 'desc',
        sort: OPERATIONS_FEED_SORT,
    }, { enabled: propertyLoaded });
    const contactsQuery = useContacts(id, '', { enabled: propertyLoaded });
    const tasksQuery = useActiveTasks(id, { enabled: propertyLoaded });
    const completeTask = useCompleteTask(id);
    const uncompleteTask = useUncompleteTask(id);

    const updateProperty = useUpdateProperty();
    const archiveProperty = useArchiveProperty();
    const unarchiveProperty = useUnarchiveProperty();
    const setPin = useSetPropertyPin();
    const deleteProperty = useDeleteProperty();
    const leaveProperty = useLeaveProperty();

    const [statusSheetOpen, setStatusSheetOpen] = React.useState(false);
    // Guard #628: какое статусное действие запросено из-под гарда; null —
    // шит закрыт. Само действие (ремонт/архив) применяется в
    // handleGuardConfirm после завершения аренды.
    const [guardedAction, setGuardedAction] = React.useState<GuardedStatusAction | null>(null);
    // Guard удаления #632: тап «Удалить объект» у арендованного не открывает
    // шит удаления — сперва «Завершить аренду».
    const [deleteGuardOpen, setDeleteGuardOpen] = React.useState(false);
    const [completeSheetOpen, setCompleteSheetOpen] = React.useState(false);
    // Шит пина #630: подтверждение перед «Сделать основным».
    const [pinSheetOpen, setPinSheetOpen] = React.useState(false);
    const [archiveOpen, setArchiveOpen] = React.useState(false);
    const [deleteOpen, setDeleteOpen] = React.useState(false);
    // Шит выхода участника (#703, макет 2235-100370): «Управление» →
    // «Покинуть объект» — канон-подтверждение #701 и возврат к объектам.
    const [leaveOpen, setLeaveOpen] = React.useState(false);

    const property = propertyQuery.data;
    // Пилюли шапки (#773): «В архиве» у архивного объекта (признак для
    // любого читателя — чужой архивный живёт только deep-link'ом) и роль
    // доступа у чужого.
    const headerPills = propertyHeaderPills(property);

    const currentRental = rentalsQuery.data
        ? currentRentalOf(rentalsQuery.data)
        : undefined;
    const hasRental = currentRental !== undefined;
    // Один сентинел и для хука, и для гарда мутации (#627): id пуст до
    // загрузки аренд — сам хук безобиден, вызов мутации гардится в
    // handleCompleteRental.
    const currentRentalId = currentRental?.id ?? null;
    const completeRental = useCompleteRental(id, currentRentalId ?? '');

    const payments = paymentsQuery.data ?? [];
    // Платежи с накопленной просрочкой — красная точка на иконке
    // (как на «Платежах объекта»: просрочки приходят порциями).
    const overduePaymentIds = overduePaymentIdsOf(overdueQuery.data ?? []);
    const paymentGroups = propertyPaymentGroups(payments, overduePaymentIds, today);
    const contacts = contactsQuery.data ?? [];
    const tenant = currentRental?.tenant ?? null;
    // Арендатор уже показан отдельной строкой с бейджем роли — из книги
    // его исключаем, чтобы человек не дублировался (макет 1185:40820);
    // максимум 3 строки блока: арендатор + 2 контакта (решение владельца
    // 11.09), без арендатора — 3 контакта.
    const propertyContacts = (
        tenant ? contacts.filter((contact) => contact.id !== tenant.contactId) : contacts
    ).slice(0, tenant !== null ? 2 : 3);
    const detailTasks = tasksQuery.data ? propertyDetailTasks(tasksQuery.data) : [];
    const tasksToday = tasksQuery.data?.today;
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

    // Центральные права объекта (#703): «Управление»/кебаб — ролевой
    // canManageMembers (билдеры ветвят архив сами: архивный владелец
    // видит «Вернуть из архива» и удаление), «Покинуть объект» — canLeave.
    // Create-CTA пустых секций — шов propertySectionCta (#774): зритель
    // не видит их вовсе, архив глушит (#773).
    const permissions = propertyPermissions(
        propertyQuery.isSuccess ? property : undefined,
    );
    const sectionCta = propertySectionCta(permissions);
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
            canMutate: permissions.canManageMembers,
            canPin: isPaid && permissions.canManageMembers,
            canLeave: permissions.canLeave,
            canLifecycle: permissions.canLifecycle,
        })
        : [];
    const statusSheetItems = property
        ? buildPropertyStatusSheetItems(property.status, hasRental, permissions.canLifecycle)
        : [];

    const handleArchive = () => {
        archiveProperty.mutate(id, {
            onSuccess: () => {
                setArchiveOpen(false);
                notify.scenarios.property.movedToArchive();
            },
            onError: showMutationError,
        });
    };

    // Быстрое завершение (#627): сегодняшней датой, без записи о возврате
    // залога (полный мастер с датой и залогом — на детализации аренды,
    // #534). Повторное завершение — 409: тост ошибки, шит остаётся
    // открытым (кнопка выходит из loading — можно повторить или отменить).
    const handleCompleteRental = () => {
        if (currentRentalId === null) {
            return;
        }
        completeRental.mutate(
            {completedDate: dateToIsoLocal(new Date())},
            {
                onSuccess: () => {
                    setCompleteSheetOpen(false);
                    notify.scenarios.rentals.completed();
                },
                onError: (error) => notify.scenarios.rentals.completeError(error),
            },
        );
    };

    // Guard #628 (решение владельца 12.09): «Завершить» — одно составное
    // действие: завершает аренду сегодняшней датой (как #627) и тут же
    // применяет выбранный под гардом статус (ремонт или архив). Кнопка
    // в loading на всю цепочку. Отказ смены статуса после успешного
    // завершения — тост ошибки, аренда остаётся завершённой, повтор —
    // ручной (пункт уже без гарда); отказ завершения — шит остаётся
    // открытым, как в #627.
    const handleGuardConfirm = () => {
        if (guardedAction === null) {
            return;
        }
        const action = guardedAction;
        const finishStatus = {
            onSuccess: () => {
                setGuardedAction(null);
                if (action === 'archive') {
                    notify.scenarios.property.archivedAfterRental();
                } else {
                    notify.scenarios.property.maintenanceAfterRental();
                }
            },
            onError: (error: ApiError) => {
                setGuardedAction(null);
                showMutationError(error);
            },
        };
        completeRental.mutate(
            {completedDate: dateToIsoLocal(new Date())},
            {
                onSuccess: () => {
                    if (action === 'archive') {
                        archiveProperty.mutate(id, finishStatus);
                    } else {
                        updateProperty.mutate(
                            {id, data: {status: 'maintenance'}},
                            finishStatus,
                        );
                    }
                },
                onError: (error) => notify.scenarios.rentals.completeError(error),
            },
        );
    };

    // Подтверждение шита пина (#630): исполняет «Сделать основным».
    // Успех закрывает шит (первенство видно в списке, строка «Управления»
    // переключится на «Убрать из основных»); отказ — тост ошибки, шит
    // остаётся открытым, как в #627/#628.
    const handlePinConfirm = () => {
        setPin.mutate(
            {id, pinned: true},
            {
                onSuccess: () => setPinSheetOpen(false),
                onError: showMutationError,
            },
        );
    };

    const handleAction = (key: PropertyDetailActionKey) => {
        const guarded = guardedStatusAction(key, hasRental);
        if (guarded !== null) {
            setGuardedAction(guarded);
            return;
        }
        // Guard удаления #632: у арендованного сперва «Завершить аренду»
        // (#627), шит удаления не открывается.
        if (deleteBlockedByRental(key, hasRental)) {
            setDeleteGuardOpen(true);
            return;
        }
        // Пин #630 (Figma 1583:57452) исполняется только после
        // шита-подтверждения; unpin — прямое действие (шита в макетах нет).
        if (key === 'pin') {
            setPinSheetOpen(true);
            return;
        }
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
            case 'unpin':
                setPin.mutate(
                    {id, pinned: false},
                    {onError: showMutationError},
                );
                break;
            case 'start-rental':
                router.push(ROUTES.propertyRentalNew(id));
                break;
            case 'complete-rental':
                setCompleteSheetOpen(true);
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
                router.push(ROUTES.propertyParticipants(id));
                break;
            case 'leave':
                setLeaveOpen(true);
                break;
            case 'delete':
                setDeleteOpen(true);
                break;
        }
    };

    // Выход участника (#703): подтверждение — канон #701 (2010-132970),
    // успех — тост и возврат к списку объектов (попап #701 живёт в срезе
    // участников, чужой слайс-виджет сюда не импортируется).
    const handleLeave = () => {
        leaveProperty.mutate(id, {
            onSuccess: () => {
                setLeaveOpen(false);
                notify.scenarios.access.leftProperty();
                goBack(router, ROUTES.properties);
            },
            onError: (error) => notify.scenarios.access.leavePropertyError(error),
        });
    };

    const handleDelete = () => {
        deleteProperty.mutate(
            {id},
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
    };

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
                            canMutate={permissions.canManageMembers}
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
                        <PropertyMediaBlock name={property.name} address={property.address}>
                            {/* Пилюли шапки (#773): «В архиве» и/или роль
                             * доступа — ряд под адресом, канон 2200-97365
                             * и 1603-92103. */}
                            {headerPills.length > 0 && (
                                <div className="mt-3 flex flex-wrap items-center justify-center gap-2">
                                    {headerPills.map((pill) =>
                                        pill.kind === 'archived' ? (
                                            <PropertyArchivedPill key="archived"/>
                                        ) : (
                                            <PropertyAccessPill key="access" role={pill.role} refreshing={propertyQuery.isFetching}/>
                                        ),
                                    )}
                                </div>
                            )}
                        </PropertyMediaBlock>

                        <PropertySectionCard
                            title="Аренда"
                            href={ROUTES.propertyRental(id)}
                            className="mt-20"
                        >
                            {rentalsQuery.isPending ? (
                                <PropertySectionEmptySkeleton/>
                            ) : currentRental !== undefined ? (
                                <PropertyRentalBlock
                                    rental={currentRental}
                                    onExtend={() => router.push(ROUTES.propertyRentalExtend(id))}
                                    // Смотрящему — старый путь в мастер с его
                                    // честным отказом (writeGate ADR 0053 §3):
                                    // шит с последующим 403 смотрителю не даёт
                                    // ничего (решение ревью #627).
                                    onComplete={
                                        permissions.canManageMembers
                                            ? () => setCompleteSheetOpen(true)
                                            : () => router.push(ROUTES.propertyRentalComplete(id))
                                    }
                                />
                            ) : (
                                <PropertySectionEmpty
                                    imageSrc={propertySectionImages.rental}
                                    copy={resolvePropertySectionEmpty('rental', emptySet, property.status)}
                                    onCta={sectionCta.visible ? sectionCTAs.rental : undefined}
                                    ctaDisabled={sectionCta.disabled}
                                />
                            )}
                        </PropertySectionCard>

                        <PropertySectionCard
                            title="Регулярные платежи"
                            href={ROUTES.propertyPayments(id)}
                        >
                            {paymentsQuery.isPending ? (
                                <PropertyPaymentsStripSkeleton/>
                            ) : payments.length > 0 ? (
                                <PropertyPaymentsIconsBlock groups={paymentGroups}/>
                            ) : (
                                <PropertySectionEmpty
                                    imageSrc={propertySectionImages.payments}
                                    copy={resolvePropertySectionEmpty('payments', emptySet, property.status)}
                                    onCta={sectionCta.visible ? sectionCTAs.payments : undefined}
                                    ctaDisabled={sectionCta.disabled}
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
                                <PropertySectionEmptySkeleton/>
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
                                    onCta={sectionCta.visible ? sectionCTAs.contacts : undefined}
                                    ctaDisabled={sectionCta.disabled}
                                />
                            )}
                        </PropertySectionCard>

                        <PropertySectionCard
                            title="Задачи"
                            href={ROUTES.propertyTasks(id)}
                        >
                            {tasksQuery.isPending ? (
                                <PropertySectionEmptySkeleton/>
                            ) : detailTasks.length > 0 ? (
                                <PropertyTasksBlock
                                    tasks={detailTasks}
                                    today={tasksToday ?? today}
                                    canMutate={permissions.canEdit}
                                    togglingFor={(task) =>
                                        (completeTask.isPending || uncompleteTask.isPending)
                                        && (completeTask.variables === task.id
                                            || uncompleteTask.variables === task.id)
                                    }
                                    onToggle={(task) => {
                                        if (task.status === 'completed') {
                                            uncompleteTask.mutate(task.id, {
                                                onError: (error) =>
                                                    notify.scenarios.tasks.uncompleteError(error),
                                            });
                                        } else {
                                            completeTask.mutate(task.id, {
                                                onError: (error) =>
                                                    notify.scenarios.tasks.completeError(error),
                                            });
                                        }
                                    }}
                                    onOpenFor={(task) => {
                                        if (!permissions.canEdit || task.status === 'completed'
                                            || task.ruleId === null) {
                                            return undefined;
                                        }
                                        const ruleId = task.ruleId;
                                        return () =>
                                            router.push(ROUTES.propertyTaskEdit(id, ruleId));
                                    }}
                                />
                            ) : (
                                <PropertySectionEmpty
                                    imageSrc={propertySectionImages.tasks}
                                    copy={resolvePropertySectionEmpty('tasks', emptySet, property.status)}
                                    onCta={sectionCta.visible ? sectionCTAs.tasks : undefined}
                                    ctaDisabled={sectionCta.disabled}
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
                                    onCta={sectionCta.visible ? sectionCTAs.about : undefined}
                                    ctaDisabled={sectionCta.disabled}
                                />
                            )}
                        </PropertySectionCard>

                        {permissions.canLeave && property.access?.ownerName !== undefined && (
                            <PropertyOwnerSection
                                ownerName={property.access.ownerName}
                                ownerEmail={property.access.ownerEmail}
                            />
                        )}

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

            {/* Выход участника (#703): канон #701 (2010-132970) — кнопки
             * в ряд, красное «Покинуть» справа; кнопка в loading на время
             * мутации. */}
            <ConfirmDialog
                open={leaveOpen}
                onOpenChange={setLeaveOpen}
                title="Уверены, что хотите покинуть объект?"
                titleClassName="text-[28px] leading-8"
                description="Вы потеряете доступ к объекту пользователя. Попросить доступ можно будет снова"
                descriptionClassName="text-base"
                confirmLabel="Покинуть"
                cancelLabel="Отмена"
                confirmVariant="danger"
                pending={leaveProperty.isPending}
                onConfirm={handleLeave}
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

            {/* Guard-шит (#628, Figma 1583:55882): сменить статус
             * арендованного нельзя — подпись макета R/400 16/18, кнопки
             * «Отменить»/«Завершить». «Завершить» — составное действие
             * (решение владельца 12.09): завершает аренду (#627) и тут же
             * применяет статус; pending на обе мутации, закрытие глушится. */}
            <ConfirmDialog
                open={guardedAction !== null}
                onOpenChange={(open) => {
                    if (!open) {
                        setGuardedAction(null);
                    }
                }}
                title="Нельзя изменить статус, пока объект арендован"
                description="Завершите аренду, чтобы изменить статус объекта"
                descriptionClassName="text-base leading-[18px]"
                confirmLabel="Завершить"
                cancelLabel="Отменить"
                pending={completeRental.isPending || updateProperty.isPending || archiveProperty.isPending}
                onConfirm={handleGuardConfirm}
            />

            {/* Guard удаления (#632): «Удалить объект» у арендованного
             * не доходит до шита удаления — сперва «Завершить аренду».
             * Составного действия нет (удаление необратимо): «Завершить
             * аренду» лишь открывает шит завершения #627, его подтверждение
             * остаётся за владельцем. Тексты — по образцу гарда статуса. */}
            <ConfirmDialog
                open={deleteGuardOpen}
                onOpenChange={setDeleteGuardOpen}
                title="Нельзя удалить объект, пока он арендован"
                description="Завершите аренду, чтобы удалить объект"
                descriptionClassName="text-base leading-[18px]"
                confirmLabel="Завершить аренду"
                cancelLabel="Отменить"
                onConfirm={() => {
                    setDeleteGuardOpen(false);
                    setCompleteSheetOpen(true);
                }}
            />

            {/* Шит «Завершить аренду?» (#627, Figma 1583:56380): канон
             * ConfirmDialog — подпись макета R/400 16/18. Кнопка в loading
             * на время мутации, закрытие глушится. */}
            <ConfirmDialog
                open={completeSheetOpen}
                onOpenChange={setCompleteSheetOpen}
                title="Завершить аренду?"
                description="Объект станет свободным, арендный платеж завершится. Данные аренды сохранятся в разделе «Прошлые аренды»"
                descriptionClassName="text-base leading-[18px]"
                confirmLabel="Завершить"
                cancelLabel="Отменить"
                pending={completeRental.isPending}
                onConfirm={handleCompleteRental}
            />

            {/* Шит пина (#630, Figma 1583:57452): объяснение перед
             * «Сделать основным» — H3-заголовок канона, подпись макета
             * R/400 16/18, кнопки столбиком: подтверждение сверху,
             * «Понятно» закрывает без действия. Кнопка в loading на время
             * мутации, закрытие глушится; отказ — тост, шит остаётся
             * открытым. Unpin исполняется без шита (в макетах его нет). */}
            <ConfirmDialog
                open={pinSheetOpen}
                onOpenChange={setPinSheetOpen}
                title="Сделать объект основным"
                description="Этот объект будет открываться первым при входе в раздел «Объекты»"
                descriptionClassName="text-base leading-[18px]"
                confirmLabel="Сделать объект основным"
                cancelLabel="Понятно"
                stacked
                pending={setPin.isPending}
                onConfirm={handlePinConfirm}
            />

            {/* Шит удаления (#629, Figma 1583:56558): канон ConfirmDialog,
             * кнопки столбиком, подтверждение danger сверху (макет — 16/18
             * под заголовком H1 28/32). Чекбокс «Удалить все данные» срезан
             * решением владельца 12.09: удаление тотальное (ADR 0049), бэк
             * уносит аренды, платежи, операции, задачи и контакты каскадом.
             * Заметка о участниках не перенесена (там письмо и так уходит).
             * Кнопка в loading на время мутации, закрытие глушится. */}
            <ConfirmDialog
                open={deleteOpen}
                onOpenChange={setDeleteOpen}
                title="Удалить объект?"
                titleClassName="text-[28px] leading-8"
                description="Объект будет удален. Вместо удаления объект можно перевести в архив"
                descriptionClassName="text-base leading-[18px]"
                confirmLabel="Удалить"
                cancelLabel="Отменить"
                confirmVariant="danger"
                stacked
                pending={deleteProperty.isPending}
                onConfirm={handleDelete}
            >
                <p className="text-sm leading-4 text-danger-soft">
                    Будут удалены данные аренд объекта, все операции объекта, платежи, контакты и задачи, связанные с объектом. Это действие нельзя отменить
                </p>
            </ConfirmDialog>
        </>
    );
}
