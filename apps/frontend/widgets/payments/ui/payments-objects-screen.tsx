"use client";

import { useState } from "react";
import type { JSX } from "react";
import { useRouter } from "next/navigation";
import NextLink from "next/link";
import { Archive, ArrowLeft, Pin, Search } from "@/shared/assets/icons";
import { ROUTES } from "@/shared/config/routes";
import { propertyPermissions } from "@/entities/property";
import { useProperties } from "@/features/properties";
import {
  PaymentsAddSheet,
  useGlobalPaymentObjects,
  useGlobalPayments,
  usePaymentWizardDraft,
} from "@/features/payments";
import { CategoryIcon } from "@/features/payment-categories";
import type {
  GlobalPayment,
  GlobalPaymentObject,
} from "@/entities/payment";
import {
  Button,
  buttonVariants,
  EmptyState,
  IconButton,
  PageContent,
  Skeleton,
  TopNav,
  TopNavTitle,
} from "@/shared/ui/design";
import { goBack } from "@/shared/lib/navigation";
import {
  isPaymentlessObject,
  paymentObjectStacks,
} from "../lib/payments-objects-model";
import {
  PaymentObjectAvatar,
  PaymentsStateCard,
} from "./payments-sections";

/**
 * Страница «Объекты» — ленд секции «Платежи объектов» (карта #573, тикет
 * #582; макет 654:7558): карточки видимых объектов (#575) со стопками
 * правил («Автоплатежи»/«Платежи», красная точка — просрочка правила).
 * Обрезка стопки — решение владельца 10.09: обе группы — по 4 иконки, одна
 * — 7, без счётчика; пустые группы не показываются. Булавка — пассивный
 * индикатор глобального скрепления (#577; закрепление живёт на странице
 * объекта, карточка только отображает), закреплённые сверху (порядок
 * сервера). Кликается вся карточка — платежи объекта. Объект без платежей
 * (решения владельца 10.09, вечер): при нескольких объектах — компактная
 * карточка-шапка (890:29967); единственный объект без платежей —
 * полноэкранное пустое состояние (1041-51463/1041-51460/1036-37530 —
 * как у глобальных платежей: контент сверху, CTA «Добавить платёж» на ПК
 * под текстом, ниже — прижата к низу) в визард этого объекта. Совсем без
 * объектов (#1004) — «Объектов пока нет» с CTA «Добавить объект». Лупа в
 * шапке — поиск объектов, на пустых состояниях спрятана (#1004);
 * карандаша макета нет (решение владельца 10.09). Внизу — «Архивные
 * объекты» во всю ширину карточек (существующий архив; архивные в
 * карточках и поиске не участвуют).
 */
