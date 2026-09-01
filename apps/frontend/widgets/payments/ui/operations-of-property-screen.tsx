'use client';

import { useMemo, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft, ChevronDown, Search } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { useInfiniteScroll } from '@/shared/lib/hooks/useInfiniteScroll';
import { formatMoneyKopecks, ratioToPercent } from '@/shared/lib/format-money';
import { categoryStyle } from '@/features/payment-categories';
import {
  groupOperationsByDate,
  operationsMonthOf,
  operationsMonthRange,
  usePropertyOperationsScopedPaged,
  usePropertyOperationsSummary,
} from '@/features/payments';
import {
  clientTodayIso,
  type OperationsSummary,
} from '@/entities/payment';
import {
  Button,
  ChipButton,
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

/**
 * Экран «Операции объекта» (#474, Figma 1492-41825): оплаченные операции
 * за период, сгруппированные по датам; сверху — чипы «Период» (выбран,
 * синий; дефолт — текущий месяц) и «Категория» (шиты выбора — тикет #477),
 * под ними карточки «Расходы»/«Доходы» с суммой за период; у расходов —
 * полоса-разбивка по категориям (топ-4 + остаток белым), у доходов —
 * сплошная белая (Figma 1492:59525). Карточки ведут на экраны «Расходы
 * объекта»/«Доходы объекта» (#475). Сводка и список считают один и тот же
 * скоуп на сервере (#473) — карточки и список всегда согласны. Порции по 50
 * с бесконечным скроллом; строка ведёт на страницу операции. Поиск — иконка
 * в хедере (экран поиска — тикет #476).
 */
export function OperationsOfPropertyScreen({
  propertyId,
}: {
  readonly propertyId: string;
}): JSX.Element {
  const router = useRouter();

  const today = clientTodayIso();
  const period = useMemo(() => operationsMonthRange(operationsMonthOf(today)), [today]);
  const scope = {
    status: 'paid',
    order: 'desc',
    dateFrom: period.from,
    dateTo: period.to,
  } as const;

  const listQuery = usePropertyOperationsScopedPaged(propertyId, scope);
  const summaryQuery = usePropertyOperationsSummary(propertyId, scope);

  const sentinelRef = useInfiniteScroll(
    () => {
      if (listQuery.hasNextPage && !listQuery.isFetchingNextPage) {
        void listQuery.fetchNextPage();
      }
    },
    listQuery.hasNextPage === true,
  );

  const groups = groupOperationsByDate(listQuery.data ?? [], today);
  const segments = expenseSegments(summaryQuery.data);
  const pending = listQuery.isPending || summaryQuery.isPending;

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
          {/* Чипы фильтров (Figma 1492:59532): период выбран — синий, категория
           * серая; сами шиты выбора — тикет #477. */}
          <div className="flex gap-1.5 overflow-x-auto px-6">
            <ChipButton selected trailingIcon={<ChevronDown />}>
              {period.label}
            </ChipButton>
            <ChipButton trailingIcon={<ChevronDown />}>Категория</ChipButton>
          </div>

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
                  {/* Карточки сводки (Figma 1492:59516/1492:59525): сумма за
                   * период + полоса; расходы — разбивка по категориям. Клик —
                   * вход на экран направления (#475). */}
                  <div className="flex gap-2 px-6">
                    <SummaryCard
                      label="Расходы"
                      totalKopecks={summaryQuery.data?.expenseTotalKopecks}
                      segments={segments}
                      openLabel="Открыть расходы объекта"
                      onOpen={() => router.push(ROUTES.propertyOperationsExpense(propertyId))}
                    />
                    <SummaryCard
                      label="Доходы"
                      totalKopecks={summaryQuery.data?.incomeTotalKopecks}
                      segments={[]}
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

/** Карточка сводки (Figma 1492:59516, EL-3091ce92): серая карточка radius 24,
 * сумма 16/500, подпись 14/400, полоса 6px со скруглением; сегменты —
 * категории расходов (цвет каталога), остаток и пустая разбивка — белым.
 * Вся карточка — кнопка на экран направления (#475), как заголовки секций
 * «Платежей объекта». */
function SummaryCard({
  label,
  totalKopecks,
  segments,
  openLabel,
  onOpen,
}: {
  readonly label: string;
  readonly totalKopecks: number | undefined;
  readonly segments: ReadonlyArray<SummarySegment>;
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
      <div className="mt-4 flex h-1.5 w-full overflow-hidden rounded-pill">
        {(segments.length > 0
          ? segments
          : [{ color: fallbackBarColor(totalKopecks), percent: 100 }]
        ).map((segment, index) => (
          <span
            key={`${segment.color}-${index}`}
            className="h-full"
            style={{ backgroundColor: segment.color, width: `${segment.percent}%` }}
          />
        ))}
      </div>
    </button>
  );
}

/** Цвет полосы без разбивки (Figma 1510-77309): пустой период (0 ₽) —
 * серый #D3D7D9 (токен --dl-input-border), есть операции — белая
 * (главный макет 1492-41825, карточка «Доходы»). */
function fallbackBarColor(totalKopecks: number | undefined): string {
  return totalKopecks !== undefined && totalKopecks > 0 ? '#FFFFFF' : '#D3D7D9';
}

type SummarySegment = {
  readonly color: string;
  readonly percent: number;
};

/** Полоса-разбивка расходов (Figma 1492:59521-59524): топ-4 категории за
 * период, ширины пропорциональны суммам, цвет — подложка иконки каталога;
 * остаток после топа — белым сегментом. */
function expenseSegments(
  summary: OperationsSummary | undefined,
): ReadonlyArray<SummarySegment> {
  if (summary === undefined || summary.expenseTotalKopecks <= 0) {
    return [];
  }
  const total = summary.expenseTotalKopecks;
  const top = summary.categories
    .filter((category) => category.type === 'expense')
    .slice(0, 4);
  const segments = top.map((category) => ({
    color: categoryStyle('default', category.slug).color,
    percent: ratioToPercent(category.totalKopecks / total),
  }));
  const covered = segments.reduce((sum, segment) => sum + segment.percent, 0);
  if (covered < 99) {
    segments.push({ color: '#FFFFFF', percent: 100 - covered });
  }
  return segments;
}
