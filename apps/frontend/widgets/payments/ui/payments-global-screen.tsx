"use client";

import type { JSX, ReactNode } from "react";
import Image from "next/image";
import { useRouter } from "next/navigation";
import {
  BoldHome,
  BoldObjects,
  BoldStar,
  BoldWarning,
  Filter,
  Search,
  SmallArrowDown,
} from "@/shared/assets/icons";
import { ROUTES } from "@/shared/config/routes";
import {
  CategoryIcon,
  categoryStyle,
} from "@/features/payment-categories";
import {
  useGlobalPaymentObjects,
  useGlobalPayments,
} from "@/features/payments";
import type { GlobalPayment } from "@/entities/payment";
import { PaymentCardButton } from "@/entities/payment";
import {
  Button,
  EmptyState,
  HubCollapseAnchor,
  HubTitle,
  PageContent,
  Skeleton,
  TopNav,
} from "@/shared/ui/design";
import { useKeyboardActivation } from "@/shared/lib/hooks/useKeyboardActivation";
import {
  globalFavoritePayments,
  globalOverduePayments,
  globalPaymentObjectHasOverdue,
  nearestDateLine,
  overdueDaysLine,
  overdueOperationsCountLabel,
  paymentsCountLabel,
} from "../lib/payments-global-model";
import {
  GlobalCardIcon,
  GlobalPaymentRuleIcon,
  PaymentsStateCard,
} from "./payments-sections";

/** Максимум карточек в ленте секции главного экрана (решение владельца
 * 09.09): четыре платежа, замыкающая «Все …» — пятая; остальное — на
 * странице категории. */
const SECTION_CARDS_LIMIT = 4;

/**
 * Экран «Платежи» — глобальная страница платежей (карта #573, тикет #578;
 * состав — решения владельца #574, макеты 879:9679/880:17866/879:9399).
 * Три секции: «Избранные», «Просроченные», «Платежи объектов» (секции «На
 * оплату» в срезе нет); заголовок с шевроном открывает страницу категории,
 * замыкающая карточка «Все …» ведёт туда же. Карточки платежей — канон
 * PaymentCardButton (168.5, горизонтальная лента): иконка категории, сумма,
 * дата-«Ближайший» (просто дата графика, без статуса; у просроченных — «N
 * дней» красным и красный (!) на иконке). Пустая секция — карточка-
 * плейсхолдер с 3D-иллюстрацией из Figma. Совсем без объектов — глобальное
 * пустое состояние с CTA «Добавить объект». Поиск — пилюля «Найти платёж»
 * на страницу поиска (#581); иконка-слайдеры справа — декор макета, не
 * кликается. Один объект в книге — тап по его карточке ведёт сразу на
 * платежи объекта, без страницы «Объекты».
 */
