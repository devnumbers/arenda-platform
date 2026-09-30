"use client";

import { useState, type JSX } from "react";
import { usePathname, useRouter } from "next/navigation";
import { Add, ArrowLeft, Search } from "@/shared/assets/icons";
import { ROUTES } from "@/shared/config/routes";
import { buildUrlWithParams } from "@/shared/lib/url-params";
import { dateToIsoLocal } from "@/shared/lib/calendar";
import { OPERATIONS_FEED_SORT, type PaymentOperationScope } from "@/shared/api/query-keys";
import {
  groupOperationsByDate,
  operationsCategoryChipLabel,
  operationsFiltersHref,
  operationsFiltersParams,
  operationsPeriodChipLabel,
  useOperationsFilters,
  usePropertyOperationsScopedPaged,
  usePropertyOperationsSummary,
} from "@/features/payments";
import { useProperty } from "@/features/properties";
import { propertyPermissions } from "@/entities/property";
import {
  Button,
  IconButton,
  InfiniteQueryTail,
  PageContent,
  TopNav,
  TopNavTitle,
} from "@/shared/ui/design";
import { PaymentsStateCard } from "./payments-sections";
import { OperationsDateFeedSkeleton, OperationsSummarySkeleton } from "./operations-skeletons";
import {
  OperationsDateList,
  OperationsNeverHad,
} from "./operations-list";
import { OperationsFilterChips } from "./operations-filter-chips";
import { OperationsPeriodPickerDialog } from "./operations-period-picker";
import { operationsFeedGate } from "../lib/operations-feed-gate";
import { summaryBarSegments } from "@/features/payment-categories";
import { OperationsSummaryCard } from "./operations-summary-card";

/**
 * Экран «Операции объекта» (#474, Figma 1492-41825): оплаченные операции
 * за период, сгруппированные по датам; сверху — чипы «Период» и
 * «Категория» («Все категории», синий с активным фильтром); под ними
 * карточки «Расходы»/«Доходы» с суммой за период; у обоих — полоса-разбивка
 * пилюлями категорий (все категории с операциями, зазор 2px,
 * пропорционально суммам — Figma 1510-77101, решение владельца 2026-09-01
 * отменяет схему #474 «топ-4 + остаток белым, доходы белым»). Карточки
 * ведут на экраны «Расходы объекта»/«Доходы объекта» (#475). Дефолт
 * периода — весь период (#674, карта #669, как в глобальной ленте #670):
 * без явного диапазона даты в запрос не уходят, чип «Период» нейтральный
 * (серый); выбор/сброс — пикер (пустой старт, «Сбросить»). Список сужается
 * категориями (#477), сводка категорийный фильтр не принимает — карточки
 * всегда за выбранный период. Фильтры живут в адресе (?from=&to=&category=
 * — шарабельно, назад возвращает к списку), пересчёт сводки при смене
 * периода — ключ react-query. Порции по 50 с бесконечным скроллом; строка
 * ведёт на страницу операции. Поиск — иконка в хедере (экран поиска —
 * тикет #476). «+» в хедере ведёт в визард одиночной операции без шага
 * объекта (#570/#571, макет 1492-41825; решение владельца 2026-09-08
 * отменяет решение #539 о скрытой «+») — только у того, кто может
 * создавать операции (CanEdit, не архив — контракт #569; зритель кнопки
 * не видит). Совсем пустой объект (all-time сводка без операций, #478)
 * вместо всего контента показывает «Операций еще не было» (Figma
 * 1518-92899) — без чипов, сводки, поиска и «+», но с CTA «Добавить
 * операцию» (#571).
 */
