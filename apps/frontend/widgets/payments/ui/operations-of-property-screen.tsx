'use client';

import { useMemo, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft, ChevronDown, Search } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { useInfiniteScroll } from '@/shared/lib/hooks/useInfiniteScroll';
import { formatMoneyKopecks, ratioToPercent } from '@/shared/lib/format-money';
import {
  CategoryIcon,
  categoryStyle,
} from '@/features/payment-categories';
import {
  groupOperationsByDate,
  usePropertyOperationsScopedPaged,
  usePropertyOperationsSummary,
} from '@/features/payments';
import {
  clientTodayIso,
  PaymentRowButton,
  type OperationsSummary,
  type PaymentOperation,
} from '@/entities/payment';
import {
  Button,
  ChipButton,
  IconButton,
  MONTH_LABELS,
  PageContent,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import {
  PaymentsHeading,
  PaymentsSkeleton,
  PaymentsStateCard,
} from './payments-sections';

/**
 * Экран «Операции объекта» (#474, Figma 1492-41825): оплаченные операции
 * за период, сгруппированные по датам; сверху — чипы «Период» (выбран,
 * синий; дефолт — текущий месяц) и «Категория» (шиты выбора — тикет #477),
 * под ними карточки «Расходы»/«Доходы» с суммой за период; у расходов —
 * полоса-разбивка по категориям (топ-4 + остаток белым), у доходов —
 * сплошная белая (Figma 1492:59525). Сводка и список считают один и тот же
 * скоуп на сервере (#473) — карточки и список всегда согласны. Порции по 50
 * с бесконечным скроллом; строка ведёт на страницу операции. Поиск — иконка
 * в хедере (экран поиска — тикет #476); пустое состояние — заглушка до
 * тикета #478.
 */
export function OperationsOfPropertyScreen({
  propertyId,
}: {
  readonly propertyId: string;
}): JSX.Element {
  const router = useRouter();

  const today = clientTodayIso();
  const period = useMemo(() => currentMonthRange(today), [today]);
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

  const openOperation = (operation: PaymentOperation): void =>
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
                   * период + полоса; расходы — разбивка по категориям. */}
                  <div className="flex gap-2 px-6">
                    <SummaryCard
                      label="Расходы"
                      totalKopecks={summaryQuery.data?.expenseTotalKopecks}
                      segments={segments}
                    />
                    <SummaryCard
                      label="Доходы"
                      totalKopecks={summaryQuery.data?.incomeTotalKopecks}
                      segments={[]}
                    />
                  </div>

                  {groups.length === 0 ? (
                    // Заглушка пустого состояния; 1:1 с Figma 1518-92899 —
                    // тикет #478.
                    <div className="flex flex-col items-center pt-24">
                      <p className="text-base leading-[18px] text-content">
                        Операций за период нет
                      </p>
                    </div>
                  ) : (
                    <div className="flex flex-col gap-2">
                      {groups.map((group) => (
                        <section key={group.date} className="flex flex-col">
                          <PaymentsHeading>{group.label}</PaymentsHeading>
                          {group.operations.map((operation) => (
                            <OperationRow
                              key={operation.id}
                              operation={operation}
                              onSelect={() => openOperation(operation)}
                            />
                          ))}
                        </section>
                      ))}

                      {listQuery.hasNextPage === true && (
                        <div ref={sentinelRef} aria-hidden />
                      )}
                      {listQuery.isFetchingNextPage && <LoadingMoreIndicator />}
                    </div>
                  )}
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
 * категории расходов (цвет каталога), остаток и пустая разбивка — белым. */
function SummaryCard({
  label,
  totalKopecks,
  segments,
}: {
  readonly label: string;
  readonly totalKopecks: number | undefined;
  readonly segments: ReadonlyArray<SummarySegment>;
}): JSX.Element {
  return (
    <div className="min-w-0 flex-1 rounded-card bg-surface-muted px-6 pb-6 pt-5">
      <div className="flex flex-col gap-0.5">
        <span className="text-base font-medium text-content">
          {totalKopecks === undefined ? '—' : formatMoneyKopecks(totalKopecks)}
        </span>
        <span className="text-sm text-content">{label}</span>
      </div>
      <div className="mt-4 flex h-1.5 w-full overflow-hidden rounded-pill">
        {(segments.length > 0
          ? segments
          : [{ color: '#FFFFFF', percent: 100 }]
        ).map((segment, index) => (
          <span
            key={`${segment.color}-${index}`}
            className="h-full"
            style={{ backgroundColor: segment.color, width: `${segment.percent}%` }}
          />
        ))}
      </div>
    </div>
  );
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

/** Строка операции (1332:61665, Row Button White): иконка категории с белым
 * кантом, название, знаковая сумма — доход зелёным с плюсом, расход тёмным
 * с минусом (Figma 1492:42480). */
function OperationRow({
  operation,
  onSelect,
}: {
  readonly operation: PaymentOperation;
  readonly onSelect: () => void;
}): JSX.Element {
  const style = categoryStyle('default', operation.categorySlug);
  return (
    <PaymentRowButton
      className="px-3 py-3"
      categoryIcon={<CategoryIcon icon={style.icon} color={style.color} surface="white" />}
      title={operation.title}
      amountKopecks={
        operation.type === 'expense' ? -operation.amountKopecks : operation.amountKopecks
      }
      signedAmount
      onSelect={onSelect}
    />
  );
}

function LoadingMoreIndicator(): JSX.Element {
  return (
    <div className="flex justify-center py-4" role="status" aria-label="Загружаем еще">
      <div className="h-8 w-8 animate-pulse rounded-pill bg-surface-muted" />
    </div>
  );
}

/** Границы текущего месяца по клиентскому «сегодня»: «сегодня» интерфейса
 * считает сервер по TZ собственника (ADR 0048); клиентская зона влияет
 * только на выбор месяца по умолчанию — листание и шит периода придут в
 * тикете #477. */
function currentMonthRange(today: string): { from: string; to: string; label: string } {
  const year = Number(today.slice(0, 4));
  const month = Number(today.slice(5, 7)) - 1;
  const lastDay = new Date(year, month + 1, 0).getDate();
  const from = `${today.slice(0, 7)}-01`;
  const to = `${today.slice(0, 7)}-${String(lastDay).padStart(2, '0')}`;
  return { from, to, label: `${MONTH_LABELS[month]} ${year}` };
}
