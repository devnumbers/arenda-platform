"use client";

import { useState, type JSX } from "react";
import { useRouter } from "next/navigation";
import { Search } from "@/shared/assets/icons";
import { ROUTES } from "@/shared/config/routes";
import { clientTodayIso } from "@/entities/payment";
import { useKeyboardActivation } from "@/shared/lib/hooks/useKeyboardActivation";
import { useInfiniteScroll } from "@/shared/lib/hooks/useInfiniteScroll";
import {
  defaultOperationsPeriod,
  globalOperationsFiltersParams,
  groupOperationsByDate,
  operationsCategoryChipLabel,
  operationsCategoryRows,
  operationsPeriodDefaultChipLabel,
  operationsPeriodRangeChipLabel,
  operationsPropertyChipLabel,
  useGlobalOperationsFilters,
  useGlobalOperationsPaged,
  useGlobalOperationsSummary,
} from "@/features/payments";
import { Button, PageContent } from "@/shared/ui/design";
import { PageHeader } from "@/shared/ui/page-header";
import { PaymentsSkeleton, PaymentsStateCard } from "./payments-sections";
import {
  LoadingMoreIndicator,
  OperationsDateList,
  OperationsNeverHad,
} from "./operations-list";
import { OperationsFilterChips } from "./operations-filter-chips";
import { OperationsGlobalPeriodPickerDialog } from "./operations-period-picker";
import { OperationsSummaryCard } from "./operations-summary-card";
import { hasNoPaidOperationsEver } from "../lib/operations-empty-states";
import { summaryBarSegments } from "../lib/summary-bar";

/**
 * Экран «Операции» — глобальная лента по всем объектам (#541, макеты
 * 1726-90017/1733-26973): платёжные факты видимой книги (свои объекты плюс
 * объекты с активным членством, архив сервером исключён — контракт #540),
 * сгруппированные по датам; у строки — подзаголовок-объект. Сверху — чипы
 * «Период» (дефолт — текущий месяц), «Объект» («Все объекты»/«1 объект»/
 * «N объектов» — ведёт на мультивыбор #542) и «Категория» (#544); карточки
 * «Расходы»/«Доходы» ведут на страницы направления (#548 — отменяет
 * решение #539 о некликабельных карточках); полоса
 * разбивки пилюлями категорий та же, что на объектном экране; сводка
 * категорийный фильтр не принимает, объектный и период — принимают.
 * Фильтры живут в адресе (?from=&to=&property=&category= — шарабельно).
 * Порции по 50 с бесконечным скроллом; строка ведёт на страницу операции
 * своего объекта. Поиск — пилюля «Найти операцию» (#543), кнопка «+»
 * скрыта (решение владельца #539). Совсем пустая книга (all-time сводка
 * выбранного скоупа без операций) вместо контента — «Операций еще не
 * было», как на объектном экране (#478). Вход — пункт бокового меню
 * кабинета, только ПК (решение владельца #539).
 */
