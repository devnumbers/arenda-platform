"use client";

import { useState, type JSX } from "react";
import { useRouter } from "next/navigation";
import { Add } from "@/shared/assets/icons";
import { ROUTES } from "@/shared/config/routes";
import { OPERATIONS_FEED_SORT } from "@/shared/api/query-keys";
import { dateToIsoLocal } from "@/shared/lib/calendar";
import { propertyPermissions } from "@/entities/property";
import {
  globalOperationsFiltersHref,
  groupOperationsByDate,
  operationsCategoryChipLabel,
  operationsPeriodChipLabel,
  operationsPropertyChipLabel,
  useGlobalOperationsFilters,
  useGlobalOperationsPaged,
  useGlobalOperationsSummary,
} from "@/features/payments";
import { useProperties } from "@/features/properties";
import { HubCollapseAnchor, HubTitle, IconButton, InfiniteQueryTail, Button, PageContent, SearchPill, TopNav } from "@/shared/ui/design";
import { PaymentsStateCard } from "./payments-sections";
import { OperationsDateFeedSkeleton, OperationsSummarySkeleton } from "./operations-skeletons";
import {
  OperationsDateList,
  OperationsNeverHad,
} from "./operations-list";
import { OperationsFilterChips } from "./operations-filter-chips";
import { OperationsGlobalPeriodPickerDialog } from "./operations-period-picker";
import { OperationsSummaryCard } from "./operations-summary-card";
import { operationsFeedGate } from "../lib/operations-feed-gate";
import { summaryBarSegments } from "@/features/payment-categories";

/**
 * Экран «Операции» — глобальная лента по всем объектам (#541, макеты
 * 1726-90017/1733-26973): платёжные факты видимой книги (свои объекты плюс
 * объекты с активным членством, архив сервером исключён — контракт #540),
 * сгруппированные по датам; у строки — подзаголовок-объект. Сверху — чипы
 * «Период» (дефолт — весь период #670: нейтральный серый чип, даты в
 * запрос не уходят; с явным диапазоном — синий с датами), «Объект»
 * («Все объекты»/«1 объект»/
 * «N объектов» — ведёт на мультивыбор #542) и «Категория» (#544); карточки
 * «Расходы»/«Доходы» ведут на страницы направления (#548 — отменяет
 * решение #539 о некликабельных карточках); полоса
 * разбивки пилюлями категорий та же, что на объектном экране; сводка
 * категорийный фильтр не принимает, объектный и период — принимают.
 * Фильтры живут в адресе (?from=&to=&property=&category= — шарабельно).
 * Порции по 50 с бесконечным скроллом; строка ведёт на страницу операции
 * своего объекта. Поиск — пилюля «Найти операцию» (#543); «+» в хаб-шапке
 * ведёт в визард одиночной операции (#570; решение владельца 2026-09-08
 * #571 отменяет решение #539 о скрытой «+»). Совсем пустая книга (all-time
 * сводка выбранного скоупа без операций) вместо контента — «Операций еще
 * не было» с CTA «Добавить операцию» (#571), как на объектном экране
 * (#478). Вход — «Операции» в сайдбаре ПК и в шите «Еще» на мобайле и
 * планшете (единый хром, #564).
 */
