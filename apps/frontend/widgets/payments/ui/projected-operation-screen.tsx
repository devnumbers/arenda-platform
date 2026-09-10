'use client';

import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { clientTodayIso, formatDayMonth, type IsoDate } from '@/entities/payment';
import { useProperty } from '@/features/properties';
import { projectedOperation, usePayment } from '@/features/payments';
import { Button, IconButton, PageContent, TopNav, TopNavTitle } from '@/shared/ui/design';
import { PaymentsStateCard } from './payments-sections';
import { OperationDetailSkeleton } from './payments-skeletons';
import { OperationView } from './operation-detail-screen';

/**
 * Просмотр проекции будущего вхождения (решение владельца — чисто фронт):
 * в БД у правила лежит ровно одна будущая planned, остальные даты считаются
 * клиентской проекцией. Тот же вид операции (общий OperationView), но без
 * кнопки оплаты — платится только материализованная операция; фактических
 * строк нет по построению. Дата сверяется с текущим расписанием правила:
 * прямый заход по устаревшей/невалидной дате показывает «Вхождение не
 * найдено» (диплинк-гарантий нет — проекция сдвигается правками правила).
 */
export function ProjectedOperationScreen({
  propertyId,
  paymentId,
  date,
}: {
  readonly propertyId: string;
  readonly paymentId: string;
  readonly date: IsoDate;
}): JSX.Element {
  const router = useRouter();
  const paymentQuery = usePayment(propertyId, paymentId);
  const propertyQuery = useProperty(propertyId);

  const property = propertyQuery.isSuccess ? propertyQuery.data : undefined;
  const payment = paymentQuery.isSuccess ? paymentQuery.data : undefined;

  const loading = paymentQuery.isPending || propertyQuery.isPending;
  const failed = paymentQuery.isError || propertyQuery.isError;

  const today = clientTodayIso();
  const operation =
    payment !== undefined ? projectedOperation(payment, date, today) : undefined;

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
        <TopNavTitle title={operation !== undefined ? formatDayMonth(date) : 'Операция'} />
      </TopNav>

      <PageContent>
        <div className="flex flex-col gap-6">
          {loading && <OperationDetailSkeleton />}

          {failed && (
            <PaymentsStateCard
              title="Не удалось загрузить операцию"
              hint="Проверьте подключение и попробуйте снова"
              action={
                <Button
                  variant="secondary"
                  size="small"
                  onClick={() => {
                    void paymentQuery.refetch();
                    void propertyQuery.refetch();
                  }}
                >
                  Повторить
                </Button>
              }
            />
          )}

          {!loading && !failed && payment !== undefined && operation === undefined && (
            <PaymentsStateCard
              title="Вхождение не найдено"
              hint="Этой даты нет в текущем расписании платежа"
              action={
                <Button
                  variant="secondary"
                  size="small"
                  onClick={() => router.push(ROUTES.propertyPaymentSchedule(propertyId, paymentId))}
                >
                  К графику
                </Button>
              }
            />
          )}

          {!loading && !failed && operation !== undefined && (
            <OperationView
              propertyId={propertyId}
              operation={operation}
              propertyTitle={property?.name ?? ''}
              today={today}
            />
          )}
        </div>
      </PageContent>
    </>
  );
}
