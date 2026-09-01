'use client';

import { useMemo, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft, ChevronDown, Search } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { useInfiniteScroll } from '@/shared/lib/hooks/useInfiniteScroll';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import {
  groupOperationsByDate,
  operationsMonthOf,
  operationsMonthRange,
  usePropertyOperationsScopedPaged,
  usePropertyOperationsSummary,
} from '@/features/payments';
import { clientTodayIso } from '@/entities/payment';
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
import { summaryBarSegments, type SummaryBarSegment } from '../lib/summary-bar';

/** Пилюля полосы без операций в периоде (Figma 1510-77101): серая #D3D7D9. */
const EMPTY_BAR_COLOR = '#D3D7D9';

/**
 * Экран «Операции объекта» (#474, Figma 1492-41825): оплаченные операции
 * за период, сгруппированные по датам; сверху — чипы «Период» (выбран,
 * синий; дефолт — текущий месяц) и «Категория» (шиты выбора — тикет #477),
 * под ними карточки «Расходы»/«Доходы» с суммой за период; у обоих —
 * полоса-разбивка пилюлями категорий (все категории с операциями, зазор
 * 2px, пропорционально суммам — Figma 1510-77101, решение владельца
 * 2026-09-01 отменяет схему #474 «топ-4 + остаток белым, доходы белым»).
 * Карточки ведут на экраны «Расходы
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
