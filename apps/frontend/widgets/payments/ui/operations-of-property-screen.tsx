'use client';

import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft, Search } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { useInfiniteScroll } from '@/shared/lib/hooks/useInfiniteScroll';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import type { PaymentOperationScope } from '@/shared/api/query-keys';
import {
  defaultOperationsPeriod,
  groupOperationsByDate,
  operationsCategoryRows,
  useOperationsFilters,
  usePropertyOperationsScopedPaged,
  usePropertyOperationsSummary,
} from '@/features/payments';
import { clientTodayIso } from '@/entities/payment';
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
import { LoadingMoreIndicator, OperationsDateList } from './operations-list';
import { OperationsFiltersArea } from './operations-filters-area';
import { summaryBarSegments, type SummaryBarSegment } from '../lib/summary-bar';

/** Пилюля полосы без операций в периоде (Figma 1510-77101): серая #D3D7D9. */
const EMPTY_BAR_COLOR = '#D3D7D9';

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
 * иконка в хедере (экран поиска — тикет #476).
 */
export function OperationsOfPropertyScreen({
  propertyId,
}: {
  readonly propertyId: string;
}): JSX.Element {
  const router = useRouter();
  const { filters, applyPeriod, applyCategories } = useOperationsFilters();

  const today = clientTodayIso();
  const period = filters.period ?? defaultOperationsPeriod(today);
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

  const sentinelRef = useInfiniteScroll(
    () => {
      if (listQuery.hasNextPage && !listQuery.isFetchingNextPage) {
        void listQuery.fetchNextPage();
      }
    },
    listQuery.hasNextPage === true,
  );

  const groups = groupOperationsByDate(listQuery.data ?? [], today);
  const pending = listQuery.isPending || summaryQuery.isPending;
  const categoryRows = operationsCategoryRows(summaryQuery.data?.categories ?? []);

  const openOperation = (operation: { readonly id: string }): void =>
    router.push(ROUTES.propertyOperation(propertyId, operation.id));

  return (
    <>
      <TopNav
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            onClick={() => goBack(router, ROUTES.property(propertyId))}
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
        <TopNavTitle title="Операции объекта" />
      </TopNav>

      <PageContent>
        <div className="flex flex-col gap-6 pt-4">
          {/* Чипы и шиты фильтров период/категории (#477): дефолт — текущий
           * месяц и «Все категории», состояние живёт в адресе. */}
          <OperationsFiltersArea
            period={period}
            periodExplicit={filters.period !== null}
            categories={filters.categories}
            categoryRows={categoryRows}
            onApplyPeriod={applyPeriod}
            onApplyCategories={applyCategories}
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
                    <SummaryCard
                      label="Расходы"
                      totalKopecks={summaryQuery.data?.expenseTotalKopecks}
                      segments={summaryBarSegments(summaryQuery.data, 'expense')}
                      openLabel="Открыть расходы объекта"
                      onOpen={() => router.push(ROUTES.propertyOperationsExpense(propertyId))}
                    />
                    <SummaryCard
                      label="Доходы"
                      totalKopecks={summaryQuery.data?.incomeTotalKopecks}
                      segments={summaryBarSegments(summaryQuery.data, 'income')}
                      openLabel="Открыть доходы объекта"
                      onOpen={() => router.push(ROUTES.propertyOperationsIncome(propertyId))}
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

/** Карточка сводки (Figma 1510-77101, EL-3091ce92): серая карточка radius 24,
 * сумма 16/500, подпись 14/400, полоса 6px из «пилюль» категорий — по одной
 * на категорию, зазор 2px держит раздельно даже совпадающие цвета каталога,
 * ширина пропорциональна сумме (flex-grow по весам). Без операций в периоде —
 * единственная серая пилюля #D3D7D9. Вся карточка — кнопка на экран
 * направления (#475), как заголовки секций «Платежей объекта». */
function SummaryCard({
  label,
  totalKopecks,
  segments,
  openLabel,
  onOpen,
}: {
  readonly label: string;
  readonly totalKopecks: number | undefined;
  readonly segments: ReadonlyArray<SummaryBarSegment>;
  readonly openLabel: string;
  readonly onOpen: () => void;
}): JSX.Element {
  return (
    <button
      type="button"
      onClick={onOpen}
      aria-label={openLabel}
      className="min-w-0 flex-1 cursor-pointer rounded-card bg-surface-muted px-6 pb-6 pt-5 text-left outline-none transition-opacity hover:opacity-80 active:opacity-80 focus-visible:ring-4 focus-visible:ring-primary"
    >
      <div className="flex flex-col gap-0.5">
        <span className="text-base font-medium text-content">
          {totalKopecks === undefined ? '—' : formatMoneyKopecks(totalKopecks)}
        </span>
        <span className="text-sm text-content">{label}</span>
      </div>
      <div className="mt-4 flex h-1.5 w-full gap-[2px]">
        {(segments.length > 0 ? segments : [{ color: EMPTY_BAR_COLOR, weight: 1 }]).map(
          (segment, index) => (
            <span
              key={`${segment.color}-${index}`}
              className="h-full rounded-pill"
              style={{ backgroundColor: segment.color, flexGrow: segment.weight, flexBasis: 0 }}
            />
          ),
        )}
      </div>
    </button>
  );
}