export function PaymentsObjectsScreen(): JSX.Element {
  const router = useRouter();
  const feedQuery = useGlobalPayments();
  const objectsQuery = useGlobalPaymentObjects();

  const pending =
    (feedQuery.data === undefined || objectsQuery.data === undefined) &&
    !feedQuery.isError &&
    !objectsQuery.isError;
  const error = feedQuery.isError || objectsQuery.isError;
  const objects = objectsQuery.data ?? [];
  const [singleObject] = objects;
  // Право правки на единственном объекте — гейт CTA пустой книги (#703):
  // у чистого зрителя единственный объект чужой, визард упёрся бы в 403.
  const singleProperty = useProperties().data?.find(
    (property) => property.id === singleObject?.propertyId,
  );
  const canAddOnSingleObject = propertyPermissions(singleProperty).canEdit;
  // Единственный объект книги без платежей — полноэкранное пустое
  // состояние вместо списка (решение владельца 10.09).
  const singleEmpty =
    !pending &&
    !error &&
    objects.length === 1 &&
    singleObject !== undefined &&
    isPaymentlessObject(singleObject);
  const showEmptyState = !pending && !error && singleEmpty;
  // Пустые состояния (#1004): лупа поиска в шапке видна, только когда есть
  // что искать — прячется на «объектов нет вообще» и на пустоте
  // единственного объекта; вне фазы загрузки и на ошибке остаётся.
  const hideHeaderSearch =
    !pending && !error && (objects.length === 0 || showEmptyState);

  // Иконки стопок — категории правил фида (#575): объектный ответ ключей
  // категорий не несёт, джойн по paymentId.
  const paymentsById = new Map(
    (feedQuery.data?.items ?? []).map((payment) => [payment.id, payment]),
  );

  const retry = (): void => {
    void feedQuery.refetch();
    void objectsQuery.refetch();
  };

  return (
    <>
      <TopNav
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            onClick={() => goBack(router, ROUTES.payments)}
          />
        }
        trailing={
          hideHeaderSearch ? undefined : (
            <IconButton
              icon={<Search />}
              label="Поиск объектов"
              onClick={() => router.push(ROUTES.paymentsObjectsSearch)}
            />
          )
        }
      >
        <TopNavTitle title="Объекты" />
      </TopNav>

      <PageContent>
        {showEmptyState && (
          <PaymentsObjectsEmpty
            propertyId={singleObject.propertyId}
            /* Чистому зрителю CTA не рисуется (#703): единственный объект
             * чужой, визард упёрся бы в отказ сервера. */
            canAdd={canAddOnSingleObject}
          />
        )}

        {!showEmptyState && (
          <>
            {/* Поля 24px несёт обёртка (§14); детям списка вставки не нужны —
             * дефолтные маргины кнопок снял легаси-сброс форм-контролов
             * в @layer base (globals.css, button { margin: 0 }), а
             * margin-утилиты на голых <button> работают сами (§13). */}
            <div className="flex flex-col gap-4 px-6 pt-1">
              {pending && (
                <>
                  <Skeleton className="h-[120px] rounded-card" />
                  <Skeleton className="h-[120px] rounded-card" />
                </>
              )}

              {!pending && error && (
                <PaymentsStateCard
                  title="Не удалось загрузить объекты"
                  hint="Проверьте подключение и попробуйте еще раз"
                  action={
                    <Button variant="secondary" size="small" onClick={retry}>
                      Повторить
                    </Button>
                  }
                />
              )}

              {!pending && !error && objects.length === 0 && (
                <EmptyState
                  imageSrc="/images/payments/payments-empty.png"
                  title="Объектов пока нет"
                  description="Создайте объект, чтобы добавлять платежи"
                  action={
                    <Button onClick={() => router.push(ROUTES.propertyNew)}>
                      Добавить объект
                    </Button>
                  }
                />
              )}

              {!pending &&
                !error &&
                objects.map((object) => (
                  <PaymentObjectCard
                    key={object.propertyId}
                    object={object}
                    paymentsById={paymentsById}
                    onSelect={() =>
                      router.push(ROUTES.propertyPayments(object.propertyId))
                    }
                  />
                ))}

              {!pending && !error && objects.length > 0 && (
                <ArchivedObjectsButton />
              )}
            </div>
          </>
        )}
      </PageContent>
    </>
  );
}

/** Пустое состояние единственного объекта без платежей (1041-51463 /
 * 1041-51460 / 1036-37530, схема глобальных платежей): иллюстрация с
 * текстами прижаты к верху — канон EmptyState; CTA «Добавить платёж»
 * раздвоена по ярусам: на ПК (≥1024) — под текстом, на планшете и мобилке
 * — прижата к низу; сверяется с черновиком платежа объекта (#1066):
 * есть — модалка «У вас есть черновик», нет — straight в визард; у чистого
 * зрителя CTA нет вовсе (#703). Списка и кнопки архива нет. */
