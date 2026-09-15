'use client';

import { useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { Add, ArrowLeft, Search } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { clientTodayIso, type PaymentType } from '@/entities/payment';
import {
  globalOperationsFiltersParams,
  groupOperationsByDate,
  operationsCategoryChipLabel,
  operationsPeriodChipLabel,
  operationsPropertyChipLabel,
  useGlobalOperationsFilters,
  useGlobalOperationsPaged,
  useGlobalOperationsSummary,
} from '@/features/payments';
import {
  Button,
  IconButton,
  InfiniteQueryTail,
  PageContent,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { PaymentsStateCard } from './payments-sections';
import { OperationsDateFeedSkeleton, OperationsSummarySkeleton } from './operations-skeletons';
import { OperationsDateList, OperationsNeverHad } from './operations-list';
import { OperationsFilterChips } from './operations-filter-chips';
import { OperationsGlobalPeriodPickerDialog } from './operations-period-picker';
import { OperationsSummaryCard } from './operations-summary-card';
import { operationsFeedGate } from '../lib/operations-feed-gate';
import { summaryBarSegments } from '@/features/payment-categories';
import {
  globalDirectionListScope,
  globalDirectionSummaryScope,
} from '../lib/operations-global-direction-model';

export type OperationsGlobalDirectionScreenProps = {
  readonly type: PaymentType;
};

/**
 * Страница направления глобальной ленты (#548, Figma 1858-104152): все
 * доходы или все расходы выбранного скоупа; дефолт периода — весь период
 * (#671, как на главной #670): без явного диапазона даты в запрос не
 * уходят, чип «Период» нейтральный. Вход — карточки
 * сводки на главной (отменяет решение #539 о некликабельных карточках).
 * Фильтры — те же глобальные, живут в адресе: чипы периода/объекта/
 * категории открывают общие пикеры и выборщики (#542/#544) с ?return=
 * обратно на страницу; период — канонический range-пикер (не месячный чип
 * макета — консистентность зоны важнее буквы макета, решение владельца
 * 2026-09-07). Одна карточка направления — сводка с `type=` (контракт
 * #540), некликабельна, полоса разбивки та же. Лента — контракт `type`
 * списка /operations, порции по 50 с бесконечным скроллом; строка ведёт
 * на страницу операции своего объекта. Совсем пустая книга направления —
 * «Операций еще не было» с CTA «Добавить операцию» (#571), как на главной
 * (#478). В шапке — лупа на существующий поиск #543 (без типа направления
 * — решение владельца) и «+» в визард с пресетом направления (макет
 * 1858-104152; решение владельца 2026-09-08 #571 отменяет решение #539).
 * Единый хром экранов (#564): шапка — канон подэкрана (TopNav с «Назад»
 * на ленту, лупой и «+» в trailing).
 */
export function OperationsGlobalDirectionScreen({
  type,
}: OperationsGlobalDirectionScreenProps): JSX.Element {
  const router = useRouter();
  const { filters } = useGlobalOperationsFilters();

  const title = type === 'income' ? 'Доходы' : 'Расходы';
  const selfRoute = type === 'income' ? ROUTES.operationsIncomes : ROUTES.operationsExpenses;

  const today = clientTodayIso();
  // Дефолт направления — весь период (#671): период в запросе только с
  // явным выбором, без него даты не уходят; пикер открывается пустым,
  // «Сбросить» возвращает к дефолту.
  const [periodOpen, setPeriodOpen] = useState(false);

  const listQuery = useGlobalOperationsPaged({
    ...globalDirectionListScope(filters.period, filters.propertyIds, filters.categories, type),
    includeArchived: filters.archived,
  });
  const summaryQuery = useGlobalOperationsSummary({
    ...globalDirectionSummaryScope(filters.period, filters.propertyIds, type),
    includeArchived: filters.archived,
  });
  // All-time сводка направления (без периода/категорий): отличает «операций
  // направления не было никогда» (#478) от пустого периода/фильтра.
  const everQuery = useGlobalOperationsSummary({
    order: 'desc',
    propertyIds: filters.propertyIds,
    type,
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
  }): void => router.push(ROUTES.propertyOperation(operation.propertyId, operation.id));

  // Визард с пресетом направления от точки входа (#571): с «Доходов» —
  // Доход, с «Расходов» — Расход (маршрут распознаёт ?type=).
  const newOperationHref = `${ROUTES.operationsNew}?type=${type}`;

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
      {/* Шапка направления — канон подэкрана: «Назад» на ленту (goBack
       * сохраняет фильтры прямой загрузки), лупа на существующий поиск
       * (#543) с текущими фильтрами и «+» в визард с пресетом направления
       * (#571, решение владельца 2026-09-08 — отмена решения #539); в
       * neverHad-состоянии иконки скрыты — конвенция зоны (#478),
       * действие — CTA пустого состояния. */}
      <TopNav
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            onClick={() => goBack(router, filterHref(ROUTES.operations))}
          />
        }
        trailing={
          neverHad ? undefined : (
            <>
              <IconButton
                icon={<Search />}
                label="Найти операцию"
                onClick={() => router.push(filterHref(ROUTES.operationsSearch))}
              />
              <IconButton
                icon={<Add />}
                label="Добавить операцию"
                onClick={() => router.push(newOperationHref)}
              />
            </>
          )
        }
      >
        <TopNavTitle title={title} />
      </TopNav>

      <PageContent>
        {neverHad ? (
          <OperationsNeverHad
            action={
              <Button onClick={() => router.push(newOperationHref)}>
                Добавить операцию
              </Button>
            }
          />
        ) : (
          <div className="flex flex-col gap-6 pt-4">
            <OperationsFilterChips
              className="px-6"
              periodLabel={operationsPeriodChipLabel(filters.period)}
              periodActive={filters.period !== null}
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
                {/* Паритет §7: одна карточка направления во всю ширину и
                 * группы дат со строками; чипы выше — вне фазы загрузки. */}
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
                    {/* Карточка направления (Figma 1858-104152): одна, из
                     * сводки с type=, некликабельна — вход на направление
                     * только с главной (#548). */}
                    <div className="px-6">
                      <OperationsSummaryCard
                        label={title}
                        totalKopecks={
                          type === 'expense'
                            ? summaryQuery.data?.expenseTotalKopecks
                            : summaryQuery.data?.incomeTotalKopecks
                        }
                        segments={summaryBarSegments(summaryQuery.data, type)}
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
