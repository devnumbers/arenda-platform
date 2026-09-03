'use client';

import type { JSX } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { ArrowLeft, ArrowSLeft, ArrowSRight, Search } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { useInfiniteScroll } from '@/shared/lib/hooks/useInfiniteScroll';
import type { PaymentOperationScope } from '@/shared/api/query-keys';
import { Button, IconButton, PageContent, TopNav, TopNavTitle } from '@/shared/ui/design';
import {
  clientTodayIso,
  type PaymentOperation,
} from '@/entities/payment';
import {
  defaultOperationsPeriod,
  groupOperationsByDate,
  operationsCategoryChipLabel,
  operationsCategoryRows,
  operationsFiltersHref,
  operationsPeriodDefaultChipLabel,
  operationsPeriodRangeChipLabel,
  shiftOperationsPeriod,
  useOperationsFilters,
  usePropertyOperationsScopedPaged,
  usePropertyOperationsSummary,
} from '@/features/payments';
import { operationsTypeHeadline } from '../lib/operations-empty-states';
import { PaymentsSkeleton, PaymentsStateCard } from './payments-sections';
import { LoadingMoreIndicator, OperationsDateList } from './operations-list';
import { OperationsFilterChips } from './operations-filter-chips';

/** Копия экрана по направлению (Figma 1494-61191 / 1492-59865). */
const SCREEN_COPY = {
  income: { title: 'Доходы объекта' },
  expense: { title: 'Расходы объекта' },
} as const;

