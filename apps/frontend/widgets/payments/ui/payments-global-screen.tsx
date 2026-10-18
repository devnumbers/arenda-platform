"use client";

import { useState, type JSX, type ReactNode } from "react";
import Image from "next/image";
import { useRouter } from "next/navigation";
import {
  Add,
  BoldClock,
  BoldObjects,
  BoldStar,
  BoldWarning,
  SmallArrowDown,
} from "@/shared/assets/icons";
import { ROUTES } from "@/shared/config/routes";
import {
  CategoryIcon,
  categoryStyle,
} from "@/features/payment-categories";
import { propertyPermissions } from "@/entities/property";
import { useProperties } from "@/features/properties";
import {
  PaymentsAddSheet,
  useGlobalPaymentObjects,
  useGlobalPayments,
  usePaymentsDue,
} from "@/features/payments";
import type { GlobalPayment, PaymentDueItem } from "@/entities/payment";
import type { PropertyType } from "@/entities/property";
import { dueDaysLabel, PaymentCardButton } from "@/entities/payment";
import {
  Button,
  EmptyState,
  HubCollapseAnchor,
  HubTitle,
  IconButton,
  PageContent,
  SearchPill,
  Skeleton,
  TopNav,
} from "@/shared/ui/design";
import { SnapRail } from "@/shared/ui/snap-rail";
import { globalFavoritePayments,
  globalOverduePayments,
  globalPaymentObjectHasOverdue,
  nearestDateLine,
  overdueDaysLine,
  overdueOperationsCountLabel,
  paymentHubAddTarget,
  paymentHubSectionHasAllCard,
  paymentsCountLabel,
} from "../lib/payments-global-model";
import {
  GlobalCardIcon,
  GlobalPaymentRuleIcon,
  PaymentObjectAvatar,
  PaymentsStateCard,
} from "./payments-sections";

/** Максимум карточек в ленте секции главного экрана (решение владельца
 * 09.09): четыре платежа, замыкающая «Все …» — пятая; остальное — на
 * странице категории. */
const SECTION_CARDS_LIMIT = 4;

/**
 * Экран «Платежи» — глобальная страница платежей (карта #573, тикет #578;
 * состав — решения владельца #574, макеты 879:9679/880:17866, пустое
 * состояние — 1041-51463/1041-51460/1036-37530; новый первый блок — макет
 * 3226-75059, хедер 3229-94522, тикет #1235).
 * Секции (порядок макета 1036:37525): «Избранные», «Просроченные»,
 * «На оплату» (#1347 — между «Просроченными» и «Платежами объектов»),
 * «Платежи объектов». Заголовок с шевроном открывает страницу категории,
 * замыкающая карточка «Все …» ведёт туда же и видна от трёх карточек
 * секции (решение владельца #1339, единообразно для всех секций).
 * Карточки платежей — канон PaymentCardButton (168.5, горизонтальная
 * лента): иконка категории, сумма, дата-«Ближайший» (просто дата графика,
 * без статуса; у просроченных — «N дней» красным и красный (!) на иконке;
 * у «На оплату» — метка «Сегодня/Завтра/Через N дней» синим, 1036:37525).
 * Пустая секция — карточка-плейсхолдер с 3D-иллюстрацией из Figma. Совсем
 * без объектов — глобальное пустое состояние с CTA «Добавить объект».
 * Поиск — пилюля «Найти платёж» на страницу поиска (#581); иконка-слайдеры
 * справа — декор макета, не кликается.
 * «+» создания (макет 3226-75059) — в ряду заголовка и в правом слоте
 * компакт-бара; видна, когда в книге есть хоть один объект с правом правки
 * (#703), на глобальной пустоте скрыта — действие там CTA состояния.
 * Ведёт через существующие поверхности (новых роутов создания нет): у
 * единственного редактируемого объекта — шит единого входа #453 (черновик →
 * выбор типа → визард его объекта), у нескольких — страница «Объекты»
 * (#582: карточка → платежи объекта → «Добавить»). Один объект в книге —
 * тап по его карточке ведёт сразу на платежи объекта, без страницы
 * «Объекты».
 */
