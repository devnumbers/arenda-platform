'use client';

import { type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { useInfiniteScroll } from '@/shared/lib/hooks/useInfiniteScroll';
import { clientTodayIso } from '@/entities/payment';
import { usePaymentOperationsPaged } from '@/features/payments';
import { Button, IconButton, PageContent, TopNav, TopNavTitle } from '@/shared/ui/design';
import {
  OverdueOperationRow,
  PaymentsEmptyState,
  PaymentsSkeleton,
  PaymentsStateCard,
} from './payments-sections';

/**
 * Полный список просроченных подэкраном страницы платежа (#466; фрейма в
 * Figma нет — спроектирован по решению владельца, резолюция #452): строки
 * «Графика» с red-стилизацией («N дней», красная сумма, danger-бейдж на
 * иконке категории), порядок asc — старейшая просрочка первой, порции по 50
 * с бесконечным скроллом. Строки не кликабельны — оплату делает кнопка
 * «Оплатить» на странице платежа (строки этого среза без индивидуальных
 * действий, резолюция #452).
 */
export function PaymentOverdueScreen({
  propertyId,
  paymentId,
}: {
  readonly propertyId: string;
  readonly paymentId: string;
}): JSX.Element {
  const router = useRouter();
  const overdueQuery = usePaymentOperationsPaged(propertyId, paymentId, {
    status: 'overdue',
    order: 'asc',
  });

  const sentinelRef = useInfiniteScroll(
    () => {
      if (overdueQuery.hasNextPage && !overdueQuery.isFetchingNextPage) {
        void overdueQuery.fetchNextPage();
      }
    },
    overdueQuery.hasNextPage === true,
  );

  const today = clientTodayIso();
  const operations = overdueQuery.data ?? [];

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
        <TopNavTitle title="Просроченные операции" />
      </TopNav>

      <PageContent>
        <div className="flex flex-col gap-2">
          {overdueQuery.isPending && (
            <>
              <PaymentsSkeleton withHeading />
              <PaymentsSkeleton withHeading />
            </>
          )}

          {overdueQuery.isError && (
            <PaymentsStateCard
              title="Не удалось загрузить просроченные"
              hint="Проверьте подключение и попробуйте снова"
              action={
                <Button
                  variant="secondary"
                  size="small"
                  onClick={() => void overdueQuery.refetch()}
                >
                  Повторить
                </Button>
              }
            />
          )}

          {!overdueQuery.isPending && !overdueQuery.isError && (
            <>
              {operations.length === 0 ? (
                // Иллюстрированное пустое состояние — как на общей странице
                // просроченных (1043:60174, решение владельца).
                <PaymentsEmptyState
                  image="/images/payments/empty-payments.png"
                  title="Нет просроченных операций"
                  hint="Когда платеж просрочится, он будет здесь"
                />
              ) : (
                <>
                  <section className="flex flex-col">
                    {operations.map((operation) => (
                      <OverdueOperationRow
                        key={operation.id}
                        operation={operation}
                        today={today}
                        variant="white"
                        className="py-3"
                      />
                    ))}
                  </section>

                  {overdueQuery.hasNextPage === true && (
                    <div ref={sentinelRef} aria-hidden />
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
