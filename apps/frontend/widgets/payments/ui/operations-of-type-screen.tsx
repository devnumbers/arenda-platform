'use client';

import { useMemo, useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import {
  ArrowLeft,
  ArrowSLeft,
  ArrowSRight,
  ChevronDown,
  Search,
} from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { useInfiniteScroll } from '@/shared/lib/hooks/useInfiniteScroll';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import {
  Button,
  ChipButton,
  IconButton,
  PageContent,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import {
  groupOperationsByDate,
  isCurrentOperationsMonth,
  operationsMonthOf,
  operationsMonthRange,
  shiftOperationsMonth,
  usePropertyOperationsScopedPaged,
  usePropertyOperationsSummary,
  type OperationsMonth,
} from '@/features/payments';
import { clientTodayIso, type PaymentOperation } from '@/entities/payment';
import { PaymentsSkeleton, PaymentsStateCard } from './payments-sections';
import { LoadingMoreIndicator, OperationsDateList } from './operations-list';

/** Копия экрана по направлению (Figma 1494-61191 / 1492-59865). */
const SCREEN_COPY = {
  income: { title: 'Доходы объекта' },
  expense: { title: 'Расходы объекта' },
} as const;

/**
 * Экран «Доходы объекта» / «Расходы объекта» (#475, Figma 1494-61191 и
 * 1492-59865): те же чипы, что на главном экране операций, вместо двух
 * карточек — H1-сумма направления за период с круглыми стрелками листания
 * по месяцам (ArrowSLeft/ArrowSRight 44×44); ниже — список операций одного
 * типа за период, группировка и строки как на главном (общий
 * OperationsDateList). Скоуп сужен `type` — фильтр списка и сводки #473.
 * Листание: правая стрелка гасится на текущем месяце — на экранах только
 * paid-операции (резолюция #474), в будущем их не бывает; влево — без
 * границы, месяцы без операций показывают пустое состояние. Порции по 50
 * с бесконечным скроллом; чипы выбора периода/категории — тикет #477.
 */
export function OperationsOfTypeScreen({
  propertyId,
  type,
}: {
  readonly propertyId: string;
  readonly type: keyof typeof SCREEN_COPY;
}): JSX.Element {
  const router = useRouter();

  const today = clientTodayIso();
  const [month, setMonth] = useState<OperationsMonth>(() => operationsMonthOf(today));
  const period = useMemo(() => operationsMonthRange(month), [month]);
  const atCurrentMonth = isCurrentOperationsMonth(month, today);

  const scope = {
    status: 'paid',
    order: 'desc',
    type,
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
  const totalKopecks =
    summaryQuery.data === undefined
      ? undefined
      : type === 'expense'
        ? summaryQuery.data.expenseTotalKopecks
        : summaryQuery.data.incomeTotalKopecks;
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
            onClick={() => goBack(router, ROUTES.propertyOperations(propertyId))}
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
          {/* Чипы фильтров — как на главном (Figma 1502:65149): период следует
           * за листанием; шиты выбора — тикет #477. */}
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
                  {/* Сумма периода с листанием по месяцам (Figma 1502:65151):
                   * стрелки 44×44, H1 28/32 по центру. */}
                  <div className="flex items-stretch px-3.5">
                    <IconButton
                      icon={<ArrowSLeft />}
                      label="Предыдущий месяц"
                      onClick={() => setMonth(shiftOperationsMonth(month, -1))}
                    />
                    <div className="flex min-w-0 flex-1 items-center justify-center">
                      <span className="truncate text-[28px] font-semibold leading-8 text-content">
                        {totalKopecks === undefined ? '—' : formatMoneyKopecks(totalKopecks)}
                      </span>
                    </div>
                    <IconButton
                      icon={<ArrowSRight />}
                      label="Следующий месяц"
                      disabled={atCurrentMonth}
                      onClick={() => setMonth(shiftOperationsMonth(month, 1))}
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