export function PaymentsGlobalScreen(): JSX.Element {
  const router = useRouter();
  const feedQuery = useGlobalPayments();
  const objectsQuery = useGlobalPaymentObjects();
  const propertiesQuery = useProperties();
  const dueQuery = usePaymentsDue();

  // Скелетон — пока данных нет вовсе (первая загрузка); ошибка без данных
  // показывает карточку повтора, не скелетон (канон состояний).
  const pending =
    (feedQuery.data === undefined ||
      objectsQuery.data === undefined ||
      dueQuery.data === undefined) &&
    !feedQuery.isError &&
    !objectsQuery.isError &&
    !dueQuery.isError;
  const error = feedQuery.isError || objectsQuery.isError || dueQuery.isError;
  const objects = objectsQuery.data ?? [];
  // Глобальное пустое (1041-51463/1041-51460/1036-37530): видимых
  // объектов нет — платежам негде
  // жить; разделы-плейсхолдеры при этом не показываются.
  const globalEmpty = !pending && !error && objects.length === 0;

  // «+» создания: справочник объектов не загружен, упал или редактируемых
  // нет — консервативно скрыта (canon #703, как у «Операций»); на
  // глобальной пустоте скрыта — создание объекта там CTA состояния.
  const editableIds = (propertiesQuery.data ?? [])
    .filter((property) => propertyPermissions(property).canEdit)
    .map((property) => property.id);
  const addTarget = paymentHubAddTarget(editableIds);
  const showAdd = !globalEmpty && addTarget !== null;

  // Шит единого входа помнит свой объект: цель может смениться на
  // «Объекты» после перечитывания справочника — открытая модалка не должна
  // перемонтироваться.
  const [addSheetOpen, setAddSheetOpen] = useState(false);
  const [addSheetPropertyId, setAddSheetPropertyId] = useState<string | null>(
    null,
  );

  const openAdd = (): void => {
    if (addTarget === null) return;
    if (addTarget.kind === "objects") {
      router.push(ROUTES.paymentsObjects);
      return;
    }
    setAddSheetPropertyId(addTarget.propertyId);
    setAddSheetOpen(true);
  };

  const retry = (): void => {
    void feedQuery.refetch();
    void objectsQuery.refetch();
    void dueQuery.refetch();
  };

  const openPayment = (payment: GlobalPayment): void =>
    router.push(ROUTES.propertyPayment(payment.propertyId, payment.id));

  // Два экземпляра узла: у ряда заголовка свой testid — слоты TopNav
  // рендерят свой узел в трёх местах (крыло, инлайн-компакт, мобайл-клон),
  // общий testid давал бы строгую неоднозначность в e2e. Fade-in 250мс:
  // кнопка приезжает после резолва справочника — короткий фейд закрывает
  // шов телепорта (#1349, dl-fade-in в globals.css; fade — канон §8).
  const addButton = (
    <IconButton
      icon={<Add />}
      label="Создать платёж"
      data-testid="payments-create"
      className="animate-[dl-fade-in_250ms_var(--dl-ease)_both]"
      onClick={openAdd}
    />
  );
  const addButtonCompact = (
    <IconButton
      icon={<Add />}
      label="Создать платёж"
      data-testid="payments-create-compact"
      className="animate-[dl-fade-in_250ms_var(--dl-ease)_both]"
      onClick={openAdd}
    />
  );

  return (
    <>
      {/* Хаб-шапка: «крылья» (лого + профиль) и на мобайле; в компакт-баре
       * при сворачивании — лупа на поиск платежей (канон «Операций» #543)
       * и «+» справа (макет 3229-94522, канон сворачивания). */}
      <TopNav
        mobileWings
        collapse={{
          title: "Платежи",
          search: { href: ROUTES.paymentsSearch, label: "Найти платёж" },
          trailing: showAdd ? addButtonCompact : undefined,
        }}
      />

      <PageContent>
        <HubCollapseAnchor>
          {/* Строка заголовка h-8 с «+» (макет 3226-75059, паттерн хаба
           * «Объектов» #1234): кнопка 44 переполняет строку симметрично —
           * центрирована против линии заголовка. */}
          <div className="flex h-8 items-center justify-between pr-3.5">
            <HubTitle>Платежи</HubTitle>
            {showAdd && addButton}
          </div>
        </HubCollapseAnchor>

        {globalEmpty ? (
          <PaymentsGlobalEmpty
            onAddProperty={() => router.push(ROUTES.propertyNew)}
          />
        ) : (
          <div className="flex flex-col gap-6 px-6 pt-4">
            <PaymentsSearchPill
              onOpenSearch={() => router.push(ROUTES.paymentsSearch)}
            />

            {pending && (
              <>
                <PaymentsGlobalSectionSkeleton />
                <PaymentsGlobalSectionSkeleton />
                <PaymentsGlobalSectionSkeleton />
                <PaymentsGlobalSectionSkeleton />
              </>
            )}

            {!pending && (
              <>
                {error ? (
                  <PaymentsStateCard
                    title="Не удалось загрузить платежи"
                    hint="Проверьте подключение и попробуйте еще раз"
                    action={
                      <Button
                        variant="secondary"
                        size="small"
                        onClick={retry}
                      >
                        Повторить
                      </Button>
                    }
                  />
                ) : (
                  <>
                    <PaymentsGlobalSection
                      testId="payments-section-favorites"
                      title="Избранные"
                      onOpen={() => router.push(ROUTES.paymentsFavorites)}
                    >
                      <FavoritesSectionBody
                        favorites={globalFavoritePayments(
                          feedQuery.data?.items ?? [],
                        ).slice(0, SECTION_CARDS_LIMIT)}
                        favoriteCount={feedQuery.data?.favoriteCount ?? 0}
                        onSelectPayment={openPayment}
                        onOpenAll={() =>
                          router.push(ROUTES.paymentsFavorites)
                        }
                      />
                    </PaymentsGlobalSection>

                    <PaymentsGlobalSection
                      testId="payments-section-overdue"
                      title="Просроченные"
                      onOpen={() => router.push(ROUTES.paymentsOverdue)}
                    >
                      <OverdueSectionBody
                        overdue={globalOverduePayments(
                          feedQuery.data?.items ?? [],
                        ).slice(0, SECTION_CARDS_LIMIT)}
                        overdueOperationsCount={
                          feedQuery.data?.overdueOperationsCount ?? 0
                        }
                        onSelectOperation={(propertyId, operationId) =>
                          router.push(
                            ROUTES.propertyOperation(propertyId, operationId),
                          )
                        }
                        onSelectPayment={openPayment}
                        onOpenAll={() => router.push(ROUTES.paymentsOverdue)}
                      />
                    </PaymentsGlobalSection>

                    <PaymentsGlobalSection
                      testId="payments-section-due"
                      title="На оплату"
                      onOpen={() => router.push(ROUTES.paymentsDue)}
                    >
                      <DueSectionBody
                        due={(dueQuery.data?.items ?? []).slice(
                          0,
                          SECTION_CARDS_LIMIT,
                        )}
                        dueOperationsCount={
                          feedQuery.data?.dueOperationsCount ?? 0
                        }
                        onSelectPayment={(item) =>
                          router.push(
                            ROUTES.propertyPayment(
                              item.propertyId,
                              item.paymentId,
                            ),
                          )
                        }
                        onOpenAll={() => router.push(ROUTES.paymentsDue)}
                      />
                    </PaymentsGlobalSection>

                    <PaymentsGlobalSection
                      testId="payments-section-objects"
                      title="Платежи объектов"
                      onOpen={() => router.push(ROUTES.paymentsObjects)}
                    >
                      <ObjectsSectionBody
                        objects={objects
                          .map((object) => ({
                            propertyId: object.propertyId,
                            name: object.name,
                            type: object.type,
                            photoUrl: object.photoUrl,
                            hasOverdue: globalPaymentObjectHasOverdue(object),
                          }))
                          .slice(0, SECTION_CARDS_LIMIT)}
                        onSelectObject={(propertyId) =>
                          router.push(ROUTES.propertyPayments(propertyId))
                        }
                        onOpenAll={() => router.push(ROUTES.paymentsObjects)}
                      />
                    </PaymentsGlobalSection>
                  </>
                )}
              </>
            )}
          </div>
        )}
      </PageContent>

      {/* Единый вход в создание платежа (#453) для единственного
       * редактируемого объекта: черновик → выбор типа → визард его
       * объекта. Без fixedType — шит сам разбирает черновики обоих типов. */}
      {addSheetPropertyId !== null && (
        <PaymentsAddSheet
          propertyId={addSheetPropertyId}
          open={addSheetOpen}
          onOpenChange={setAddSheetOpen}
        />
      )}
    </>
  );
}