function PaymentsObjectsEmpty({
  propertyId,
  canAdd,
}: {
  readonly propertyId: string;
  readonly canAdd: boolean;
}): JSX.Element {
  const router = useRouter();
  const draft = usePaymentWizardDraft(propertyId, "payment");
  const [sheetOpen, setSheetOpen] = useState(false);

  const onAddPayment = canAdd
    ? () =>
        draft.hasDraft
          ? setSheetOpen(true)
          : router.push(ROUTES.propertyPaymentNew(propertyId, "payment"))
    : undefined;

  return (
    <div
      data-testid="payments-objects-empty"
      // Растягиваем блок до нижнего края видимой области: на планшете и
      // мобилке CTA прижата книзу (216px = верх страницы и нижний резерв
      // PageContent, как у PaymentsGlobalEmpty).
      className="flex min-h-[calc(100dvh-216px)] flex-col px-6"
    >
      <EmptyState
        imageSrc="/images/payments/object-empty.webp"
        title="Вы пока не добавляли платежи"
        description="Добавьте платежи, чтобы не терять их из виду"
        action={
          onAddPayment !== undefined && (
            <Button onClick={onAddPayment} className="hidden lg:inline-flex">
              Добавить платёж
            </Button>
          )
        }
      />
      {onAddPayment !== undefined && (
        // mt-auto прижимает нижнюю кнопку к краю растянутого блока; на ПК
        // она скрыта — там кнопка живёт в action-слоте под текстом.
        <Button onClick={onAddPayment} className="mt-auto lg:hidden">
          Добавить платёж
        </Button>
      )}
      {canAdd && (
        <PaymentsAddSheet
          propertyId={propertyId}
          open={sheetOpen}
          onOpenChange={setSheetOpen}
          fixedType="payment"
        />
      )}
    </div>
  );
}

/** Кнопка «Архивные объекты» (654:7558): серая, с иконкой архива, на
 * существующую страницу архива; во всю ширину карточек. Ссылка-кнопка на
 * каноне (легаси LinkButton снесён, #901): NextLink+buttonVariants
 * (next/link не дружит с Radix Slot — прецедент канонного button.tsx). */
function ArchivedObjectsButton(): JSX.Element {
  return (
    <NextLink
      href={ROUTES.propertyArchive}
      className={buttonVariants({ variant: "secondary", className: "w-full" })}
    >
      <Archive className="h-6 w-6 shrink-0" aria-hidden />
      Архивные объекты
    </NextLink>
  );
}

/** Карточка объекта (654:7558): серый блок rounded-24, шапка — аватар
 * (фото или белый круг с домом), название, адрес, булавка-индикатор у
 * закреплённых; стопки правил под шапкой. Кликается вся карточка. */
function PaymentObjectCard({
  object,
  paymentsById,
  onSelect,
}: {
  readonly object: GlobalPaymentObject;
  readonly paymentsById: ReadonlyMap<string, GlobalPayment>;
  readonly onSelect: () => void;
}): JSX.Element {
  const stacks = paymentObjectStacks(object, paymentsById);

  return (
    <button
      type="button"
      data-testid={`payments-object-card-${object.propertyId}`}
      onClick={onSelect}
      className="flex cursor-pointer flex-col gap-3 rounded-card bg-surface-muted p-6 text-left outline-none transition-opacity hover:opacity-90 active:opacity-90 focus-visible:ring-4 focus-visible:ring-primary"
    >
      <div className="flex items-center gap-3">
        <PaymentObjectAvatar photoUrl={object.photoUrl} surface="card" />
        <span className="flex min-w-0 flex-1 flex-col gap-1">
          <span className="truncate text-base font-medium leading-[18px] text-content">
            {object.name}
          </span>
          <span className="truncate text-sm leading-4 text-content-tertiary">
            {object.address}
          </span>
        </span>
        {object.pinnedAt !== null && (
          // Индикатор глобального скрепления (#577): пассивен — закрепление
          // живёт на странице объекта (решение владельца 10.09).
          <span className="flex shrink-0" aria-hidden>
            <Pin className="h-6 w-6 text-content" />
          </span>
        )}
      </div>

      {stacks.length > 0 && (
        <div className="flex gap-4">
          {stacks.map((stack) => (
            <div
              key={stack.label}
              className="flex min-w-0 flex-1 flex-col gap-3"
            >
              <span className="text-sm leading-4 text-content">
                {stack.label}
              </span>
              <div className="flex">
                {stack.keys.map((rule) => (
                  <CategoryIcon
                    key={rule.paymentId}
                    icon={rule.icon}
                    color={rule.color}
                    badge={rule.hasOverdue ? "notification" : undefined}
                    surface="muted"
                    // Нахлёст стопки 654:7558 (gap -12): каждый следующий
                    // ключ ложится на правый край предыдущего.
                    className="-ml-3 first:ml-0"
                  />
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </button>
  );
}
