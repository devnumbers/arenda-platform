'use client';

import { type JSX, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { dateToIsoLocal } from '@/shared/lib/calendar';
import {
  PaymentRowButton,
  type PaymentOperation,
} from '@/entities/payment';
import { paidPaymentNumber, paymentOrdinalLabel, useRentals } from '@/features/rentals';
import { CategoryIcon, categoryStyle } from '@/features/payment-categories';
import { OPERATIONS_FEED_SORT } from '@/shared/api/query-keys';
import {
  groupPaidOperations,
  useHistoryOrder,
  usePaymentOperationsPaged,
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
 * «История операций» завершённой аренды (#535, Figma 1302:52209):
 * paid-вхождения Платежа арендной платы, группы по дате оплаты (сорт
 * сервера sort=paid_date — решение владельца 30.09, дополнение #994,
 * как в истории платежа #466), чип «Сначала новые» переключает
 * сортировку (серверная — порядок закреплён за API, порции по 50 с
 * бесконечным скроллом — канон #452); направление живёт в адресе
 * (?order=asc, дефолт не пишется, #785) — переживает перезагрузку.
 * Подзаголовок строки — порядковый номер платежа: нумерация следует
 * порядку оплат на экране (решение владельца, дополнение #994 — без
 * плановой раскладки), итог оплаченных — progress.paidMonths аренды.
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
  const historyQuery = usePaymentOperationsPaged(
    propertyId,
    rental?.rentPayment.paymentId ?? '',
    { status: 'paid', order, sort: OPERATIONS_FEED_SORT },
  );

  const operations = historyQuery.data ?? [];
  const groups = groupPaidOperations(operations, dateToIsoLocal(new Date()));
  // Итог оплаченных сервер считает по TZ собственника; рассинхрон с длиной
  // списка гасится в paidPaymentNumber.
  const paidTotal = Math.max(rental?.progress.paidMonths ?? 0, operations.length);
  // Позиция вхождения в общем desc/asc-списке — для порядкового номера.
  const positionById = new Map(operations.map((operation, index) => [operation.id, index]));

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
                          ordinal={paidPaymentNumber(
                            positionById.get(operation.id) ?? 0,
                            paidTotal,
                            order,
                          )}
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
 * название, порядковый номер платежа; сумма знаковая — доход зелёным
 * с плюсом. Тап — страница операции. */
function HistoryRow({
  operation,
  ordinal,
  onSelect,
}: {
  readonly operation: PaymentOperation;
  readonly ordinal: number;
  readonly onSelect?: () => void;
}): JSX.Element {
  const style = categoryStyle('default', operation.categorySlug);
  return (
    <PaymentRowButton
      className="px-3 py-3"
      categoryIcon={<CategoryIcon icon={style.icon} color={style.color} surface="white" />}
      title={operation.title}
      subtitle={paymentOrdinalLabel(ordinal)}
      amountKopecks={
        operation.type === 'expense' ? -operation.amountKopecks : operation.amountKopecks
      }
      signedAmount
      onSelect={onSelect}
    />
  );
}
