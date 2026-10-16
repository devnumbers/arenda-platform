'use client';

import { type JSX, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { dateToIsoLocal, type IsoDate } from '@/shared/lib/calendar';
import { formatDayMonthWithYear } from '@/shared/lib/date-format';
import {
  PaymentRowButton,
  type PaymentOperation,
} from '@/entities/payment';
import { completedRentalOperationsScope, useRentals } from '@/features/rentals';
import { CategoryIcon, categoryStyle } from '@/features/payment-categories';
import {
  groupPaidOperations,
  useHistoryOrder,
  usePropertyOperationsScopedPaged,
  HistoryOrderChip,
  type HistoryOrder,
} from '@/features/payments';
import {
  Button,
  EmptyState,
  IconButton,
  InfiniteQueryTail,
  PageContent,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { RentalHistoryFeedSkeleton } from './rental-skeletons';

/**
 * «История операций» завершённой аренды (#535, Figma 1302:52209; ревизия
 * #1161): Платёж арендной платы удалён Завершением, источником служат
 * операции объекта за период аренды [начало, дата завершения] — тот же
 * источник, что у «Итогов аренды» (период фильтруется серверно по дате
 * вхождения). Группы по дате оплаты строки, чип «Сначала новые»
 * переключает сортировку (серверная — порядок закреплён за API, порции по
 * 50 с бесконечным скроллом — канон #452); направление живёт в адресе
 * (?order=asc, дефолт не пишется, #785) — переживает перезагрузку.
 * Подзаголовок строки — дата оплаты: в списке соседствуют операции любых
 * платежей и ручные, порядковая нумерация «N-й платеж» чужие строки
 * метила бы неверно.
 */
export function RentalHistoryScreen({
  propertyId,
  rentalId,
  initialOrder,
}: {
  readonly propertyId: string;
  readonly rentalId: string;
  readonly initialOrder?: HistoryOrder;
}): JSX.Element {
  const router = useRouter();
  // Дефолт — сначала новые (макет); направление — в адресе (#785) и часть
  // ключа запроса.
  const { order, toggleOrder } = useHistoryOrder(initialOrder);
  const rentalsQuery = useRentals(propertyId);
  const rental = rentalsQuery.data?.find((item) => item.id === rentalId);
  // Запрос глушится, пока аренда не найдена: без её дат период не собрать.
  const historyQuery = usePropertyOperationsScopedPaged(
    propertyId,
    // Скоуп собирается только при найденной аренде: без её дат период не
    // собрать — запрос глушится enabled'ом ниже.
    completedRentalOperationsScope(rental, order),
    { enabled: rental !== undefined },
  );

  const operations = historyQuery.data ?? [];
  const groups = groupPaidOperations(operations, dateToIsoLocal(new Date()));

  const back = (): void =>
    goBack(
      router,
      rental !== undefined
        ? ROUTES.propertyRentalCompleted(propertyId, rental.id)
        : ROUTES.propertyRentalPast(propertyId),
    );

  // Чип сортировки — вне фазы загрузки (паритет §7, механика истории
  // платежей #605): живой уже в pending, над скелетоном и готовым списком;
  // на пустой истории прячется вместе с контентом. Направление — в адресе
  // (#785), переключение во время загрузки безвредно: меняет ключ
  // серверного запроса.
  const sortChipRow = (
    <div className="px-6 pb-2">
      <HistoryOrderChip order={order} onToggle={toggleOrder} />
    </div>
  );

  return (
    <>
      <TopNav
        leading={<IconButton icon={<ArrowLeft />} label="Назад" onClick={back} />}
      >
        <TopNavTitle title="История операций" />
      </TopNav>

      <PageContent>
        <div className="flex flex-col gap-2">
          {/* Ветви взаимоисключающие — по достигнутому состоянию: запрос
              истории заглушен, пока аренда не найдена (пустой paymentId),
              его pending/error значимы только при найденной аренде. */}
          {(rentalsQuery.isPending ||
            (rental !== undefined && historyQuery.isPending)) && (
            <>
              {sortChipRow}
              <RentalHistoryFeedSkeleton />
            </>
          )}

          {(rentalsQuery.isError || (rental !== undefined && historyQuery.isError)) && (
            <div className="flex flex-col items-center gap-4 pt-6">
              <p className="text-center text-base leading-[18px] text-content-secondary">
                Не удалось загрузить историю
              </p>
              <Button
                variant="secondary"
                size="small"
                onClick={() => {
                  void rentalsQuery.refetch();
                  void historyQuery.refetch();
                }}
              >
                Повторить
              </Button>
            </div>
          )}

          {/* Прямая ссылка на несуществующую аренду — честная пустота
              вместо вечного скелетона. */}
          {rentalsQuery.isSuccess && rental === undefined && (
            <EmptyState
              imageSrc="/images/rentals/rental-hero.png"
              title="Аренда не найдена"
              description="Возможно, она была удалена"
            />
          )}

          {rental !== undefined &&
            !rentalsQuery.isError &&
            !historyQuery.isPending &&
            !historyQuery.isError && (
            <>
              {groups.length === 0 ? (
                <EmptyState
                  imageSrc="/images/rentals/rental-hero.png"
                  title="Не было операций"
                />
              ) : (
                <>
                  {sortChipRow}

                  {groups.map((group) => (
                    <section key={group.label} className="flex flex-col">
                      <GroupHeading>{group.label}</GroupHeading>
                      {group.operations.map((operation) => (
                        <HistoryRow
                          key={operation.id}
                          operation={operation}
                          today={dateToIsoLocal(new Date())}
                          onSelect={() =>
                            router.push(ROUTES.propertyOperation(propertyId, operation.id))
                          }
                        />
                      ))}
                    </section>
                  ))}

                  <InfiniteQueryTail query={historyQuery} />
                </>
              )}
            </>
          )}
        </div>
      </PageContent>
    </>
  );
}

/** Заголовок группы дат (канон заголовков секций платежей, #466):
 * H2 20/24 с боковыми полями контента. */
function GroupHeading({ children }: { readonly children: ReactNode }): JSX.Element {
  return <h2 className="px-6 text-xl font-semibold leading-6 text-content">{children}</h2>;
}

/** Строка истории аренды (1302:52209): иконка категории с белым кантом,
 * название, дата оплаты строки (фактическая, с плановой в запасе);
 * сумма знаковая — доход зелёным с плюсом. Тап — страница операции. */
function HistoryRow({
  operation,
  today,
  onSelect,
}: {
  readonly operation: PaymentOperation;
  readonly today: IsoDate;
  readonly onSelect?: () => void;
}): JSX.Element {
  const style = categoryStyle('default', operation.categorySlug);
  return (
    <PaymentRowButton
      className="px-3 py-3"
      categoryIcon={<CategoryIcon icon={style.icon} color={style.color} surface="white" />}
      title={operation.title}
      subtitle={formatDayMonthWithYear(operation.paidDate ?? operation.date, today)}
      amountKopecks={
        operation.type === 'expense' ? -operation.amountKopecks : operation.amountKopecks
      }
      signedAmount
      onSelect={onSelect}
    />
  );
}