export function OperationsGlobalScreen(): JSX.Element {
  const router = useRouter();
  const { filters } = useGlobalOperationsFilters();

  const today = clientTodayIso();
  const period = filters.period ?? defaultOperationsPeriod(today);
  // Пикер периода — канонический оверлей поверх списка.
  const [periodOpen, setPeriodOpen] = useState(false);
  // Список сужается выбранными категориями; сводка (#540) категорийный
  // фильтр не принимает — карточки показывают объекты и период целиком.
  const periodScope = {
    order: "desc" as const,
    propertyIds: filters.propertyIds,
    dateFrom: period.from,
    dateTo: period.to,
  };
  const listScope =
    filters.categories.length > 0
      ? { ...periodScope, categories: filters.categories }
      : periodScope;

  const listQuery = useGlobalOperationsPaged(listScope);
  const summaryQuery = useGlobalOperationsSummary(periodScope);
  // All-time сводка выбранного скоупа объектов (без периода/категорий):
  // отличает «операций не было никогда» (#478) от пустого периода/фильтра.
  const everQuery = useGlobalOperationsSummary({
    order: "desc",
    propertyIds: filters.propertyIds,
  });

  const sentinelRef = useInfiniteScroll(() => {
    if (listQuery.hasNextPage && !listQuery.isFetchingNextPage) {
      void listQuery.fetchNextPage();
    }
  }, listQuery.hasNextPage === true);

  const groups = groupOperationsByDate(listQuery.data ?? [], today);
  // Скелетон — только пока данных нет вовсе (первая загрузка); ошибка без
  // данных показывает карточку повтора, не скелетон.
  const pending =
    (listQuery.data === undefined ||
      summaryQuery.data === undefined ||
      everQuery.data === undefined) &&
    !listQuery.isError &&
    !summaryQuery.isError &&
    !everQuery.isError;
  const neverHad =
    !listQuery.isError && hasNoPaidOperationsEver(everQuery.data);
  const categoryRows = operationsCategoryRows(
    summaryQuery.data?.categories ?? [],
  );

  const openOperation = (operation: {
    readonly propertyId: string;
    readonly id: string;
  }): void =>
    router.push(ROUTES.propertyOperation(operation.propertyId, operation.id));

  const filterHref = (base: string, extra?: Record<string, string>): string => {
    const params = new URLSearchParams(globalOperationsFiltersParams(filters));
    for (const [name, value] of Object.entries(extra ?? {})) {
      params.set(name, value);
    }
    const query = params.toString();
    return query.length > 0 ? `${base}?${query}` : base;
  };

  return (
    <>
      <PageHeader title="Операции" />

      <PageContent>
        {neverHad ? (
          <OperationsNeverHad />
        ) : (
          <div className="-mx-5 flex min-[1200px]:mx-0 flex-col gap-6 px-6 pt-1">
            {/* Ритм страницы — ровно 24px по бокам (решение владельца
             * 2026-09-05): контент кабинета даёт 20px до 1200px и 0 после,
             * страница выравнивает себя до 24 сама и прижимает все элементы
             * (пилюля, чипы, карточки, лента) к этому краю без своих вставок. */}
            {/* Пилюля поиска (#543) — кнопка на отдельную страницу; «+» скрыта
             * (решение владельца #539: создания разовой операции вне правила
             * нет). */}
            <OperationsSearchPill
              onOpenSearch={() =>
                router.push(filterHref(ROUTES.operationsSearch))
              }
            />

            <OperationsFilterChips
              periodLabel={
                filters.period !== null
                  ? operationsPeriodRangeChipLabel(period)
                  : operationsPeriodDefaultChipLabel(period)
              }
              propertyLabel={operationsPropertyChipLabel(filters.propertyIds)}
              propertyActive={filters.propertyIds.length > 0}
              categoriesLabel={operationsCategoryChipLabel(
                filters.categories,
                categoryRows,
              )}
              categoriesActive={filters.categories.length > 0}
              onOpenPeriod={() => setPeriodOpen(true)}
              onOpenProperties={() =>
                router.push(
                  filterHref(ROUTES.operationsObjects, {
                    return: ROUTES.operations,
                  }),
                )
              }
              onOpenCategories={() =>
                router.push(
                  filterHref(ROUTES.operationsCategories, {
                    return: ROUTES.operations,
                  }),
                )
              }
            />

            {pending && (
              <>
                <PaymentsSkeleton withHeading />
                <PaymentsSkeleton withHeading />
              </>
            )}

            {!pending && (
              <>
                {listQuery.isError ? (
                  <PaymentsStateCard
                    title="Не удалось загрузить операции"
                    hint="Проверьте подключение и попробуйте еще раз"
                    action={
                      <Button
                        variant="secondary"
                        size="small"
                        onClick={() => void listQuery.refetch()}
                      >
                        Повторить
                      </Button>
                    }
                  />
                ) : (
                  <>
                    {/* Карточки сводки (Figma 1510-77101): сумма за период +
                     * полоса-разбивка пилюлями категорий. Тап ведёт на
                     * страницу направления (#548 — отменяет решение #539
                     * о некликабельных карточках). */}
                    <div className="flex gap-2">
                      <OperationsSummaryCard
                        label="Расходы"
                        totalKopecks={summaryQuery.data?.expenseTotalKopecks}
                        segments={summaryBarSegments(
                          summaryQuery.data,
                          "expense",
                        )}
                        openLabel="Открыть все расходы"
                        onOpen={() =>
                          router.push(
                            filterHref(ROUTES.operationsExpenses, {
                              return: ROUTES.operations,
                            }),
                          )
                        }
                      />
                      <OperationsSummaryCard
                        label="Доходы"
                        totalKopecks={summaryQuery.data?.incomeTotalKopecks}
                        segments={summaryBarSegments(
                          summaryQuery.data,
                          "income",
                        )}
                        openLabel="Открыть все доходы"
                        onOpen={() =>
                          router.push(
                            filterHref(ROUTES.operationsIncomes, {
                              return: ROUTES.operations,
                            }),
                          )
                        }
                      />
                    </div>

                    <OperationsDateList
                      groups={groups}
                      onSelectOperation={openOperation}
                      inset={false}
                      renderSubtitle={(operation) => operation.propertyName}
                      tail={
                        <>
                          {listQuery.hasNextPage === true && (
                            <div ref={sentinelRef} aria-hidden />
                          )}
                          {listQuery.isFetchingNextPage && (
                            <LoadingMoreIndicator />
                          )}
                        </>
                      }
                    />
                  </>
                )}
              </>
            )}
          </div>
        )}
      </PageContent>

      {/* Пикер периода — рендер только в открытом состоянии: лента и
          черновик живут, пока смонтирован. */}
      {periodOpen && (
        <OperationsGlobalPeriodPickerDialog
          onClose={() => setPeriodOpen(false)}
        />
      )}
    </>
  );
}

/** Пилюля поиска (макет 1726-90017): серый rounded-pill на всю ширину,
 * лупа и подпись «Найти операцию»; тап открывает страницу поиска #543.
 * Кнопка «+» из макета скрыта (решение владельца #539). */
function OperationsSearchPill({
  onOpenSearch,
}: {
  readonly onOpenSearch: () => void;
}): JSX.Element {
  const activatorProps = useKeyboardActivation({ onSelect: onOpenSearch });

  return (
    <div
      {...activatorProps}
      className="flex h-14 w-full cursor-pointer items-center rounded-pill bg-surface-muted pl-[18px] text-left outline-none transition-opacity hover:opacity-90 focus-visible:ring-4 focus-visible:ring-primary active:opacity-90"
    >
      <Search className="h-6 w-6 shrink-0 text-content" aria-hidden />
      <span className="min-w-0 flex-1 truncate px-2 text-base font-medium text-content">
        Найти операцию
      </span>
    </div>
  );
}
