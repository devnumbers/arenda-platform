'use client';

import { useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import Image from 'next/image';
import { ArrowLeft, ChangeHorizontal } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { useInfiniteScroll } from '@/shared/lib/hooks/useInfiniteScroll';
import {
  clientTodayIso,
  PaymentRowButton,
  type PaymentOperation,
} from '@/entities/payment';
import { CategoryIcon, categoryStyle } from '@/features/payment-categories';
import {
  groupPaidOperations,
  usePaymentOperationsPaged,
} from '@/features/payments';
import { Button, ChipButton, IconButton, PageContent, TopNav, TopNavTitle } from '@/shared/ui/design';
import { operationStatusLabel } from '../lib/operation-status-label';
import {
  PaymentsHeading,
  PaymentsSkeleton,
  PaymentsStateCard,
} from './payments-sections';

/**
 * Подэкран «История операций» (#466, Figma 671:7776): только paid-вхождения,
 * группы «Сегодня» / «Вчера» / дата; чип «Новые» переключает сортировку
 * «сначала новые ↔ сначала старые» (серверная — порядок закреплён за API);
 * серверные порции по 50 с бесконечным скроллом. Суммы расходов — со
 * знаком минус (Figma). Группировка по дате вхождения: досрочно оплаченное
 * будущее вхождение остаётся в дате своего периода (учёт, не касса).
 * Пустая история — иллюстрация и «Платежей еще не было» (Figma 858:21271).
 */
export function PaymentHistoryScreen({
  propertyId,
  paymentId,
}: {
  readonly propertyId: string;
  readonly paymentId: string;
}): JSX.Element {
  const router = useRouter();
  // Дефолт — сначала новые (Figma); направление — часть ключа запроса.
  const [order, setOrder] = useState<'desc' | 'asc'>('desc');
  const historyQuery = usePaymentOperationsPaged(propertyId, paymentId, {
    status: 'paid',
    order,
  });

  const toggleOrder = (): void => {
    setOrder((current) => (current === 'desc' ? 'asc' : 'desc'));
  };

  const sentinelRef = useInfiniteScroll(
    () => {
      if (historyQuery.hasNextPage && !historyQuery.isFetchingNextPage) {
        void historyQuery.fetchNextPage();
      }
    },
    historyQuery.hasNextPage === true,
  );

  const today = clientTodayIso();
  const groups = groupPaidOperations(historyQuery.data ?? [], today);

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
        <TopNavTitle title="История операций" />
      </TopNav>

      <PageContent>
        <div className="flex flex-col gap-2">
          {historyQuery.isPending && (
            <>
              <PaymentsSkeleton withHeading />
              <PaymentsSkeleton withHeading />
            </>
          )}

          {historyQuery.isError && (
            <PaymentsStateCard
              title="Не удалось загрузить историю"
              hint="Проверьте подключение и попробуйте снова"
              action={
                <Button
                  variant="secondary"
                  size="small"
                  onClick={() => void historyQuery.refetch()}
                >
                  Повторить
                </Button>
              }
            />
          )}

          {!historyQuery.isPending && !historyQuery.isError && (
            <>
              {groups.length === 0 ? (
                // Пустая история (Figma 858:21271): иллюстрация 128 и одна
                // строка 16/18 — без карточки и подсказки.
                <div className="flex flex-col items-center gap-4 pt-24">
                  <Image
                    src="/images/payments/history-empty.png"
                    alt=""
                    width={128}
                    height={128}
                    className="h-32 w-32"
                  />
                  <p className="text-base leading-[18px] text-content">
                    Платежей еще не было
                  </p>
                </div>
              ) : (
                <>
                  <div className="px-6 pb-2">
                    <ChipButton
                      trailingIcon={<ChangeHorizontal />}
                      onClick={toggleOrder}
                      aria-label={
                        order === 'desc'
                          ? 'Сортировка: сначала новые — переключить на «сначала старые»'
                          : 'Сортировка: сначала старые — переключить на «сначала новые»'
                      }
                    >
                      {order === 'desc' ? 'Новые' : 'Старые'}
                    </ChipButton>
                  </div>

                  {groups.map((group) => (
                    <section key={group.label} className="flex flex-col">
                      <PaymentsHeading>{group.label}</PaymentsHeading>
                      {group.operations.map((operation) => (
                        <HistoryRow
                          key={operation.id}
                          operation={operation}
                          onSelect={() => router.push(ROUTES.propertyOperation(propertyId, operation.id))}
                        />
                      ))}
                    </section>
                  ))}

                  {historyQuery.hasNextPage === true && <div ref={sentinelRef} aria-hidden />}
                  {historyQuery.isFetchingNextPage && <LoadingMoreIndicator />}
                </>
              )}
            </>
          )}
        </div>
      </PageContent>
    </>
  );
}

/** Строка истории (Figma 671:7776, 1332:61665): иконка категории с белым
 * кантом (строка внутри страницы), название, подпись оплаты — «Заранее
 * на N дней» / «Задержан на N дней» / дата при точном попадании; сумма
 * справа знаковая: расход с минусом, доход с плюсом зелёным. onSelect
 * ведёт на страницу операции. */
function HistoryRow({
  operation,
  onSelect,
}: {
  readonly operation: PaymentOperation;
  readonly onSelect?: () => void;
}): JSX.Element {
  const style = categoryStyle('default', operation.categorySlug);
  const label = operationStatusLabel(operation);

  return (
    <PaymentRowButton
      className="px-3 py-3"
      categoryIcon={<CategoryIcon icon={style.icon} color={style.color} surface="white" />}
      title={operation.title}
      subtitle={label?.text}
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