/** Пилюля поиска платежей (879:9683) — адаптер канона SearchPill: подпись
 * «Найти платёж», тап открывает страницу поиска (#581). Декор-слайдеры
 * справа снесены (решение владельца 01.10): мёртвый элемент без действия,
 * пилюля кликается целиком. Экспорт для route-loading (#609). */
export function PaymentsSearchPill({
  onOpenSearch,
}: {
  readonly onOpenSearch: () => void;
}): JSX.Element {
  return (
    <SearchPill
      onOpenSearch={onOpenSearch}
      label="Найти платёж"
      testId="payments-search-pill"
    />
  );
}

/** Секция главного экрана (879:9688): заголовок-кнопка с шевроном — вся
 * строка кликабельна и ведёт на страницу категории (кнопка внутри h2 —
 * валидная вложенность, доступное имя = видимый текст); контент (лента или
 * плейсхолдер) приносит отступ сверху сам. */
function PaymentsGlobalSection({
  title,
  onOpen,
  testId,
  children,
}: {
  readonly title: string;
  readonly onOpen: () => void;
  readonly testId: string;
  readonly children: ReactNode;
}): JSX.Element {
  return (
    <section data-testid={testId} className="flex flex-col gap-4">
      <h2 className="text-xl font-semibold leading-6 text-content">
        <button
          type="button"
          onClick={onOpen}
          className="pressable flex w-full cursor-pointer items-center justify-between gap-3 rounded-pill outline-none hover:opacity-80 focus-visible:ring-4 focus-visible:ring-primary"
        >
          {title}
          <SmallArrowDown
            className="-rotate-90 shrink-0 text-content-tertiary"
            aria-hidden
          />
        </button>
      </h2>
      {children}
    </section>
  );
}

