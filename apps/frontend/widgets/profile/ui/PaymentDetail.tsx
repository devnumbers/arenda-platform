'use client';

import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import clsx from 'clsx';
import { Card } from '@heroui/react/card';
import { Skeleton } from '@heroui/react/skeleton';
import { Button } from '@/shared/ui/button';
import {
  PAYMENT_STALE_MS,
  useSubscriptionPayment,
} from '@/features/billing';
import {
  PAYMENT_PERIOD_LABELS,
  PAYMENT_STATUS_LABELS,
} from '@/entities/billing';
import { getTariffLabel } from '@/entities/user';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { formatDate } from '@/shared/lib/format-date';
import styles from './PaymentDetail.module.css';

function PaymentDetailSkeleton(): JSX.Element {
  return (
    <Card className={styles.card}>
      <Skeleton className={styles.rowSkeleton} />
      <Skeleton className={styles.rowSkeleton} />
      <Skeleton className={styles.rowSkeleton} />
      <Skeleton className={styles.rowSkeleton} />
    </Card>
  );
}

function isPaymentFresh(createdAt: string): boolean {
  return Date.now() - new Date(createdAt).getTime() < PAYMENT_STALE_MS;
}

export type PaymentDetailProps = {
  readonly id: string;
};

export function PaymentDetail({ id }: PaymentDetailProps): JSX.Element {
  const { data: payment, isPending, isError, refetch } = useSubscriptionPayment(id);
  const router = useRouter();

  if (isError) {
    return (
      <div className={styles.error}>
        <p className={styles.errorText}>
          Не удалось загрузить операцию
        </p>
        <Button onClick={() => void refetch()} variant="secondary">
          Повторить
        </Button>
      </div>
    );
  }

  if (isPending) {
    return <PaymentDetailSkeleton />;
  }

  if (!payment) {
    return (
      <div className={styles.error}>
        <p className={styles.errorText}>Операция не найдена</p>
        <Button onClick={() => router.back()} variant="secondary">
          Назад
        </Button>
      </div>
    );
  }

  const tariffName = getTariffLabel(payment.tariff.name);

  return (
    <Card className={styles.card}>
      <dl className={styles.details}>
        <div className={styles.row}>
          <dt className={styles.label}>Сумма</dt>
          <dd className={styles.value}>
            {formatMoneyKopecks(payment.amountKopecks)}
          </dd>
        </div>

        <div className={styles.row}>
          <dt className={styles.label}>Тариф</dt>
          <dd className={styles.value}>{tariffName}</dd>
        </div>

        <div className={styles.row}>
          <dt className={styles.label}>Период</dt>
          <dd className={styles.value}>{PAYMENT_PERIOD_LABELS[payment.period]}</dd>
        </div>

        <div className={styles.row}>
          <dt className={styles.label}>Статус</dt>
          <dd
            className={clsx(
              styles.status,
              styles[payment.status],
            )}
          >
            {PAYMENT_STATUS_LABELS[payment.status]}
          </dd>
        </div>

        <div className={styles.row}>
          <dt className={styles.label}>Дата</dt>
          <dd className={styles.value}>{formatDate(payment.createdAt)}</dd>
        </div>

        <div className={styles.row}>
          <dt className={styles.label}>Провайдер</dt>
          <dd className={styles.value}>{payment.provider}</dd>
        </div>
      </dl>

      {payment.status === 'pending' &&
        payment.paymentUrl &&
        isPaymentFresh(payment.createdAt) && (
          <a
            href={payment.paymentUrl}
            className={styles.paymentLink}
            target="_blank"
            rel="noopener noreferrer"
          >
            Вернуться к оплате
          </a>
        )}
    </Card>
  );
}
