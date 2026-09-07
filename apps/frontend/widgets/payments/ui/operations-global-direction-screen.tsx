'use client';

import { useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { Search } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { useInfiniteScroll } from '@/shared/lib/hooks/useInfiniteScroll';
import { clientTodayIso, type PaymentType } from '@/entities/payment';
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
} from '@/features/payments';
import { Button, IconButton, PageContent } from '@/shared/ui/design';
import { PageHeader } from '@/shared/ui/page-header';
import { PaymentsSkeleton, PaymentsStateCard } from './payments-sections';
import { LoadingMoreIndicator, OperationsDateList, OperationsNeverHad } from './operations-list';
import { OperationsFilterChips } from './operations-filter-chips';
import { OperationsGlobalPeriodPickerDialog } from './operations-period-picker';
import { OperationsSummaryCard } from './operations-summary-card';
import { hasNoPaidOperationsEver } from '../lib/operations-empty-states';
import { summaryBarSegments } from '../lib/summary-bar';
import {
  globalDirectionListScope,
  globalDirectionSummaryScope,
} from '../lib/operations-global-direction-model';

export type OperationsGlobalDirectionScreenProps = {
  readonly type: PaymentType;
};

/**
 * Страница направления глобальной ленты (#548, Figma 1858-104152): все
 * доходы или все расходы выбранного скоупа за период. Вход — карточки
 * сводки на главной (отменяет решение #539 о некликабельных карточках).
 * Фильтры — те же глобальные, живут в адресе: чипы периода/объекта/
 * категории открывают общие пикеры и выборщики (#542/#544) с ?return=
 * обратно на страницу; период — канонический range-пикер (не месячный чип
 * макета — консистентность зоны важнее буквы макета, решение владельца
 * 2026-09-07). Одна карточка направления — сводка с `type=` (контракт
 * #540), некликабельна, полоса разбивки та же. Лента — контракт `type`
 * списка /operations, порции по 50 с бесконечным скроллом; строка ведёт
 * на страницу операции своего объекта. Совсем пустая книга направления —
 * «Операций еще не было», как на главной (#478). В шапке — лупа на
 * существующий поиск #543 (без типа направления — решение владельца);
 * «+» из макета не делаем (решение #539). Каркас кабинетный (решение
 * #542), ритм 24px всей страницей (#541).
 */
export function OperationsGlobalDirectionScreen({
  type,
}: OperationsGlobalDirectionScreenProps): JSX.Element {
  const router = useRouter();
  const { filters } = useGlobalOperationsFilters();

  const title = type === 'income' ? 'Доходы' : 'Расходы';
  const selfRoute = type === 'income' ? ROUTES.operationsIncomes : ROUTES.operationsExpenses;

  const today = clientTodayIso();
  const period = filters.period ?? defaultOperationsPeriod(today);
  // Пикер периода — канонический оверлей поверх списка.
  const [periodOpen, setPeriodOpen] = useState(false);

  const listQuery = useGlobalOperationsPaged(
    globalDirectionListScope(period, filters.propertyIds, filters.categories, type),
  );
  const summaryQuery = useGlobalOperationsSummary(
    globalDirectionSummaryScope(period, filters.propertyIds, type),
  );
  // All-time сводка направления (без периода/категорий): отличает «операций
  // направления не было никогда» (#478) от пустого периода/фильтра.
  const everQuery = useGlobalOperationsSummary({
    order: 'desc',
    propertyIds: filters.propertyIds,
    type,
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
  const neverHad = !listQuery.isError && hasNoPaidOperationsEver(everQuery.data);
  const categoryRows = operationsCategoryRows(summaryQuery.data?.categories ?? []);

  const openOperation = (operation: {
    readonly propertyId: string;
    readonly id: string;
  }): void => router.push(ROUTES.propertyOperation(operation.propertyId, operation.id));

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
      {/* Шапка направления: «назад» на ленту (filterHref сохраняет фильтры)
       * и лупа на существующий поиск (#543) с текущими фильтрами; в
       * neverHad-состоянии лупа скрыта — конвенция зоны (#478). */}
      <PageHeader
        title={title}
        backHref={filterHref(ROUTES.operations)}
        actions={
          neverHad ? undefined : (
            <IconButton
              icon={<Search />}
              label="Найти операцию"
              onClick={() => router.push(filterHref(ROUTES.operationsSearch))}
            />
          )
        }
      />

      <PageContent>
        {neverHad ? (
          <OperationsNeverHad />
        ) : (
          <div className="-mx-5 flex min-[1200px]:mx-0 flex-col gap-6 px-6 pt-1">
            {/* Ритм страницы — ровно 24px по бокам (#541): чипы, карточка
             * и лента прижаты к краю контента без своих вставок. */}
            <OperationsFilterChips
              periodLabel={
                filters.period !== null
                  ? operationsPeriodRangeChipLabel(period)
                  : operationsPeriodDefaultChipLabel(period)
              }
              propertyLabel={operationsPropertyChipLabel(filters.propertyIds)}
              propertyActive={filters.propertyIds.length > 0}
              categoriesLabel={operationsCategoryChipLabel(filters.categories, categoryRows)}
              categoriesActive={filters.categories.length > 0}
              onOpenPeriod={() => setPeriodOpen(true)}
              onOpenProperties={() =>
                router.push(filterHref(ROUTES.operationsObjects, { return: selfRoute }))
              }
              onOpenCategories={() =>
                router.push(filterHref(ROUTES.operationsCategories, { return: selfRoute }))
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
                    {/* Карточка направления (Figma 1858-104152): одна, из
                     * сводки с type=, некликабельна — вход на направление
                     * только с главной (#548). */}
                    <OperationsSummaryCard
                      label={title}
                      totalKopecks={
                        type === 'expense'
                          ? summaryQuery.data?.expenseTotalKopecks
                          : summaryQuery.data?.incomeTotalKopecks
                      }
                      segments={summaryBarSegments(summaryQuery.data, type)}
                    />

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
