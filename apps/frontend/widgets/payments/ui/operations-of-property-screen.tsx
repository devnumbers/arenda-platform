'use client';

import { useState, type JSX } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { ArrowLeft, Search } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { clientTodayIso } from '@/entities/payment';
import { useInfiniteScroll } from '@/shared/lib/hooks/useInfiniteScroll';
import type { PaymentOperationScope } from '@/shared/api/query-keys';
import {
  defaultOperationsPeriod,
  groupOperationsByDate,
  operationsCategoryChipLabel,
  operationsCategoryRows,
  operationsFiltersHref,
  operationsPeriodDefaultChipLabel,
  operationsPeriodRangeChipLabel,
  useOperationsFilters,
  usePropertyOperationsScopedPaged,
  usePropertyOperationsSummary,
} from '@/features/payments';
import {
  Button,
  IconButton,
  PageContent,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import {
  PaymentsSkeleton,
  PaymentsStateCard,
} from './payments-sections';
import {
  LoadingMoreIndicator,
  OperationsDateList,
  OperationsNeverHad,
} from './operations-list';
import { OperationsFilterChips } from './operations-filter-chips';
import { OperationsPeriodPickerDialog } from './operations-period-picker';
import { hasNoPaidOperationsEver } from '../lib/operations-empty-states';
import { summaryBarSegments } from '../lib/summary-bar';
import { OperationsSummaryCard } from './operations-summary-card';

/**
 * Экран «Операции объекта» (#474, Figma 1492-41825): оплаченные операции
 * за период, сгруппированные по датам; сверху — чипы «Период» (выбран,
 * синий; дефолт — текущий месяц) и «Категория» («Все категории», синий с
 * активным фильтром); под ними карточки «Расходы»/«Доходы» с суммой за
 * период; у обоих — полоса-разбивка пилюлями категорий (все категории с
 * операциями, зазор 2px, пропорционально суммам — Figma 1510-77101,
 * решение владельца 2026-09-01 отменяет схему #474 «топ-4 + остаток
 * белым, доходы белым»). Карточки ведут на экраны «Расходы
 * объекта»/«Доходы объекта» (#475). Список сужается категориями (#477),
 * сводка категорийный фильтр не принимает — карточки всегда за весь
 * период, а строки шита категорий показывают суммы периода. Фильтры живут
 * в адресе (?from=&to=&category= — шарабельно, назад возвращает к
 * списку), пересчёт сводки при смене периода — ключ react-query. Порции
 * по 50 с бесконечным скроллом; строка ведёт на страницу операции. Поиск —
 * иконка в хедере (экран поиска — тикет #476). Совсем пустой объект
 * (all-time сводка без операций, #478) вместо всего контента показывает
 * «Операций еще не было» (Figma 1518-92899) — без чипов, сводки и поиска.
 */
export function OperationsOfPropertyScreen({
  propertyId,
}: {
  readonly propertyId: string;
}): JSX.Element {
  const router = useRouter();
  const pathname = usePathname();
  const { filters } = useOperationsFilters();

  const today = clientTodayIso();
  const period = filters.period ?? defaultOperationsPeriod(today);
  // Пикер периода — канонический оверлей поверх списка (решение владельца
  // 2026-09-04, раньше — отдельный маршрут /operations/period): состояние
  // живёт в адресе, черновик — в пикере.
  const [periodOpen, setPeriodOpen] = useState(false);
  // Список сужается выбранными категориями; сводка (#473) категорийный
  // фильтр не принимает — карточки всегда показывают весь период.
  const periodScope: PaymentOperationScope = {
    status: 'paid',
    order: 'desc',
    dateFrom: period.from,
    dateTo: period.to,
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
    status: 'paid',
    order: 'desc',
  });

  const sentinelRef = useInfiniteScroll(
    () => {
      if (listQuery.hasNextPage && !listQuery.isFetchingNextPage) {
        void listQuery.fetchNextPage();
      }
    },
    listQuery.hasNextPage === true,
  );

  const groups = groupOperationsByDate(listQuery.data ?? [], today);
  // Скелетон — только пока данных нет вовсе (первая загрузка): смена
  // фильтров держит прежние данные (keepPreviousData) и не дёргает
  // страницу; ошибка без данных показывает карточку повтора, не скелетон.
  const pending =
    (listQuery.data === undefined
      || summaryQuery.data === undefined
      || everQuery.data === undefined)
    && !listQuery.isError
    && !summaryQuery.isError
    && !everQuery.isError;
  const neverHad = !listQuery.isError && hasNoPaidOperationsEver(everQuery.data);
  const categoryRows = operationsCategoryRows(summaryQuery.data?.categories ?? []);

  const openOperation = (operation: { readonly id: string }): void =>
    router.push(ROUTES.propertyOperation(propertyId, operation.id));

  const openCategories = (): void => {
    const params = new URLSearchParams();
    if (filters.period !== null) {
      params.set('from', filters.period.from);
      params.set('to', filters.period.to);
    }
    if (filters.categories.length > 0) {
      params.set('category', filters.categories.join(','));
    }
    params.set('return', pathname);
    router.push(`${ROUTES.propertyOperationsCategories(propertyId)}?${params.toString()}`);
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
          // правая кнопка хедера скрыта).
          neverHad ? undefined : (
            <IconButton
              icon={<Search />}
              label="Поиск операций"
              onClick={() => router.push(ROUTES.propertyOperationsSearch(propertyId))}
            />
          )
        }
      >
        <TopNavTitle title="Операции объекта" />
      </TopNav>

      <PageContent>
        {neverHad ? (
          <OperationsNeverHad />
        ) : (
          <div className="flex flex-col gap-6 pt-4">
            {/* Чипы фильтров (Figma 1492:59532): период выбран — синий; категории
             * подсвечиваются при активном фильтре. Выбор — отдельные страницы
             * (#477), состояние живёт в адресе. */}
            <OperationsFilterChips
              periodLabel={
                filters.period !== null
                  ? operationsPeriodRangeChipLabel(period)
                  : operationsPeriodDefaultChipLabel(period)
              }
              categoriesLabel={operationsCategoryChipLabel(filters.categories, categoryRows)}
              categoriesActive={filters.categories.length > 0}
              onOpenPeriod={() => setPeriodOpen(true)}
              onOpenCategories={openCategories}
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
                     * полоса-разбивка пилюлями категорий у обоих направлений.
                     * Клик — вход на экран направления (#475). */}
                    <div className="flex gap-2 px-6">
                      <OperationsSummaryCard
                        label="Расходы"
                        totalKopecks={summaryQuery.data?.expenseTotalKopecks}
                        segments={summaryBarSegments(summaryQuery.data, 'expense')}
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
                        segments={summaryBarSegments(summaryQuery.data, 'income')}
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
        )}
      </PageContent>

      {/* Пикер периода — рендер только в открытом состоянии: лента и
          черновик живут, пока смонтирован. */}
      {periodOpen && <OperationsPeriodPickerDialog onClose={() => setPeriodOpen(false)} />}
    </>
  );
}