/** Лента секции хаба: канонный SnapRail (нативный тач + drag-прокси мыши
 * #1349, механика — DESIGN.md §8 «Drag горизонтальных snap-лент») с bleed
 * на мобайл-ярусе (≤560): `-mx-6` гасит собственный `px-6` обёртки хаба
 * (легитимный inner -mx против контейнерного паддинга, «Column width
 * rule»), `px-6` держит карточки на визуальной линии 24, а
 * `scroll-padding-inline: 24px` делает линию 24 линией снапа нативного
 * скролла и drag-прокси. */
function SectionRail({
  testId,
  children,
}: {
  readonly testId: string;
  readonly children: ReactNode;
}): JSX.Element {
  return (
    <SnapRail
      testId={testId}
      className="max-tablet:-mx-6 max-tablet:px-6 max-tablet:[scroll-padding-inline:24px]"
    >
      {children}
    </SnapRail>
  );
}

/** Тело секции «Избранные»: лента карточек + замыкающая «Все избранные»
 * (синяя звезда, счётчик целого скоупа) от трёх карточек — решение
 * владельца #1339, — либо плейсхолдер (880:18354). */
function FavoritesSectionBody({
  favorites,
  favoriteCount,
  onSelectPayment,
  onOpenAll,
}: {
  readonly favorites: ReadonlyArray<GlobalPayment>;
  readonly favoriteCount: number;
  readonly onSelectPayment: (payment: GlobalPayment) => void;
  readonly onOpenAll: () => void;
}): JSX.Element {
  if (favorites.length === 0) {
    return (
      <PaymentsGlobalPlaceholder image="/images/payments/favorites-empty.png">
        Здесь будут платежи, которые вы добавите в избранное
      </PaymentsGlobalPlaceholder>
    );
  }

  return (
    <SectionRail testId="payments-rail-favorites">
      {favorites.map((payment) => {
        return (
          <PaymentCardButton
            key={payment.id}
            leading={<GlobalPaymentRuleIcon payment={payment} surface="muted" />}
            title={payment.title}
            subtitle={payment.propertyName}
            amountKopecks={payment.amountKopecks}
            description={nearestDateLine(payment)}
            onSelect={() => onSelectPayment(payment)}
          />
        );
      })}
      {paymentHubSectionHasAllCard(favorites.length) && (
        <PaymentCardButton
          leading={
            <GlobalCardIcon>
              <BoldStar />
            </GlobalCardIcon>
          }
          title="Все избранные"
          subtitle={paymentsCountLabel(favoriteCount)}
          accentTitle
          onSelect={onOpenAll}
        />
      )}
    </SectionRail>
  );
}

/** Тело секции «Просроченные»: карточки с красными суммой, сроком («N
 * дней») и (!) на иконке; тап ведёт на страницу СТАРЕЙШЕЙ просроченной
 * операции (решение владельца 09.09), не на правило; замыкающая «Все
 * просроченные» (879:9700) считает просроченные операции целого скоупа
 * (#575) и видна от трёх карточек (решение владельца #1339). */
