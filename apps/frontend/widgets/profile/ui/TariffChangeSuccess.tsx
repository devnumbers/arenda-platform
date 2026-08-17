'use client';

import { useEffect, type JSX } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { useQueryClient } from '@tanstack/react-query';
import clsx from 'clsx';
import { Loading, StatusDanger, StatusGood } from '@/shared/assets/icons';
import { Icon } from '@/shared/ui/icon';
import { Button } from '@/shared/ui/button';
import { LinkButton } from '@/shared/ui/link-button';
import {
  PAYMENT_STALE_MS,
  useSubscription,
  useSubscriptionPayment,
} from '@/features/billing';
import { billingKeys } from '@/shared/api/query-keys';
import { formatDate } from '@/shared/lib/format-date';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import styles from './TariffChangeSuccess.module.css';

function isPaymentStale(createdAt: string): boolean {
  return Date.now() - new Date(createdAt).getTime() > PAYMENT_STALE_MS;
}

export function TariffChangeSuccess(): JSX.Element {
  const router = useRouter();
  const searchParams = useSearchParams();
  const paymentId = searchParams.get('paymentId');
  const queryClient = useQueryClient();
  const { data: subscription, isPending } = useSubscription();
  const { data: payment, isPending: isPaymentPending } =
    useSubscriptionPayment(paymentId ?? '', Boolean(paymentId));

  useEffect(() => {
    if (payment?.status === 'succeeded') {
      void queryClient.invalidateQueries({ queryKey: billingKeys.subscription });
    }
  }, [payment?.status, queryClient]);

  if (paymentId) {
    if (isPaymentPending || payment?.status === 'pending') {
      const isStale = payment ? isPaymentStale(payment.createdAt) : false;

      return (
        <div className={styles.root}>
          <div className={styles.card}>
            <div className={styles.iconWrapper}>
              <Icon size="l" className={styles.spinner}>
                <Loading />
              </Icon>
            </div>

            <div className={styles.text}>
              <h2 className={styles.heading}>Платёж обрабатывается</h2>
              <p className={styles.subtext}>
                Обычно это занимает до минуты, страница обновится
                автоматически
              </p>
              {isStale && (
                <p className={styles.subtext}>
                  Проверяем статус у банка, это может занять несколько минут
                </p>
              )}
            </div>

            <Button
              variant="primary"
              size="large"
              fullWidth
              onClick={() => goBack(router, ROUTES.profileTariff)}
            >
              Вернуться к тарифу
            </Button>
          </div>
        </div>
      );
    }

    if (payment?.status === 'failed') {
      return (
        <div className={styles.root}>
          <div className={styles.card}>
            <div className={clsx(styles.iconWrapper, styles.iconError)}>
              <Icon size="l">
                <StatusDanger />
              </Icon>
            </div>

            <div className={styles.text}>
              <h2 className={styles.heading}>Оплата не прошла</h2>
              <p className={styles.subtext}>
                Попробуйте сменить тариф ещё раз
              </p>
            </div>

            <LinkButton
              href={ROUTES.profileTariffChange}
              variant="primary"
              size="large"
              fullWidth
            >
              Попробовать снова
            </LinkButton>
          </div>
        </div>
      );
    }
  }

  const pending = subscription?.pendingTariff;
  const pendingChangeAt = subscription?.pendingChangeAt;

  return (
    <div className={styles.root}>
      <div className={styles.card}>
        <div className={styles.iconWrapper}>
          <Icon size="l">
            <StatusGood />
          </Icon>
        </div>

        <div className={styles.text}>
          <h2 className={styles.heading}>Тариф изменен</h2>
          {isPending ? (
            <p className={styles.subtext}>Загрузка сведений о подписке...</p>
          ) : (
            <p className={styles.subtext}>
              {pending && pendingChangeAt
                ? `Изменения вступят в силу ${formatDate(pendingChangeAt)}.`
                : 'Тариф успешно изменен.'}
            </p>
          )}
        </div>

        <Button
          variant="primary"
          size="large"
          fullWidth
          onClick={() => goBack(router, ROUTES.profileTariff)}
        >
          Вернуться к тарифу
        </Button>
      </div>
    </div>
  );
}
