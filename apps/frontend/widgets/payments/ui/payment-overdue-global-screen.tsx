"use client";

import type { JSX } from "react";
import { useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { ArrowLeft, ChangeVertical, Star } from "@/shared/assets/icons";
import { ROUTES } from "@/shared/config/routes";
import { useGlobalPayments } from "@/features/payments";
import { CategoryIcon, categoryStyle } from "@/features/payment-categories";
import type { GlobalPayment } from "@/entities/payment";
import { PaymentRowButton } from "@/entities/payment";
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
import { globalOverdueList, type OverdueSort } from "../lib/overdue-global-model";
import { overdueDaysLine } from "../lib/payments-global-model";
import { PaymentsRowsSkeleton, PaymentsStateCard } from "./payments-sections";

/**
 * Экран «Просроченные операции» (карта #573, тикет #580; вход — карточка
 * «Все просроченные» главного экрана #578). Список (706:14684): строки
 * канона PaymentRowButton — иконка категории с красным (!)-бейджем,
 * название, объект; сумма обычным цветом, срок «N дней» красным
 * (склонение — formatOverdueDays). У платежей в избранном — звезда-
 * индикатор рядом с названием объекта (как на «Избранных» 693:5546);
 * управление избранным — только со страницы платежа, звезда ничего не
 * тогглит. Сортировка по возрасту просрочки — чип «Новые ⇅ / Старые ⇅»
 * (706:15029), дефолт «Старые» (решение владельца 09.09); выбор живёт в
 * query строки (?sort=new, дефолт не пишется — конвенция книги
 * контактов) и переживает перезагрузку. Тап строке — СТАРЕЙШАЯ
 * просроченная операция правила
 * (тот же принцип, что у карточек главного экрана #578); страница только
 * читающая: оплата и закрытие просрочки — на объектных экранах. Пустое
 * состояние (885:18755) — канон EmptyState с 3D-иллюстрацией.
 */
export function PaymentOverdueGlobalScreen({
  initialSort = "old",
}: {
  readonly initialSort?: OverdueSort;
}): JSX.Element {
  const router = useRouter();
  const pathname = usePathname();
  const [sort, setSort] = useState<OverdueSort>(initialSort);
  const feedQuery = useGlobalPayments();

  // Скелетон — пока данных нет вовсе; ошибка без данных — карточка
  // повтора (канон состояний, как на соседних страницах карты).
  const pending = feedQuery.data === undefined && !feedQuery.isError;
  const overdue = globalOverdueList(feedQuery.data?.items ?? [], sort);
  const listReady = !pending && !feedQuery.isError;

  // Смена направления синхронно переписывает query строки (дефолтное
  // «Старые» не пишется — как в книге контактов).
  const toggleSort = (): void => {
    const next: OverdueSort = sort === "old" ? "new" : "old";
    setSort(next);
    const query = next === "new" ? "?sort=new" : "";
    router.replace(query !== "" ? `${pathname}${query}` : pathname, {
      scroll: false,
    });
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
      >
        <TopNavTitle title="Просроченные операции" />
      </TopNav>

      <PageContent>
        <div data-testid="payments-overdue-screen" className="flex flex-col">
          {pending && (
            <>
              {/* Паритет §7: чип сортировки реальный — вне фазы загрузки
               * (в непустой книге он стоит над списком, тап до данных лишь
               * меняет направление); строки канона PaymentRowButton —
               * иконка, название, объект, сумма и срок. */}
              <div className="px-6 pb-3">
                <ChipButton
                  data-testid="overdue-sort-chip"
                  trailingIcon={<ChangeVertical />}
                  onClick={toggleSort}
                >
                  {sort === "old" ? "Старые" : "Новые"}
                </ChipButton>
              </div>
              <PaymentsRowsSkeleton />
            </>
          )}

          {feedQuery.isError && (
            <PaymentsStateCard
              title="Не удалось загрузить просроченные"
              hint="Проверьте подключение и попробуйте еще раз"
              action={
                <Button
                  variant="secondary"
                  size="small"
                  onClick={() => void feedQuery.refetch()}
                >
                  Повторить
                </Button>
              }
            />
          )}

          {listReady && overdue.length === 0 && <OverdueEmpty />}

          {listReady && overdue.length > 0 && (
            <>
              <div className="px-6 pb-3">
                <ChipButton
                  data-testid="overdue-sort-chip"
                  trailingIcon={<ChangeVertical />}
                  onClick={toggleSort}
                >
                  {sort === "old" ? "Старые" : "Новые"}
                </ChipButton>
              </div>
              {overdue.map((payment) => (
                <PaymentRowButton
                  key={payment.id}
                  className="px-6"
                  categoryIcon={<OverdueRowIcon payment={payment} />}
                  title={payment.title}
                  subtitle={payment.propertyName}
                  subtitleSuffix={
                    payment.isFavorite ? (
                      <Star className="h-4 w-4" aria-hidden />
                    ) : undefined
                  }
                  amountKopecks={payment.amountKopecks}
                  description={
                    // Срок — единственный красный элемент строки (решение
                    // владельца 09.09): сумма обычным цветом, danger на
                    // строку не вешается.
                    <span className="text-sm font-medium text-danger">
                      {overdueDaysLine(payment)}
                    </span>
                  }
                  onSelect={() =>
                    payment.oldestOverdueOperationId !== null
                      ? router.push(
                          ROUTES.propertyOperation(
                            payment.propertyId,
                            payment.oldestOverdueOperationId,
                          ),
                        )
                      : router.push(
                          ROUTES.propertyPayment(payment.propertyId, payment.id),
                        )
                  }
                />
              ))}
            </>
          )}
        </div>
      </PageContent>
    </>
  );
}

/** Иконка категории строки просрочки: красный (!)-бейдж — накопленная
 * просрочка (как в секции главного экрана, статус-icon danger). */
function OverdueRowIcon({
  payment,
}: {
  readonly payment: GlobalPayment;
}): JSX.Element {
  const style = categoryStyle(payment.category.source, payment.category.slug);
  return (
    <CategoryIcon icon={style.icon} color={style.color} badge="danger" />
  );
}

/** Пустое состояние (885:18755): иллюстрация overdue-empty.png 128 и две
 * тёмные подписи — заголовок и подсказка. Центрируется в доступной
 * высоте, как пустое состояние избранных. */
function OverdueEmpty(): JSX.Element {
  return (
    <div
      data-testid="payments-overdue-empty"
      className="flex min-h-[calc(100dvh-216px)] flex-1 items-center justify-center"
    >
      <EmptyState
        imageSrc="/images/payments/overdue-empty.png"
        title="У вас нет просроченных операций"
        description="Здесь будут операции, которые не успели отметить вовремя"
        descriptionClassName="text-content"
      />
    </div>
  );
}