export function PaymentsGlobalScreen(): JSX.Element {
  const router = useRouter();
  const feedQuery = useGlobalPayments();
  const objectsQuery = useGlobalPaymentObjects();

  // Скелетон — пока данных нет вовсе (первая загрузка); ошибка без данных
  // показывает карточку повтора, не скелетон (канон состояний).
  const pending =
    (feedQuery.data === undefined || objectsQuery.data === undefined) &&
    !feedQuery.isError &&
    !objectsQuery.isError;
  const error = feedQuery.isError || objectsQuery.isError;
  const objects = objectsQuery.data ?? [];
  // Глобальное пустое (879:9399): видимых объектов нет — платежам негде
  // жить; разделы-плейсхолдеры при этом не показываются.
  const globalEmpty = !pending && !error && objects.length === 0;

  const retry = (): void => {
    void feedQuery.refetch();
    void objectsQuery.refetch();
  };

  const openPayment = (payment: GlobalPayment): void =>
    router.push(ROUTES.propertyPayment(payment.propertyId, payment.id));

  return (
    <>
      {/* Хаб-шапка: «крылья» (лого + профиль) и на мобайле; в компакт-баре
       * при сворачивании — лупа на поиск платежей (канон «Операций» #543). */}
      <TopNav
        mobileWings
        collapse={{
          title: "Платежи",
          search: { href: ROUTES.paymentsSearch, label: "Найти платёж" },
        }}
      />

      <PageContent>
        <HubCollapseAnchor>
          <HubTitle>Платежи</HubTitle>
        </HubCollapseAnchor>

        {globalEmpty ? (
          <PaymentsGlobalEmpty
            onAddProperty={() => router.push(ROUTES.propertyNew)}
          />
        ) : (
          <div className="-mx-5 flex min-[1200px]:mx-0 flex-col gap-6 px-6 pt-4">
            {/* Ритм страницы — ровно 24px по бокам, как на «Операциях»:
             * контент кабинета даёт 20px до 1200px, страница выравнивает
             * себя до 24 сама. */}
            <PaymentsSearchPill
              onOpenSearch={() => router.push(ROUTES.paymentsSearch)}
            />

            {pending && (
              <>
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
                      testId="payments-section-objects"
                      title="Платежи объектов"
                      onOpen={() => router.push(ROUTES.paymentsObjects)}
                    >
                      <ObjectsSectionBody
                        objects={objects
                          .map((object) => ({
                            propertyId: object.propertyId,
                            name: object.name,
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
    </>
  );
}

/** Пилюля поиска (879:9683): серый rounded-pill, лупа и подпись «Найти
 * платёж»; тап открывает страницу поиска (#581). Слайдеры справа — декор
 * макета: не кнопка, кликается вся пилюля целиком. */
function PaymentsSearchPill({
  onOpenSearch,
}: {
  readonly onOpenSearch: () => void;
}): JSX.Element {
  const activatorProps = useKeyboardActivation({ onSelect: onOpenSearch });

  return (
    <div
      {...activatorProps}
      data-testid="payments-search-pill"
      className="flex h-14 w-full cursor-pointer items-center rounded-pill bg-surface-muted pl-[18px] pr-4 text-left outline-none transition-opacity hover:opacity-90 focus-visible:ring-4 focus-visible:ring-primary active:opacity-90"
    >
      <Search className="h-6 w-6 shrink-0 text-content" aria-hidden />
      <span className="min-w-0 flex-1 truncate px-2 text-base font-medium text-content">
        Найти платёж
      </span>
      <Filter className="h-6 w-6 shrink-0 text-content" aria-hidden />
    </div>
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
          className="flex w-full cursor-pointer items-center justify-between gap-3 rounded-pill outline-none transition-opacity hover:opacity-80 active:opacity-80 focus-visible:ring-4 focus-visible:ring-primary"
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

/** Тело секции «Избранные»: лента карточек + замыкающая «Все избранные»
 * (синяя звезда, счётчик целого скоупа) либо плейсхолдер (880:18354). */
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
    <div className="flex snap-x snap-mandatory gap-2 overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
      {favorites.map((payment) => {
        return (
          <PaymentCardButton
            key={payment.id}
            className="snap-start"
            leading={<GlobalPaymentRuleIcon payment={payment} surface="muted" />}
            title={payment.title}
            subtitle={payment.propertyName}
            amountKopecks={payment.amountKopecks}
            description={nearestDateLine(payment)}
            onSelect={() => onSelectPayment(payment)}
          />
        );
      })}
      <PaymentCardButton
        className="snap-start"
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
    </div>
  );
}

/** Тело секции «Просроченные»: карточки с красными суммой, сроком («N
 * дней») и (!) на иконке; тап ведёт на страницу СТАРЕЙШЕЙ просроченной
 * операции (решение владельца 09.09), не на правило; замыкающая «Все
 * просроченные» (879:9700) считает просроченные операции целого скоупа
 * (#575). */
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
    <div className="flex snap-x snap-mandatory gap-2 overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
      {overdue.map((payment) => {
        const style = categoryStyle(
          payment.category.source,
          payment.category.slug,
        );

        return (
          <PaymentCardButton
            key={payment.id}
            className="snap-start"
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
      <PaymentCardButton
        className="snap-start"
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
    </div>
  );
}

/** Тело секции «Платежи объектов» (879:9711): карточки объектов (белый
 * круг с домом, красная точка при просрочке в стопках) и замыкающая
 * «Показать все» на страницу «Объекты» (#582) — больше одного объекта;
 * единственный объект ведёт сразу на свои платежи без страницы-списка. */
function ObjectsSectionBody({
  objects,
  onSelectObject,
  onOpenAll,
}: {
  readonly objects: ReadonlyArray<
    Readonly<{ propertyId: string; name: string; hasOverdue: boolean }>
  >;
  readonly onSelectObject: (propertyId: string) => void;
  readonly onOpenAll: () => void;
}): JSX.Element {
  return (
    <div className="flex snap-x snap-mandatory gap-2 overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
      {objects.map((object) => (
        <PaymentCardButton
          key={object.propertyId}
          className="snap-start"
          leading={
            <GlobalCardIcon variant="white" hasNotification={object.hasOverdue}>
              <BoldHome />
            </GlobalCardIcon>
          }
          title={object.name}
          onSelect={() => onSelectObject(object.propertyId)}
        />
      ))}
      {objects.length > 1 && (
        <PaymentCardButton
          className="snap-start"
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
    </div>
  );
}

/** Пустая секция (880:17866): серая карточка с пояснением и 3D-
 * иллюстрацией из Figma справа. */
function PaymentsGlobalPlaceholder({
  image,
  children,
}: {
  readonly image: string;
  readonly children: ReactNode;
}): JSX.Element {
  return (
    <div className="flex min-h-16 w-full items-center gap-6 rounded-card bg-surface-muted px-6">
      <p className="min-w-0 flex-1 text-base leading-[18px] text-content-secondary">
        {children}
      </p>
      <Image
        src={image}
        alt=""
        width={64}
        height={64}
        className="h-16 w-16 shrink-0"
      />
    </div>
  );
}

/** Глобальное пустое состояние (879:9399): иллюстрация и текст по центру,
 * CTA «Добавить объект» прижата к низу страницы. */
function PaymentsGlobalEmpty({
  onAddProperty,
}: {
  readonly onAddProperty: () => void;
}): JSX.Element {
  return (
    <div
      data-testid="payments-global-empty"
      // Растягиваем блок до нижнего края видимой области: CTA прижата книзу
      // (879:9399). 216px = верх страницы (PageHeader + отступ контента) и
      // нижний резерв PageContent под TabBar (pb-[136px]).
      className="-mx-5 flex min-h-[calc(100dvh-216px)] min-[1200px]:mx-0 flex-col px-6"
    >
      <div className="flex flex-1 items-center justify-center">
        <EmptyState
          imageSrc="/images/payments/payments-empty.png"
          title="Вы пока не добавляли платежи"
          description="Создайте объект, чтобы добавлять платежи"
          descriptionClassName="text-content"
          className="pt-0"
        />
      </div>
      <Button onClick={onAddProperty}>Добавить объект</Button>
    </div>
  );
}

/** Скелетон секции на время загрузки: строка заголовка и пара плиток
 * карточек. */
function PaymentsGlobalSectionSkeleton(): JSX.Element {
  return (
    <section className="flex flex-col gap-4" aria-hidden>
      <Skeleton className="h-6 w-40" />
      <div className="flex gap-2">
        <Skeleton className="h-[132px] w-[168.5px] shrink-0" />
        <Skeleton className="h-[132px] w-[168.5px] shrink-0" />
      </div>
    </section>
  );
}
