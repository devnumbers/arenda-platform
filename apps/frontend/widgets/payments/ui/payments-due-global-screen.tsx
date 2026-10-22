"use client";

import type { JSX } from "react";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { ArrowLeft, ChangeVertical } from "@/shared/assets/icons";
import { ROUTES } from "@/shared/config/routes";
import { usePaymentsDue } from "@/features/payments";
import { CategoryIcon, categoryStyle } from "@/features/payment-categories";
import {
  formatDayMonth,
  PaymentRowButton,
} from "@/entities/payment";
import type { PaymentDueItem } from "@/entities/payment";
import {
  Button,
  ChipButton,
  EmptyState,
  IconButton,
  PageContent,
  TopNav,
  TopNavTitle,
} from "@/shared/ui/design";
import { goBack } from "@/shared/lib/navigation";
import { useUrlParams } from "@/shared/lib/hooks/use-url-params";
import {
  DUE_SORT_PARAMS,
  dueGroupsInOrder,
  serializeDueSortToParams,
  type DueSort,
} from "../lib/payments-due-model";
import {
  PaymentsHeading,
  PaymentsRowsSkeleton,
  PaymentsStateCard,
} from "./payments-sections";
import { pushPage } from "@/shared/lib/navigation";

/**
 * Страница «Платежи на оплату» (тикет #1347; макеты 1036:40828 список,
 * 1041:46762 пустое; вход — секция «На оплату» главного экрана). Строки
 * окна напоминаний #1346 сгруппированы по РЕАЛЬНЫМ дням — «Сегодня /
 * Завтра / Через 2 дня / … / Через 7 дней» (ревизия владельца 13.10:
 * склейка дат под одним заголовком читалась как ошибка; пустых групп нет).
 * Чип «Сначала новые» (дефолт — дни по возрастанию) / «Сначала старые»
 * разворачивает порядок групп ЦЕЛИКОМ — внутри дня порядок серверный;
 * состояние живёт в query строки (?sort=old, дефолт не пишется — конвенция
 * состояния в адресе) и переживает перезагрузку; с одной группой чипу
 * нечего переворачивать — он прячется вместе с пустым списком (§7). Строка
 * канона PaymentRowButton: иконка категории, название, объект, сумма и
 * плановая дата; тап ведёт на страницу правила (сегмент read-only — оплата
 * на объектных экранах). Пустое состояние — канон EmptyState с
 * 3D-кошельком.
 */
export function PaymentsDueGlobalScreen({
  initialSort = "new",
}: {
  readonly initialSort?: DueSort;
}): JSX.Element {
  const router = useRouter();
  const { write } = useUrlParams();
  const [sort, setSort] = useState<DueSort>(initialSort);
  const dueQuery = usePaymentsDue();

  // Скелетон — пока данных нет вовсе; ошибка без данных — карточка
  // повтора (канон состояний, как на соседних страницах карты).
  const pending = dueQuery.data === undefined && !dueQuery.isError;
  const groups = dueGroupsInOrder(dueQuery.data?.items ?? [], sort);
  const listReady = !pending && !dueQuery.isError;

  // Смена направления синхронно переписывает query строки — канон
  // useUrlParams (#786), дефолтное «Сначала новые» не пишется.
  const toggleSort = (): void => {
    const next: DueSort = sort === "new" ? "old" : "new";
    setSort(next);
    write(serializeDueSortToParams(next), { own: DUE_SORT_PARAMS });
  };

  const sortChip = (
    <div className="px-6 pb-3">
      <ChipButton
        data-testid="payments-due-sort-chip"
        trailingIcon={<ChangeVertical />}
        onClick={toggleSort}
      >
        {sort === "new" ? "Сначала новые" : "Сначала старые"}
      </ChipButton>
    </div>
  );

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
      >
        <TopNavTitle title="Платежи на оплату" />
      </TopNav>

      <PageContent>
        <div data-testid="payments-due-screen" className="flex flex-col">
          {pending && (
            <>
              {/* Паритет §7: чип реальный — вне фазы загрузки; строки
               * канона PaymentRowButton. */}
              {sortChip}
              <PaymentsRowsSkeleton />
            </>
          )}

          {dueQuery.isError && (
            <PaymentsStateCard
              title="Не удалось загрузить платежи на оплату"
              hint="Проверьте подключение и попробуйте еще раз"
              action={
                <Button
                  variant="secondary"
                  size="small"
                  onClick={() => void dueQuery.refetch()}
                >
                  Повторить
                </Button>
              }
            />
          )}

          {listReady && groups.length === 0 && <PaymentsDueEmpty />}

          {listReady &&
            groups.length > 0 && (
              <div className="flex flex-col">
                {groups.length > 1 && sortChip}
                <div className="flex flex-col gap-2">
                  {groups.map((group) => (
                    <section
                      key={group.daysLeft}
                      data-testid={`payments-due-group-day${group.daysLeft}`}
                      className="flex flex-col"
                    >
                      <PaymentsHeading>{group.label}</PaymentsHeading>
                      {group.items.map((item) => (
                        <DueRow
                          key={item.operationId}
                          item={item}
                          onSelect={() =>
                            pushPage(router,
                              ROUTES.propertyPayment(
                                item.propertyId,
                                item.paymentId,
                              ),
                            )
                          }
                        />
                      ))}
                    </section>
                  ))}
                </div>
              </div>
            )}
        </div>
      </PageContent>
    </>
  );
}

/** Строка «На оплату» (1036:40828): иконка категории, название, объект,
 * сумма и плановая дата правым нижним полем. Звезда избранного в слоте
 * подзаголовка не рисуется — контракта строки #1346 флаг избранного не
 * несёт (сегмент read-only, звезда ничего не тогглит). */
function DueRow({
  item,
  onSelect,
}: {
  readonly item: PaymentDueItem;
  readonly onSelect: () => void;
}): JSX.Element {
  const style = categoryStyle(item.category.source, item.category.slug);
  return (
    <PaymentRowButton
      className="px-6"
      categoryIcon={<CategoryIcon icon={style.icon} color={style.color} />}
      title={item.title}
      subtitle={item.propertyName}
      amountKopecks={item.amountKopecks}
      description={formatDayMonth(item.dueDate)}
      onSelect={onSelect}
    />
  );
}

/** Пустое состояние (1041:46762): 3D-кошелёк, «Нет платежей на оплату» и
 * подсказка про уведомление. Центрируется в доступной высоте, как пустое
 * «Просроченных». */
function PaymentsDueEmpty(): JSX.Element {
  return (
    <div
      data-testid="payments-due-empty"
      className="flex min-h-[calc(100dvh-216px)] flex-1 items-center justify-center"
    >
      <EmptyState
        imageSrc="/images/payments/due-empty.webp"
        title="Нет платежей на оплату"
        description="Когда вам придет уведомление об оплате, платеж появится здесь"
        descriptionClassName="text-content"
      />
    </div>
  );
}