export function OperationsOfPropertyScreen({
  propertyId,
}: {
  readonly propertyId: string;
}): JSX.Element {
  const router = useRouter();
  const pathname = usePathname();
  const { filters } = useOperationsFilters();

  // Доступ к объекту — для кнопки создания (#571): общий мутационный
  // предикат (зритель — только чтение, архив read-only, контракт #569).
  const propertyQuery = useProperty(propertyId);
  const canMutate = propertyPermissions(
    propertyQuery.isSuccess ? propertyQuery.data : undefined,
  ).canEdit;

  const today = dateToIsoLocal(new Date());
  // Дефолт — весь период (#674): период в запросе только с явным выбором,
  // без него даты не уходят; пикер открывается пустым, «Сбросить»
  // возвращает к дефолту.
  const [periodOpen, setPeriodOpen] = useState(false);
  // Список сужается выбранными категориями; сводка (#473) категорийный
  // фильтр не принимает — карточки всегда за выбранный период.
  // Без применённого периода (#674) даты не уходят в запрос — весь период.
  // Лента платёжных фактов читается по paid_date (решение #933/#994).
  const periodScope: PaymentOperationScope = {
    status: "paid",
    order: "desc",
    sort: OPERATIONS_FEED_SORT,
    dateFrom: filters.period?.from,
    dateTo: filters.period?.to,
  };
  const listScope: PaymentOperationScope =
    filters.categories.length > 0
      ? { ...periodScope, categories: filters.categories }
      : periodScope;

  const listQuery = usePropertyOperationsScopedPaged(propertyId, listScope);
  const summaryQuery = usePropertyOperationsSummary(propertyId, periodScope);
  // All-time сводка (тот же контракт #473 без периода): отличает «операций
  // не было никогда» (#478, Figma 1518-92899) от пустого периода.
  const everQuery = usePropertyOperationsSummary(propertyId, {
    status: "paid",
    order: "desc",
    sort: OPERATIONS_FEED_SORT,
  });

  const groups = groupOperationsByDate(listQuery.data ?? [], today);
  // Скелетон / neverHad / разбивка категорий — общий каркас ленты (#478);
  // смена фильтров держит прежние данные (keepPreviousData), не скелетон.
  const { pending, neverHad, categoryRows } = operationsFeedGate(
    listQuery,
    summaryQuery,
    everQuery,
  );

  const openOperation = (operation: { readonly id: string }): void =>
    router.push(ROUTES.propertyOperation(propertyId, operation.id));

  const openCategories = (): void => {
    // Формат query — один хелпер с operationsFiltersHref (#472): знание
    // «как period/categories кодируются в адрес» живёт в одном модуле;
    // сборка адреса — тот же канон buildUrlWithParams (#792).
    const params = new URLSearchParams(operationsFiltersParams(filters));
    params.set("return", pathname);
    router.push(
      buildUrlWithParams(ROUTES.propertyOperationsCategories(propertyId), params),
    );
  };

  return (
    <>
      <TopNav
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            // Назад — на объект, а не по истории браузера: страница period
            // replace-ит список, и прежние периоды остаются в истории.
            onClick={() => router.push(ROUTES.property(propertyId))}
          />
        }
        trailing={
          // Совсем пустому объекту поиск не нужен (Figma 1518-92899 —
          // правая кнопка хедера скрыта), действие — CTA пустого
          // состояния; «+» (#571) — только у того, кто может создавать.
          neverHad ? undefined : (
            <>
              <IconButton
                icon={<Search />}
                label="Поиск операций"
                onClick={() =>
                  router.push(ROUTES.propertyOperationsSearch(propertyId))
                }
              />
              {canMutate && (
                <IconButton
                  icon={<Add />}
                  label="Добавить операцию"
                  onClick={() =>
                    router.push(ROUTES.propertyOperationsNew(propertyId))
                  }
                />
              )}
            </>
          )
        }
      >
        <TopNavTitle title="Операции объекта" />
      </TopNav>

      <PageContent>
        {neverHad ? (
          <OperationsNeverHad
            action={
              canMutate ? (
                <Button
                  onClick={() =>
                    router.push(ROUTES.propertyOperationsNew(propertyId))
                  }
                >
                  Добавить операцию
                </Button>
              ) : undefined
            }
          />
        ) : (
          <div className="flex flex-col gap-6 pt-4">
            {/* Чипы фильтров (Figma 1492:59532): период применён — синий,
             * дефолт «весь период» — нейтральный серый (#674); категории
             * подсвечиваются при активном фильтре. Выбор — отдельные
             * страницы (#477), состояние живёт в адресе. */}
            <OperationsFilterChips
              className="px-6"
              periodLabel={operationsPeriodChipLabel(filters.period)}
              periodActive={filters.period !== null}
              categoriesLabel={operationsCategoryChipLabel(
                filters.categories,
                categoryRows,
              )}
              categoriesActive={filters.categories.length > 0}
              onOpenPeriod={() => setPeriodOpen(true)}
              onOpenCategories={openCategories}
            />

            {pending && (
              <>
                {/* Паритет §7: копия контента — ряд карточек сводки и
                 * группы дат со строками; чипы выше — вне фазы загрузки. */}
                <OperationsSummarySkeleton cards={2} />
                <OperationsDateFeedSkeleton />
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
                     * полоса-разбивка пилюлями категорий у обоих направлений.
                     * Клик — вход на экран направления (#475). */}
                    <div className="flex gap-2 px-6">
                      <OperationsSummaryCard
                        label="Расходы"
                        totalKopecks={summaryQuery.data?.expenseTotalKopecks}
                        segments={summaryBarSegments(
                          summaryQuery.data,
                          "expense",
                        )}
                        openLabel="Открыть расходы объекта"
                        // Период и категории переживают переход на
                        // направление (решение владельца, #472).
                        onOpen={() =>
                          router.push(
                            operationsFiltersHref(
                              ROUTES.propertyOperationsExpense(propertyId),
                              filters,
                            ),
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
                        openLabel="Открыть доходы объекта"
                        onOpen={() =>
                          router.push(
                            operationsFiltersHref(
                              ROUTES.propertyOperationsIncome(propertyId),
                              filters,
                            ),
                          )
                        }
                      />
                    </div>

                    <OperationsDateList
                      groups={groups}
                      onSelectOperation={openOperation}
                      tail={<InfiniteQueryTail query={listQuery} />}
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
        <OperationsPeriodPickerDialog onClose={() => setPeriodOpen(false)} />
      )}
    </>
  );
}
