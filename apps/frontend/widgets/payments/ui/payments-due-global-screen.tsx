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
  groupDueByBucket,
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
  dueListInOrder,
  serializeDueSortToParams,
  type DueSort,
} from "../lib/payments-due-model";
import {
  PaymentsHeading,
  PaymentsRowsSkeleton,
  PaymentsStateCard,
} from "./payments-sections";

/**
 * Страница «Платежи на оплату» (тикет #1347; макеты 1036:40828 список,
 * 1041:46762 пустое; вход — секция «На оплату» главного экрана). Строки
 * окна напоминаний #1346 сгруппированы в бакеты канона due-buckets —
 * «Сегодня / Завтра / Через 3 дня / Через 7 дней» (пустых групп нет);
 * направление внутри бакета — чип «Сначала новые» (дефолт, серверный
 * порядок dueDate desc) / «Сначала старые», живёт в query строки (?sort=old,
 * дефолт не пишется — конвенция состояния в адресе) и переживает
 * перезагрузку. Строка канона PaymentRowButton: иконка категории,
 * название, объект, сумма и плановая дата; тап ведёт на страницу правила
 * (сегмент read-only — оплата на объектных экранах). Пустое состояние —
 * канон EmptyState с 3D-кошельком; чип сортировки с пустым списком
 * прячется (§7).
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
  const groups = groupDueByBucket(
    dueListInOrder(dueQuery.data?.items ?? [], sort),
  );
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
                {sortChip}
                <div className="flex flex-col gap-2">
                  {groups.map((group) => (
                    <section
                      key={group.bucket}
                      data-testid={`payments-due-group-${group.bucket}`}
                      className="flex flex-col"
                    >
                      <PaymentsHeading>{group.label}</PaymentsHeading>
                      {group.items.map((item) => (
                        <DueRow
                          key={item.operationId}
                          item={item}
                          onSelect={() =>
                            router.push(
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
        imageSrc="/images/payments/due-empty.png"
        title="Нет платежей на оплату"
        description="Когда вам придет уведомление об оплате, платеж появится здесь"
        descriptionClassName="text-content"
      />
    </div>
  );
}