/**
 * Экран «Доходы объекта» / «Расходы объекта» (#475, Figma 1494-61191 и
 * 1492-59865): те же чипы, что на главном экране операций, вместо двух
 * карточек — H1 направления за период («Нет доходов»/«Нет трат» при
 * пустом периоде, #478) с круглыми стрелками листания
 * по месяцам (ArrowSLeft/ArrowSRight 44×44); ниже — список операций одного
 * типа за период, группировка и строки как на главном (общий
 * OperationsDateList). Скоуп сужен `type` — фильтр списка и сводки #473.
 * Период и категории живут в адресе (#477): дефолт — текущий месяц,
 * стрелки сдвигают применённый период (правая гасится, когда период
 * упёрся в текущий месяц — на экранах только paid-операции, резолюция
 * #474), выбор чипами — шиты #477. Порции по 50 с бесконечным скроллом.
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
  const { filters, applyPeriod } = useOperationsFilters();

  const today = clientTodayIso();
  const period = filters.period ?? defaultOperationsPeriod(today);
  // Правая стрелка гасится, когда следующее окно уходит в будущее (экраны
  // операций — только paid, резолюция #474): период листается на свою же
  // длину (решение владельца, #472).
  const nextIsFuture = shiftOperationsPeriod(period, 1).from > today;

  const periodScope: PaymentOperationScope = {
    status: 'paid',
    order: 'desc',
    type,
    dateFrom: period.from,
    dateTo: period.to,
  };
  const listScope: PaymentOperationScope =
    filters.categories.length > 0
      ? { ...periodScope, categories: filters.categories }
      : periodScope;

  const listQuery = usePropertyOperationsScopedPaged(propertyId, listScope);
  const summaryQuery = usePropertyOperationsSummary(propertyId, periodScope);

  const sentinelRef = useInfiniteScroll(
    () => {
      if (listQuery.hasNextPage && !listQuery.isFetchingNextPage) {
        void listQuery.fetchNextPage();
      }
    },
    listQuery.hasNextPage === true,
  );

  const groups = groupOperationsByDate(listQuery.data ?? [], today);
  const totalKopecks =
    summaryQuery.data === undefined
      ? undefined
      : type === 'expense'
        ? summaryQuery.data.expenseTotalKopecks
        : summaryQuery.data.incomeTotalKopecks;
  // Скелетон — только пока данных нет вовсе (первая загрузка): смена
  // периода держит прежние данные (keepPreviousData) и не дёргает
  // страницу; ошибка без данных показывает карточку повтора, не скелетон.
  const pending =
    (listQuery.data === undefined || summaryQuery.data === undefined)
    && !listQuery.isError
    && !summaryQuery.isError;
  const categoryRows = operationsCategoryRows(summaryQuery.data?.categories ?? []);

  const openOperation = (operation: PaymentOperation): void =>
    router.push(ROUTES.propertyOperation(propertyId, operation.id));

  const openFilters = (which: 'period' | 'categories'): void => {
    const params = new URLSearchParams();
    if (filters.period !== null) {
      params.set('from', filters.period.from);
      params.set('to', filters.period.to);
    }
    if (filters.categories.length > 0) {
      params.set('category', filters.categories.join(','));
    }
    params.set('return', pathname);
    const base =
      which === 'period'
        ? ROUTES.propertyOperationsPeriod(propertyId)
        : ROUTES.propertyOperationsCategories(propertyId);
    router.push(`${base}?${params.toString()}`);
  };

  return (
    <>
      <TopNav
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            // Назад — на главный список операций с текущими фильтрами
            // (период и категории переживают переход — решение владельца,
            // #472), а не по истории браузера.
            onClick={() =>
              router.push(operationsFiltersHref(ROUTES.propertyOperations(propertyId), filters))
            }
          />
        }
        trailing={
          <IconButton
            icon={<Search />}
            label="Поиск операций"
            onClick={() => router.push(ROUTES.propertyOperationsSearch(propertyId))}
          />
        }
      >
        <TopNavTitle title={SCREEN_COPY[type].title} />
      </TopNav>

      <PageContent>
        <div className="flex flex-col gap-6 pt-4">
          {/* Чипы фильтров — как на главном (Figma 1502:65149): период
           * следует за листанием; выбор — отдельные страницы (#477). */}
          <OperationsFilterChips
            periodLabel={
              filters.period !== null
                ? operationsPeriodRangeChipLabel(period)
                : operationsPeriodDefaultChipLabel(period)
            }
            categoriesLabel={operationsCategoryChipLabel(filters.categories, categoryRows)}
            categoriesActive={filters.categories.length > 0}
            onOpenPeriod={() => openFilters('period')}
            onOpenCategories={() => openFilters('categories')}
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
                  {/* Сумма периода с листанием по месяцам (Figma 1502:65151):
                   * стрелки 44×44, H1 28/32 по центру; период применяется
                   * в адрес (#477), дефолт — текущий месяц. */}
                  <div className="flex items-stretch px-3.5">
                    <IconButton
                      icon={<ArrowSLeft />}
                      label="Предыдущий месяц"
                      onClick={() => applyPeriod(shiftOperationsPeriod(period, -1))}
                    />
                    <div className="flex min-w-0 flex-1 items-center justify-center">
                      <span className="truncate text-[28px] font-semibold leading-8 text-content">
                        {/* Пустой период — «Нет доходов»/«Нет трат» вместо
                         * «0 ₽» (Figma 1510-76177, 1510-75650, #478). */}
                        {operationsTypeHeadline(type, totalKopecks)}
                      </span>
                    </div>
                    <IconButton
                      icon={<ArrowSRight />}
                      label="Следующий месяц"
                      disabled={nextIsFuture}
                      onClick={() => applyPeriod(shiftOperationsPeriod(period, 1))}
                    />
                  </div>

                  <OperationsDateList
                    groups={groups}
                    onSelectOperation={openOperation}
                    tail={
                      <>
                        {listQuery.hasNextPage === true && <div ref={sentinelRef} aria-hidden />}
                        {listQuery.isFetchingNextPage && <LoadingMoreIndicator />}
                      </>
                    }
                  />
                </>
              )}
            </>
          )}
        </div>
      </PageContent>
    </>
  );
}
