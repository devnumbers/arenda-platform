'use client';

import type { JSX } from 'react';
import NextLink from 'next/link';
import { Card } from '@heroui/react/card';
import { Skeleton } from '@/shared/ui/design';
import { Button } from '@/shared/ui/button';
import {
  useSubscriptionPayments,
} from '@/features/billing';
import {
  PAYMENT_PERIOD_LABELS,
  PAYMENT_STATUS_LABELS,
} from '@/entities/billing';
import type { PaymentPeriod, PaymentStatus } from '@/entities/billing';
import { ROUTES } from '@/shared/config/routes';
import { getTariffLabel } from '@/entities/user';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { formatDate } from '@/shared/lib/format-date';
import type { SubscriptionPayment } from '@/entities/billing';
import styles from './PaymentList.module.css';

export type PaymentListItem = {
  readonly id: string;
  readonly tariffName: string;
  readonly period: PaymentPeriod;
  readonly amountKopecks: number;
  readonly status: PaymentStatus;
  readonly provider: string;
  readonly createdAt: string;
};

function toViewModel(payment: SubscriptionPayment): PaymentListItem {
  return {
    id: payment.id,
    tariffName: getTariffLabel(payment.tariff.name),
    period: payment.period,
    amountKopecks: payment.amountKopecks,
    status: payment.status,
    provider: payment.provider,
    createdAt: payment.createdAt,
  };
}

/** Скелетон истории платежей — экспорт для route-loading (#609). */
export function PaymentListSkeleton(): JSX.Element {
  return (
    <div className={styles.list}>
      {[1, 2].map((key) => (
        <Card key={key} className={styles.card}>
          <Skeleton className="h-6 w-1/2 rounded-lg" />
          <Skeleton className="h-[18px] w-[35%] rounded-md" />
        </Card>
      ))}
    </div>
  );
}

function PaymentCard({ payment }: { readonly payment: PaymentListItem }): JSX.Element {
  return (
    <NextLink
      href={ROUTES.profilePaymentDetail(payment.id)}
      className={styles.cardLink}
      aria-label={`Оплата от ${formatDate(payment.createdAt)}`}
    >
      <Card className={styles.card}>
        <div className={styles.cardHeader}>
          <span className={styles.amount}>
            {formatMoneyKopecks(payment.amountKopecks)}
          </span>
          <span className={styles.status}>{PAYMENT_STATUS_LABELS[payment.status]}</span>
        </div>

        <p className={styles.meta}>
          {payment.tariffName}
          {' · '}
          {PAYMENT_PERIOD_LABELS[payment.period]}
        </p>

        <p className={styles.date}>{formatDate(payment.createdAt)}</p>
      </Card>
    </NextLink>
  );
}

export function PaymentList(): JSX.Element {
  const { data: payments, isPending, isError, refetch } = useSubscriptionPayments();

  if (isError) {
    return (
      <div className={styles.error}>
        <p className={styles.errorText}>
          Не удалось загрузить историю платежей
        </p>
        <Button onClick={() => void refetch()} variant="secondary">
          Повторить
        </Button>
      </div>
    );
  }

  if (isPending) {
    return <PaymentListSkeleton />;
  }

  const items = payments.map(toViewModel);

  if (items.length === 0) {
    return (
      <div className={styles.empty}>
        <p className={styles.emptyText}>У вас пока нет платежей</p>
      </div>
    );
  }

  return (
    <div className={styles.root}>
      <div className={styles.list}>
        {items.map((payment) => (
          <PaymentCard key={payment.id} payment={payment} />
        ))}
      </div>
    </div>
  );
}
