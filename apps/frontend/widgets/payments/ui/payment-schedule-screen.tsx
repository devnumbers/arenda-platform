'use client';

import { useMemo, useState, type JSX } from 'react';
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
  extendProjection,
  materializedEntries,
  projectionCursor,
  usePayment,
  usePaymentOperationsByStatus,
  type ScheduleEntry,
} from '@/features/payments';
import { Button, IconButton, PageContent, TopNav, TopNavTitle } from '@/shared/ui/design';
import { PaymentsEmptyCard, PaymentsHeading, PaymentsStateCard } from './payments-sections';
import { PaymentGroupedListSkeleton } from './payments-skeletons';

/**
 * Подэкран «График платежей» (#466, Figma 671:7358): «Ближайший» —
 * материализованное сервером плановое вхождение, «Следующие» — клиентская
 * проекция портом вхождений; единый список порциями по 50 с бесконечным
 * скроллом (правило платформы — без «Показать еще», резолюция #452). У
 * бессрочного правила проекция догружается бесконечно — страницы
 * генерируются на лету от курсора, без горизонта и потолка; ограничение
 * только у правила с endDate. Пустые состояния: «На паузе» и завершённое
 * правило. Просрочки в график не входят — у них отдельный подэкран.
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
          {loading && <PaymentGroupedListSkeleton groups={[1, 2]} />}

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
              propertyId={propertyId}
              paymentId={paymentId}
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
  propertyId,
  paymentId,
  schedule,
  plannedOperations,
  today,
}: {
  readonly propertyId: string;
  readonly paymentId: string;
  readonly schedule: Payment;
  readonly plannedOperations: ReadonlyArray<PaymentOperation>;
  readonly today: IsoDate;
}): JSX.Element {
  const router = useRouter();
  const materialized = materializedEntries(plannedOperations);
  const [visibleCount, setVisibleCount] = useState(SCHEDULE_PAGE_SIZE);

  // Проекция — чистое вычисление по видимой глубине: страницы по 50
  // генерируются на лету от курсора, без горизонта и потолка; у бессрочного
  // правила догрузка бесконечна, исчерпание (endDate позади) фиксирует
  // страница, вернувшая меньше полного размера. Память держит только
  // проскролленное; при уходе с экрана вычисление умирает с компонентом,
  // серверные порции освобождает штатный GC react-query.
  const projection = useMemo(() => {
    let page = extendProjection(
      schedule,
      projectionCursor(plannedOperations, today),
      SCHEDULE_PAGE_SIZE,
    );
    const dates = [...page.dates];
    while (!page.exhausted && dates.length < visibleCount) {
      page = extendProjection(schedule, page.nextCursor, SCHEDULE_PAGE_SIZE);
      dates.push(...page.dates);
    }
    return { dates, exhausted: page.exhausted };
  }, [schedule, plannedOperations, today, visibleCount]);

  const totalFollowing = materialized.length - 1 + projection.dates.length;
  const hasMore = visibleCount - 1 < totalFollowing || !projection.exhausted;
  const sentinelRef = useInfiniteScroll(
    () => setVisibleCount((count) => count + SCHEDULE_PAGE_SIZE),
    hasMore,
  );

  // На активной паузе график пуст независимо от списков (резолюция #452):
  // даже если плановые строки ещё в кэше — домен на паузе не генерирует.
  // Дальше пустой список — завершённое правило (endDate позади).
  const firstProjected = projection.dates[0];
  const nearest = materialized[0] ?? (firstProjected !== undefined
    ? ({ kind: 'projected', date: firstProjected } as const)
    : undefined);
  if (isDatePaused(schedule.pauses, today)) {
    // Авторские тексты: состояний паузы этого подэкрана во Figma нет.
    return (
      <PaymentsEmptyCard
        title="На паузе"
        hint="Возобновите платеж — и его график появится здесь"
      />
    );
  }
  // Завершённое правило (isCompleted — серверный вычисляемый флаг): пустое
  // состояние сразу после оплаты последнего вхождения. Без флага проекция от
  // курсора «вчера» заново порождала бы уже оплаченные даты и «ближайший»
  // указывал на прошлое (баг короткого правила, оплаченного до endDate).
  if (schedule.isCompleted) {
    return (
      <PaymentsEmptyCard
        title="Платеж завершен"
        hint="У правила с датой окончания новых вхождений не будет"
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

  // Любая строка графика открывает операцию (решение владельца):
  // материализованная — по id, проекция — вью без кнопки оплаты.
  const selectEntry = (entry: ScheduleEntry): void => {
    if (entry.kind === 'operation') {
      router.push(ROUTES.propertyOperation(propertyId, entry.operation.id));
    } else {
      router.push(ROUTES.propertyPaymentProjectedOperation(propertyId, paymentId, entry.date));
    }
  };

  // «Следующие»: всё после ближайшего — хвост материализованных и страницы
  // проекции; глубина показа — visibleCount.
  const following: ScheduleEntry[] = [
    ...materialized.slice(1),
    ...projection.dates.map((date): ScheduleEntry => ({ kind: 'projected', date })),
  ];
  const visibleFollowing = following.slice(0, Math.max(visibleCount - 1, 0));

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
          onSelect={() => selectEntry(nearest)}
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
              onSelect={() => selectEntry(entry)}
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
  onSelect,
}: {
  readonly entry: ScheduleEntry;
  readonly category: PaymentCategoryView;
  readonly fallbackTitle: string;
  readonly defaultAmountKopecks: number;
  readonly isFavorite: boolean;
  readonly today: IsoDate;
  readonly onSelect?: () => void;
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
      // Строка внутри страницы — кант белый; звезда favorite-правила — перед
      // датой (1323:61133).
      categoryIcon={<CategoryIcon icon={style.icon} color={style.color} surface="white" />}
      title={entry.kind === 'operation' ? entry.operation.title : fallbackTitle}
      subtitle={
        <span className="inline-flex items-center gap-1">
          {isFavorite && <Star className="h-4 w-4 shrink-0" aria-hidden />}
          {formatDayMonthWithYear(
            entry.kind === 'operation' ? entry.operation.date : entry.date,
            today,
          )}
        </span>
      }
      amountKopecks={
        entry.kind === 'operation' ? entry.operation.amountKopecks : defaultAmountKopecks
      }
      onSelect={onSelect}
    />
  );
}