function OverdueSectionBody({
  overdue,
  overdueOperationsCount,
  onSelectOperation,
  onSelectPayment,
  onOpenAll,
}: {
  readonly overdue: ReadonlyArray<GlobalPayment>;
  readonly overdueOperationsCount: number;
  readonly onSelectOperation: (
    propertyId: string,
    operationId: string,
  ) => void;
  readonly onSelectPayment: (payment: GlobalPayment) => void;
  readonly onOpenAll: () => void;
}): JSX.Element {
  if (overdue.length === 0) {
    return (
      <PaymentsGlobalPlaceholder image="/images/payments/overdue-empty.png">
        Здесь будут просроченные платежи
      </PaymentsGlobalPlaceholder>
    );
  }

  return (
    <SectionRail testId="payments-rail-overdue">
      {overdue.map((payment) => {
        const style = categoryStyle(
          payment.category.source,
          payment.category.slug,
        );

        return (
          <PaymentCardButton
            key={payment.id}
            leading={
              <CategoryIcon
                icon={style.icon}
                color={style.color}
                badge="danger"
                surface="muted"
              />
            }
            title={payment.title}
            subtitle={payment.propertyName}
            amountKopecks={payment.amountKopecks}
            description={overdueDaysLine(payment)}
            danger
            onSelect={() =>
              payment.oldestOverdueOperationId !== null
                ? onSelectOperation(
                    payment.propertyId,
                    payment.oldestOverdueOperationId,
                  )
                : onSelectPayment(payment)
            }
          />
        );
      })}
      {paymentHubSectionHasAllCard(overdue.length) && (
        <PaymentCardButton
          leading={
            <GlobalCardIcon>
              <BoldWarning />
            </GlobalCardIcon>
          }
          title="Все просроченные"
          subtitle={overdueOperationsCountLabel(overdueOperationsCount)}
          accentTitle
          onSelect={onOpenAll}
        />
      )}
    </SectionRail>
  );
}

/** Тело секции «На оплату» (#1347, макет 1036:37525): карточки строк окна
 * напоминаний #1346 — иконка категории, сумма, метка дня «Сегодня/Завтра/
 * Через N дней» синим (accentDescription), название и объект; тап ведёт
 * на страницу ПРАВИЛА (сегмент read-only, оплата — на объектных экранах).
 * Замыкающая «Все платежи на оплату» (синие часы, счётчик целого скоупа)
 * — по правилу ≥3 (решение владельца #1339). */
function DueSectionBody({
  due,
  dueOperationsCount,
  onSelectPayment,
  onOpenAll,
}: {
  readonly due: ReadonlyArray<PaymentDueItem>;
  readonly dueOperationsCount: number;
  readonly onSelectPayment: (item: PaymentDueItem) => void;
  readonly onOpenAll: () => void;
}): JSX.Element {
  if (due.length === 0) {
    return (
      <PaymentsGlobalPlaceholder image="/images/payments/due-empty.png">
        Здесь будут платежи на оплату
      </PaymentsGlobalPlaceholder>
    );
  }

  return (
    <SectionRail testId="payments-rail-due">
      {due.map((item) => {
        const style = categoryStyle(item.category.source, item.category.slug);

        return (
          <PaymentCardButton
            key={item.operationId}
            leading={
              <CategoryIcon
                icon={style.icon}
                color={style.color}
                surface="muted"
              />
            }
            title={item.title}
            subtitle={item.propertyName}
            amountKopecks={item.amountKopecks}
            description={dueDaysLabel(item.daysLeft)}
            accentDescription
            onSelect={() => onSelectPayment(item)}
          />
        );
      })}
      {paymentHubSectionHasAllCard(due.length) && (
        <PaymentCardButton
          leading={
            <GlobalCardIcon>
              <BoldClock />
            </GlobalCardIcon>
          }
          title="Все платежи на оплату"
          subtitle={paymentsCountLabel(dueOperationsCount)}
          accentTitle
          onSelect={onOpenAll}
        />
      )}
    </SectionRail>
  );
}

/** Тело секции «Платежи объектов» (879:9711): карточки объектов (белый
 * круг с фото, иначе глиф типа; красная точка при просрочке в стопках) и замыкающая
 * «Показать все» на страницу «Объекты» (#582) от трёх карточек — решение
 * владельца #1339. */
