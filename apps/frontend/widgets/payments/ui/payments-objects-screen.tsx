"use client";

import type { JSX } from "react";
import Image from "next/image";
import { useRouter } from "next/navigation";
import { Archive, Pin, Search } from "@/shared/assets/icons";
import { ROUTES } from "@/shared/config/routes";
import {
  useGlobalPaymentObjects,
  useGlobalPayments,
} from "@/features/payments";
import { CategoryIcon } from "@/features/payment-categories";
import type {
  GlobalPayment,
  GlobalPaymentObject,
} from "@/entities/payment";
import {
  Button,
  EmptyState,
  IconButton,
  PageContent,
  Skeleton,
} from "@/shared/ui/design";
import { LinkButton } from "@/shared/ui/link-button";
import { PageHeader } from "@/shared/ui/page-header";
import { paymentObjectStacks } from "../lib/payments-objects-model";
import {
  PaymentObjectAvatar,
  PaymentsStateCard,
} from "./payments-sections";

/**
 * Страница «Объекты» — ленд секции «Платежи объектов» (карта #573, тикет
 * #582; макеты 654:7558/880:17866): карточки видимых объектов (#575) со
 * стопками правил («Автоплатежи»/«Платежи», красная точка — просрочка
 * правила). Обрезка стопки — решение владельца 10.09: обе группы — по 4
 * иконки, одна — 7, без счётчика; пустые группы не показываются. Булавка —
 * пассивный индикатор глобального скрепления (#577; закрепление живёт на
 * странице объекта, карточка только отображает), закреплённые сверху
 * (порядок сервера). Кликается вся карточка — платежи объекта; объект без
 * платежей показывает «Вы еще не добавили ни одного платежа» с подписью
 * «Добавить» (не кнопка; 880:17866). Лупа в шапке — поиск объектов;
 * карандаша макета нет (решение владельца 10.09). Внизу — «Архивные
 * объекты» (существующий архив; архивные в карточках и поиске не участвуют).
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
      <PageHeader
        backHref={ROUTES.payments}
        title="Объекты"
        actions={
          <IconButton
            icon={<Search />}
            label="Поиск объектов"
            onClick={() => router.push(ROUTES.paymentsObjectsSearch)}
          />
        }
      />

      <PageContent>
        {/* Ритм 24px несут сами элементы (mx-6, как секции payments-
         * sections): обёртка только гасит вставку PageContent — карточка
         * состояния со своим mx-6 не задваивается. */}
        <div className="-mx-5 flex min-[1200px]:mx-0 flex-col gap-4 pt-1">
          {pending && (
            <>
              <Skeleton className="mx-6 h-[176px] rounded-card" />
              <Skeleton className="mx-6 h-[176px] rounded-card" />
            </>
          )}

          {!pending &&
            (error ? (
              <PaymentsStateCard
                title="Не удалось загрузить объекты"
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
                {objects.length === 0 && (
                  <EmptyState
                    imageSrc="/images/payments/payments-empty.png"
                    title="Объектов пока нет"
                    description="Создайте объект, чтобы добавлять платежи"
                    className="py-16"
                  />
                )}

                {objects.map((object) => (
                  <PaymentObjectCard
                    key={object.propertyId}
                    object={object}
                    paymentsById={paymentsById}
                    onSelect={() =>
                      router.push(ROUTES.propertyPayments(object.propertyId))
                    }
                  />
                ))}

                <ArchivedObjectsButton />
              </>
            ))}
        </div>
      </PageContent>
    </>
  );
}

/** Кнопка «Архивные объекты» (654:7558): серая, с иконкой архива, на
 * существующую страницу архива. */
function ArchivedObjectsButton(): JSX.Element {
  return (
    <div className="mx-6">
      <LinkButton
        href={ROUTES.propertyArchive}
        variant="secondary"
        size="large"
        fullWidth
        leftIcon={<Archive />}
      >
        Архивные объекты
      </LinkButton>
    </div>
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
      className="mx-6 flex cursor-pointer flex-col gap-3 rounded-card bg-surface-muted p-6 text-left outline-none transition-opacity hover:opacity-90 active:opacity-90 focus-visible:ring-4 focus-visible:ring-primary"
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

      {stacks.length === 0 ? (
        <div className="flex items-center gap-6">
          <span className="flex min-w-0 flex-1 flex-col gap-2">
            <span className="text-base leading-[18px] text-content-secondary">
              Вы еще не добавили ни одного платежа
            </span>
            <span className="text-sm font-medium leading-4 text-primary">
              Добавить
            </span>
          </span>
          <Image
            src="/images/payments/object-empty.png"
            alt=""
            width={64}
            height={64}
            className="h-16 w-16 shrink-0"
          />
        </div>
      ) : (
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