export function OperationsGlobalScreen(): JSX.Element {
  const router = useRouter();
  const { filters } = useGlobalOperationsFilters();
  // «+» и CTA пустой книги — только когда есть хоть один объект с правом
  // правки (#703): у чистого зрителя визард ведёт в чужой объект и упирается
  // в отказ сервера. Пока справочник не загружен — консервативно скрыты.
  const propertiesQuery = useProperties();
  const canCreateSomewhere = (propertiesQuery.data ?? []).some((property) =>
    propertyPermissions(property).canEdit,
  );

  const today = dateToIsoLocal(new Date());
  // Пикер периода — канонический оверлей поверх списка.
  const [periodOpen, setPeriodOpen] = useState(false);
  // Список и сводка (#540) сужаются выбранными категориями — карточки
  // зеркалят отфильтрованный список (решение владельца 01.10, прежнее
  // «сводка категорийный фильтр не принимает» отменено). Без применённого
  // периода (#670) даты не уходят в запрос — весь период.
  // Лента платёжных фактов читается по paid_date (решение #933/#994).
  const periodScope = {
    order: "desc" as const,
    sort: OPERATIONS_FEED_SORT,
    propertyIds: filters.propertyIds,
    dateFrom: filters.period?.from,
    dateTo: filters.period?.to,
    includeArchived: filters.archived,
  };
  const listScope =
    filters.categories.length > 0
      ? { ...periodScope, categories: filters.categories }
      : periodScope;

  const listQuery = useGlobalOperationsPaged(listScope);
  const summaryQuery = useGlobalOperationsSummary(listScope);
  // All-time сводка выбранного скоупа объектов (без периода/категорий):
  // отличает «операций не было никогда» (#478) от пустого периода/фильтра.
  const everQuery = useGlobalOperationsSummary({
    order: "desc",
    sort: OPERATIONS_FEED_SORT,
    propertyIds: filters.propertyIds,
    includeArchived: filters.archived,
  });

  const groups = groupOperationsByDate(listQuery.data ?? [], today);
  // Скелетон / neverHad / разбивка категорий — общий каркас ленты (#478).
  const { pending, neverHad, categoryRows } = operationsFeedGate(
    listQuery,
    summaryQuery,
    everQuery,
  );

  const openOperation = (operation: {
    readonly propertyId: string;
    readonly id: string;
  }): void =>
    router.push(ROUTES.propertyOperation(operation.propertyId, operation.id));

  // «+» — в визард одиночной операции (#571, решение владельца 2026-09-08):
  // в ряду заголовка хаба и в правом слоте компакт-бара; в neverHad
  // скрыта — действие там CTA пустого состояния.
  const addButton = (
    <IconButton
      icon={<Add />}
      label="Добавить операцию"
      onClick={() => router.push(ROUTES.operationsNew)}
    />
  );
  const showAdd = !neverHad && canCreateSomewhere;

  return (
    <>
      {/* Хаб-шапка: «крылья» (лого + профиль) и на мобайле, поведение
       * стандартное — в потоке на мобайле, закреплена на планшете и ПК.
       * «+» — в визард одиночной операции (#571, решение владельца
       * 2026-09-08); в neverHad вместе с поиском скрыта, действие —
       * CTA пустого состояния. */}
      <TopNav
        mobileWings
        collapse={{
          title: 'Операции',
          search: { href: ROUTES.operationsSearch, label: 'Найти операцию' },
          trailing: showAdd ? addButton : undefined,
        }}
      />

      <PageContent>
        <HubCollapseAnchor>
          {/* Строка заголовка h-8 (тикет #865): топ заголовка — ровно 24
              от хедера (96), как у хабов без кнопки; кнопка 44 переполняет
              строку симметрично — центрирована против линии заголовка
              (макет 1733-27411), без +6px items-center от её высоты. */}
          <div className="flex h-8 items-center justify-between pr-3.5">
            <HubTitle>Операции</HubTitle>
            {showAdd && addButton}
          </div>
          {!neverHad && (
            <div className="mt-4 px-6">
              {/* Пилюля поиска (#543) — кнопка на отдельную страницу. */}
              <OperationsSearchPill
                onOpenSearch={() =>
                  router.push(
                    globalOperationsFiltersHref(ROUTES.operationsSearch, filters),
                  )
                }
              />
            </div>
          )}
        </HubCollapseAnchor>

        {neverHad ? (
          <OperationsNeverHad
            action={
              canCreateSomewhere ? (
                <Button onClick={() => router.push(ROUTES.operationsNew)}>
                  Добавить операцию
                </Button>
              ) : undefined
            }
          />
        ) : (
          /* От пилюли до чипов 24 (макет 3226-74599: пилюля 72–128, чипы
           * со 152) — дальше ритм gap-6 до конца первого блока. */
          <div className="mt-6 flex flex-col gap-6">
            <OperationsFilterChips
              className="px-6"
              periodLabel={operationsPeriodChipLabel(filters.period)}
              periodActive={filters.period !== null}
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
                  globalOperationsFiltersHref(ROUTES.operationsObjects, filters, {
                    return: ROUTES.operations,
                  }),
                )
              }
              onOpenCategories={() =>
                router.push(
                  globalOperationsFiltersHref(ROUTES.operationsCategories, filters, {
                    return: ROUTES.operations,
                  }),
                )
              }
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
                     * полоса-разбивка пилюлями категорий. Тап ведёт на
                     * страницу направления (#548 — отменяет решение #539
                     * о некликабельных карточках). */}
                    <div className="flex gap-2 px-6">
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
                            globalOperationsFiltersHref(ROUTES.operationsExpenses, filters, {
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
                            globalOperationsFiltersHref(ROUTES.operationsIncomes, filters, {
                              return: ROUTES.operations,
                            }),
                          )
                        }
                      />
                    </div>

                    <OperationsDateList
                      groups={groups}
                      onSelectOperation={openOperation}
                      renderSubtitle={(operation) => operation.propertyName}
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
        <OperationsGlobalPeriodPickerDialog
          onClose={() => setPeriodOpen(false)}
        />
      )}
    </>
  );
}

/** Пилюля поиска операций (макет 1726-90017) — адаптер канона SearchPill
 * с подписью «Найти операцию»; тап открывает страницу поиска #543.
 * Кнопка «+» рядом с пилюлей переехала в шапку страницы (решение
 * владельца 2026-09-08 #571 — отмена решения #539). Экспорт для
 * route-loading (#609). */
export function OperationsSearchPill({
  onOpenSearch,
}: {
  readonly onOpenSearch: () => void;
}): JSX.Element {
  return <SearchPill onOpenSearch={onOpenSearch} label="Найти операцию" />;
}