function ObjectsSectionBody({
  objects,
  onSelectObject,
  onOpenAll,
}: {
  readonly objects: ReadonlyArray<
    Readonly<{
      propertyId: string;
      name: string;
      type: PropertyType;
      photoUrl: string | null;
      hasOverdue: boolean;
    }>
  >;
  readonly onSelectObject: (propertyId: string) => void;
  readonly onOpenAll: () => void;
}): JSX.Element {
  return (
    <SectionRail testId="payments-rail-objects">
      {objects.map((object) => (
        <PaymentCardButton
          key={object.propertyId}
          leading={
            // Канонный аватар «глобальных платежей» (решение #1286): фото
            // из DTO GlobalPaymentObject.photoUrl (#582) во весь круг 44 —
            // глиф типа; битое фото (фото или объект удалили — 404 стрима)
            // откатывается к глифу, канон #1275. Точка просрочки — у аватара.
            <PaymentObjectAvatar
              photoUrl={object.photoUrl}
              type={object.type}
              surface="card"
              hasNotification={object.hasOverdue}
            />
          }
          title={object.name}
          onSelect={() => onSelectObject(object.propertyId)}
        />
      ))}
      {paymentHubSectionHasAllCard(objects.length) && (
        <PaymentCardButton
          leading={
            <GlobalCardIcon>
              <BoldObjects />
            </GlobalCardIcon>
          }
          title="Показать все"
          accentTitle
          onSelect={onOpenAll}
        />
      )}
    </SectionRail>
  );
}

/** Пустая секция (1043:52801/52802, сверно 13.10): серая карточка —
 * текст-плейсхолдер слева и 3D-иллюстрация 64 из Figma справа; высота —
 * hug по контенту, боковые поля асимметричны (28 слева / 24 справа,
 * как в макете). */
function PaymentsGlobalPlaceholder({
  image,
  children,
}: {
  readonly image: string;
  readonly children: ReactNode;
}): JSX.Element {
  return (
    <div className="flex w-full min-h-[96px] items-center gap-6 rounded-card bg-surface-muted pl-7 pr-6">
      <p className="min-w-0 flex-1 text-base leading-[18px] text-content-secondary">
        {children}
      </p>
      {/* Fade-in 350мс (dl-fade-in): иллюстрация проявляется вместе с
       * секцией при первом показе — разом всплывающие 3D-картинки (#1349). */}
      <Image
        src={image}
        alt=""
        width={64}
        height={64}
        className="h-16 w-16 shrink-0 animate-[dl-fade-in_350ms_var(--dl-ease)_both]"
      />
    </div>
  );
}

/** Глобальное пустое состояние (1041-51463 ПК / 1041-51460 планшет /
 * 1036-37530 мобилка): иллюстрация, заголовок и описание прижаты к верху —
 * канон EmptyState, без вертикального центрирования. CTA «Добавить
 * объект» — одна, под текстом в action-слоте (анатомия 3235-74057);
 * раздвоение по ярусам с нижней кнопкой снято решением владельца 12.10. */
function PaymentsGlobalEmpty({
  onAddProperty,
}: {
  readonly onAddProperty: () => void;
}): JSX.Element {
  return (
    <div data-testid="payments-global-empty">
      <EmptyState
        imageSrc="/images/payments/payments-empty.png"
        title="Вы пока не добавляли платежи"
        description="Создайте объект, чтобы добавлять платежи"
        action={
          <Button size="small" radius="m" onClick={onAddProperty}>
            Добавить объект
          </Button>
        }
      />
    </div>
  );
}

/** Секция-заглушка хаба «Платежи» (#605): каркас PaymentsGlobalSection —
 * заголовок и лента плиток-карточек на время загрузки. Ряд плиток несёт
 * тот же bleed ≤560, что и живая лента (SectionRail), — на мобайл-ярусе
 * данные занимают место скелетона без сдвига в край (#1349). Плитка 137px —
 * точная высота живой PaymentCardButton (паритет #605: данные занимают
 * место скелетона без сдвига; 16+44+12+30+4+15+16, двухстрочный заголовок).
 * Экспорт для route-loading (#609). */
export function PaymentsGlobalSectionSkeleton(): JSX.Element {
  return (
    <section className="flex flex-col gap-4" aria-hidden>
      <Skeleton className="h-6 w-40" />
      <div className="flex gap-2 max-tablet:-mx-6 max-tablet:px-6">
        <Skeleton className="h-[137px] w-[168.5px] shrink-0" />
        <Skeleton className="h-[137px] w-[168.5px] shrink-0" />
      </div>
    </section>
  );
}
