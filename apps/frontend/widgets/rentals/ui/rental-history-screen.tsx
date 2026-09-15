'use client';

import { useState, type JSX, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft, ChangeVertical } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import {
  clientTodayIso,
  PaymentRowButton,
  type PaymentOperation,
} from '@/entities/payment';
import { paidPaymentNumber, paymentOrdinalLabel, useRentals } from '@/features/rentals';
import { CategoryIcon, categoryStyle } from '@/features/payment-categories';
import { groupPaidOperations, usePaymentOperationsPaged } from '@/features/payments';
import {
  Button,
  ChipButton,
  EmptyState,
  IconButton,
  InfiniteQueryTail,
  PageContent,
  Skeleton,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';

/**
 * «История операций» завершённой аренды (#535, Figma 1302:52209):
 * paid-вхождения Платежа арендной платы, группы по дате вхождения, чип
 * «Сначала новые» переключает сортировку (серверная — порядок закреплён
 * за API, порции по 50 с бесконечным скроллом — канон #452). Подзаголовок
 * строки — порядковый номер платежа: нумерация по дате от старых, итог
 * оплаченных — progress.paidMonths аренды.
 */
export function RentalHistoryScreen({
  propertyId,
  rentalId,
}: {
  readonly propertyId: string;
  readonly rentalId: string;
}): JSX.Element {
  const router = useRouter();
  // Дефолт — сначала новые (макет); направление — часть ключа запроса.
  const [order, setOrder] = useState<'desc' | 'asc'>('desc');
  const rentalsQuery = useRentals(propertyId);
  const rental = rentalsQuery.data?.find((item) => item.id === rentalId);
  const historyQuery = usePaymentOperationsPaged(
    propertyId,
    rental?.rentPayment.paymentId ?? '',
    { status: 'paid', order },
  );

  const toggleOrder = (): void => {
    setOrder((current) => (current === 'desc' ? 'asc' : 'desc'));
  };

  const operations = historyQuery.data ?? [];
  const groups = groupPaidOperations(operations, clientTodayIso());
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

  return (
    <>
      <TopNav
        leading={<IconButton icon={<ArrowLeft />} label="Назад" onClick={back} />}
      >
        <TopNavTitle title="История операций" />
      </TopNav>

      <PageContent>
        <div className="flex flex-col gap-2">
          {rentalsQuery.isPending && (
            <div className="flex flex-col gap-4 pt-6">
              <Skeleton className="h-14 w-full" />
              <Skeleton className="h-14 w-full" />
              <Skeleton className="h-14 w-full" />
            </div>
          )}

          {(rentalsQuery.isError || historyQuery.isError) && (
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

          {!rentalsQuery.isPending && !rentalsQuery.isError && !historyQuery.isError && (
            <>
              {groups.length === 0 ? (
                <EmptyState
                  imageSrc="/images/rentals/rental-hero.png"
                  title="Не было операций"
                />
              ) : (
                <>
                  <div className="px-6 pb-2">
                    <ChipButton
                      trailingIcon={<ChangeVertical />}
                      onClick={toggleOrder}
                      aria-label={
                        order === 'desc'
                          ? 'Сортировка: сначала новые — переключить на «сначала старые»'
                          : 'Сортировка: сначала старые — переключить на «сначала новые»'
                      }
                    >
                      {order === 'desc' ? 'Сначала новые' : 'Сначала старые'}
                    </ChipButton>
                  </div>

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

