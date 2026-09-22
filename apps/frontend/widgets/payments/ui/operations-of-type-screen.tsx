"use client";

import { useState, type JSX } from "react";
import { usePathname, useRouter } from "next/navigation";
import { Add, ArrowLeft, Search } from "@/shared/assets/icons";
import { ROUTES } from "@/shared/config/routes";
import { buildUrlWithParams } from "@/shared/lib/url-params";
import type { PaymentOperationScope } from "@/shared/api/query-keys";
import { summaryBarSegments } from "@/features/payment-categories";
import {
  Button,
  IconButton,
  InfiniteQueryTail,
  PageContent,
  TopNav,
  TopNavTitle,
} from "@/shared/ui/design";
import { clientTodayIso, type PaymentOperation } from "@/entities/payment";
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
import { operationsFeedGate } from "../lib/operations-feed-gate";
import { PaymentsStateCard } from "./payments-sections";
import {
  OperationsDateList,
  OperationsNeverHad,
} from "./operations-list";
import { OperationsFilterChips } from "./operations-filter-chips";
import { OperationsPeriodPickerDialog } from "./operations-period-picker";
import { OperationsSummaryCard } from "./operations-summary-card";
import {
  OperationsDateFeedSkeleton,
  OperationsSummarySkeleton,
} from "./operations-skeletons";

/** Копия экрана по направлению (Figma 1863-35008 / 1863-35816): title —
 * шапка, label — карточка-сводка направления. */
const SCREEN_COPY = {
  income: { title: "Доходы объекта", label: "Доходы" },
  expense: { title: "Расходы объекта", label: "Расходы" },
} as const;

/**
 * Экран «Доходы объекта» / «Расходы объекта» (#475): список операций
 * одного типа за период, группировка и строки как на главном (общий
 * OperationsDateList). Скоуп сужен `type` — фильтр списка и сводки #473.
 * Логика — канон глобальных направлений #548 (решение владельца #679,
 * Figma 1863-35008/1863-35816): листание периода стрелками с H1-суммой
 * снесено — период управляется только чипом «Период», вместо H1 —
 * некликабельная карточка-сводка направления с полосой разбивки.
 * Дефолт периода — весь период (#675, карта #669): без явного выбора
 * даты в запрос не уходят, чип «Период» нейтральный; пикер — объектный
 * (пустой старт, «Сбросить»). Шапка — «Назад» на главный список
 * операций с текущими фильтрами (#472), лупа объектного поиска и «+»
 * в визард с пресетом направления (?type=) — только у того, кто может
 * создавать операции (propertyPermissions, контракт #569, как на главном
 * #674). Совсем пустой объект (all-time сводка без операций, #478)
 * вместо контента показывает «Операций еще не было» с CTA — лупа и «+»
 * скрыты (конвенция зоны #478/#571); пустой период/категория при живой
 * книге — обычная пустая лента. Подзаголовка имени объекта в строках
 * нет — свой объект (в отличие от глобальных направлений). Порции по 50
 * с бесконечным скроллом.
 */
export function OperationsOfTypeScreen({
  propertyId,
  type,
}: {
  readonly propertyId: string;
  readonly type: keyof typeof SCREEN_COPY;
}): JSX.Element {
  const router = useRouter();
  const pathname = usePathname();
  const { filters } = useOperationsFilters();

  // Доступ к объекту — для «+» (#679): общий мутационный предикат
  // (зритель — только чтение, архив read-only, контракт #569).
  const propertyQuery = useProperty(propertyId);
  const canMutate = propertyPermissions(
    propertyQuery.isSuccess ? propertyQuery.data : undefined,
  ).canEdit;

  const today = clientTodayIso();
  // Пикер периода — канонический оверлей поверх списка (решение владельца
  // 2026-09-04, раньше — отдельный маршрут /operations/period).
  const [periodOpen, setPeriodOpen] = useState(false);

  const periodScope: PaymentOperationScope = {
    status: "paid",
    order: "desc",
    type,
    dateFrom: filters.period?.from,
    dateTo: filters.period?.to,
  };
  const listScope: PaymentOperationScope =
    filters.categories.length > 0
      ? { ...periodScope, categories: filters.categories }
      : periodScope;

  const listQuery = usePropertyOperationsScopedPaged(propertyId, listScope);
  const summaryQuery = usePropertyOperationsSummary(propertyId, periodScope);
  // All-time сводка объекта (тот же контракт #473 без периода): отличает
  // «операций не было никогда» (#478) от пустого периода/категории.
  const everQuery = usePropertyOperationsSummary(propertyId, {
    status: "paid",
    order: "desc",
  });

  const groups = groupOperationsByDate(listQuery.data ?? [], today);
  // Скелетон / neverHad / разбивка категорий — общий каркас ленты (#478);
  // смена периода держит прежние данные (keepPreviousData), не скелетон.
  const { pending, neverHad, categoryRows } = operationsFeedGate(
    listQuery,
    summaryQuery,
    everQuery,
  );

  const openOperation = (operation: PaymentOperation): void =>
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

  // Визард с пресетами объект+направление от точки входа (#679): с
  // «Доходов объекта» — Доход, с «Расходов» — Расход (маршрут распознаёт
  // ?type=).
  const newOperationHref = `${ROUTES.propertyOperationsNew(propertyId)}?type=${type}`;

  return (
    <>
      {/* Шапка направления — «Назад» на главный список (фильтры переживают
       * переход, #472), лупа объектного поиска и «+» с пресетом
       * направления (#679, как на глобальных #571); в neverHad-состоянии
       * иконки скрыты — конвенция зоны (#478), действие — CTA пустого
       * состояния. */}
      <TopNav
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            // Назад — на главный список операций с текущими фильтрами
            // (период и категории переживают переход — решение владельца,
            // #472), а не по истории браузера.
            onClick={() =>
              router.push(
                operationsFiltersHref(
                  ROUTES.propertyOperations(propertyId),
                  filters,
                ),
              )
            }
          />
        }
        trailing={
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
                  onClick={() => router.push(newOperationHref)}
                />
              )}
            </>
          )
        }
      >
        <TopNavTitle title={SCREEN_COPY[type].title} />
      </TopNav>

      <PageContent>
        {neverHad ? (
          <OperationsNeverHad
            action={
              canMutate ? (
                <Button onClick={() => router.push(newOperationHref)}>
                  Добавить операцию
                </Button>
              ) : undefined
            }
          />
        ) : (
          <div className="flex flex-col gap-6 pt-4">
            {/* Чипы фильтров — как на главном (Figma 1502:65149): период
             * применён — синий, дефолт «весь период» — нейтральный серый
             * (#675); выбор — отдельные страницы (#477). */}
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
                {/* Паритет §7: одна карточка направления во всю ширину и
                 * группы дат со строками — как у глобальных направлений
                 * (#548); чипы выше — вне фазы загрузки. */}
                <OperationsSummarySkeleton cards={1} />
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
                    {/* Карточка направления (Figma 1863-35008/1863-35816):
                     * некликабельна, полоса разбивки та же — вход на
                     * направление только с главного экрана операций. */}
                    <div className="px-6">
                      <OperationsSummaryCard
                        label={SCREEN_COPY[type].label}
                        totalKopecks={
                          type === "expense"
                            ? summaryQuery.data?.expenseTotalKopecks
                            : summaryQuery.data?.incomeTotalKopecks
                        }
                        segments={summaryBarSegments(summaryQuery.data, type)}
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
