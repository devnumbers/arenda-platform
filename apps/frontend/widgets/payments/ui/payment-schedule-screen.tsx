'use client';

import { useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft, Star } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { useInfiniteScroll } from '@/shared/lib/hooks/useInfiniteScroll';
import {
  clientTodayIso,
  formatDayMonthWithYear,
  isDatePaused,
  PaymentRowButton,
  type IsoDate,
  type Payment,
  type PaymentCategoryView,
  type PaymentOperation,
} from '@/entities/payment';
import { CategoryIcon, categoryStyle } from '@/features/payment-categories';
import {
  buildScheduleList,
  usePayment,
  usePaymentOperationsByStatus,
  type ScheduleEntry,
} from '@/features/payments';
import { Button, IconButton, PageContent, TopNav, TopNavTitle } from '@/shared/ui/design';
import { PaymentsEmptyCard, PaymentsHeading, PaymentsSkeleton, PaymentsStateCard } from './payments-sections';

/**
 * Подэкран «График платежей» (#466, Figma 671:7358): «Ближайший» —
 * материализованное сервером плановое вхождение, «Следующие» — клиентская
 * проекция портом вхождений; единый список порциями по 50 с бесконечным
 * скроллом до endDate (правило платформы — без «Показать еще», резолюция
 * #452). Пустые состояния: «На паузе» и завершённое правило. Просрочки в
 * график не входят — у них отдельный подэкран.
 */
const SCHEDULE_PAGE_SIZE = 50;

export function PaymentScheduleScreen({
  propertyId,
  paymentId,
}: {
  readonly propertyId: string;
  readonly paymentId: string;
}): JSX.Element {
  const router = useRouter();
  const paymentQuery = usePayment(propertyId, paymentId);
  const plannedQuery = usePaymentOperationsByStatus(propertyId, paymentId, 'planned');

  const loading = paymentQuery.isPending || plannedQuery.isPending;
  const failed = paymentQuery.isError || plannedQuery.isError;

  // Локали вместо сужения прямо в JSX — паттерн экрана «Платежи объекта».
  const payment = paymentQuery.isSuccess ? paymentQuery.data : undefined;
  const plannedOperations = plannedQuery.isSuccess ? plannedQuery.data : [];

  return (
    <>
      <TopNav
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            onClick={() => goBack(router, ROUTES.propertyPayment(propertyId, paymentId))}
          />
        }
      >
        <TopNavTitle title="График платежей" />
      </TopNav>

      <PageContent>
        <div className="flex flex-col gap-6">
          {loading && (
            <>
              <PaymentsSkeleton withHeading />
              <PaymentsSkeleton withHeading />
            </>
          )}

          {failed && (
            <PaymentsStateCard
              title="Не удалось загрузить график"
              hint="Проверьте подключение и попробуйте снова"
              action={
                <Button
                  variant="secondary"
                  size="small"
                  onClick={() => {
                    void paymentQuery.refetch();
                    void plannedQuery.refetch();
                  }}
                >
                  Повторить
                </Button>
              }
            />
          )}

          {!loading && !failed && payment !== undefined && (
            <ScheduleList
              schedule={payment}
              plannedOperations={plannedOperations}
              today={clientTodayIso()}
            />
          )}
        </div>
      </PageContent>
    </>
  );
}

function ScheduleList({
  schedule,
  plannedOperations,
  today,
}: {
  readonly schedule: Payment;
  readonly plannedOperations: ReadonlyArray<PaymentOperation>;
  readonly today: IsoDate;
}): JSX.Element {
  const entries = buildScheduleList(schedule, plannedOperations, today);

  const [visibleCount, setVisibleCount] = useState(SCHEDULE_PAGE_SIZE);
  const hasMore = visibleCount < entries.length;
  const sentinelRef = useInfiniteScroll(
    () => setVisibleCount((count) => count + SCHEDULE_PAGE_SIZE),
    hasMore,
  );

  // На активной паузе график пуст независимо от списков (резолюция #452):
  // даже если плановые строки ещё в кэше — домен на паузе не генерирует.
  // Дальше пустой список — завершённое правило (endDate позади).
  const nearest = entries[0];
  if (isDatePaused(schedule.pauses, today)) {
    // Авторские тексты: состояний паузы этого подэкрана во Figma нет.
    return (
      <PaymentsEmptyCard
        title="На паузе"
        hint="Возобновите платеж — и его график появится здесь"
      />
    );
  }
  if (nearest === undefined) {
    return (
      <PaymentsEmptyCard
        title="Платеж завершен"
        hint="У правила с датой окончания новых вхождений не будет"
      />
    );
  }

  const visibleFollowing = entries
    .slice(1)
    .slice(0, visibleCount - 1);

  return (
    <>
      <section className="flex flex-col">
        <PaymentsHeading>Ближайший</PaymentsHeading>
        <ScheduleRow
          entry={nearest}
          category={schedule.category}
          fallbackTitle={schedule.title}
          defaultAmountKopecks={schedule.amountKopecks}
          isFavorite={schedule.isFavorite}
          today={today}
        />
      </section>

      {visibleFollowing.length > 0 && (
        <section className="flex flex-col">
          <PaymentsHeading>Следующие</PaymentsHeading>
          {visibleFollowing.map((entry) => (
            <ScheduleRow
              key={entry.kind === 'operation' ? entry.operation.id : entry.date}
              entry={entry}
              category={schedule.category}
              fallbackTitle={schedule.title}
              defaultAmountKopecks={schedule.amountKopecks}
              isFavorite={schedule.isFavorite}
              today={today}
            />
          ))}
        </section>
      )}

      {hasMore && <div ref={sentinelRef} aria-hidden />}
    </>
  );
}

/** Строка графика (Figma 671:7358): дата вхождения в подзаголовке (год —
 * вне текущего), звезда избранного рядом с датой — паттерн «Платежей
 * объекта»; иконка категории — у каждой строки (Figma): у операции — слаг
 * снапшота, в проекции — категория правила (фолбэк и для операции без
 * слага — снапшот пользовательской категории). */
function ScheduleRow({
  entry,
  category,
  fallbackTitle,
  defaultAmountKopecks,
  isFavorite,
  today,
}: {
  readonly entry: ScheduleEntry;
  readonly category: PaymentCategoryView;
  readonly fallbackTitle: string;
  readonly defaultAmountKopecks: number;
  readonly isFavorite: boolean;
  readonly today: IsoDate;
}): JSX.Element {
  const operationSlug =
    entry.kind === 'operation' ? entry.operation.categorySlug : undefined;
  const style =
    operationSlug !== undefined
      ? categoryStyle('default', operationSlug)
      : categoryStyle(category.source, category.slug);

  return (
    <PaymentRowButton
      className="px-3 py-3"
      categoryIcon={<CategoryIcon icon={style.icon} color={style.color} surface="muted" />}
      title={entry.kind === 'operation' ? entry.operation.title : fallbackTitle}
      subtitle={
        <span className="inline-flex items-center gap-1">
          {formatDayMonthWithYear(
            entry.kind === 'operation' ? entry.operation.date : entry.date,
            today,
          )}
          {isFavorite && <Star className="h-4 w-4 shrink-0" aria-hidden />}
        </span>
      }
      amountKopecks={
        entry.kind === 'operation' ? entry.operation.amountKopecks : defaultAmountKopecks
      }
    />
  );
}
