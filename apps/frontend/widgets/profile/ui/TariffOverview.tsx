'use client';

import { useCallback, type JSX } from 'react';
import clsx from 'clsx';
import { Card } from '@heroui/react/card';
import { Skeleton } from '@heroui/react/skeleton';
import { notify } from '@/shared/lib/toast';
import { Button } from '@/shared/ui/button';
import { LinkButton } from '@/shared/ui/link-button';
import {
  useSubscription,
  useCancelSubscription,
} from '@/features/billing/api/hooks';
import { ROUTES } from '@/shared/config/routes';
import { getTariffLabel } from '@/entities/user/lib/get-tariff-label';
import { isPaidTariff } from '@/entities/user/lib/is-paid-tariff';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { formatDate } from '@/shared/lib/format-date';
import type { Subscription } from '@/entities/billing/model/types';
import { ApiError } from '@/shared/api/errors';
import styles from './TariffOverview.module.css';

const STATUS_LABELS: Record<
  'active' | 'grace' | 'blocked' | 'cancelled',
  string
> = {
  active: 'Активна',
  grace: 'Льготный период',
  blocked: 'Заблокирована',
  cancelled: 'Отменена',
};

function getAutoRenewState(
  subscription: Subscription,
): 'enabled' | 'disabled' | 'no-card' {
  if (!subscription.autoRenewEnabled) return 'disabled';
  if (!subscription.activePaymentMethod) return 'no-card';
  return 'enabled';
}

function isExpiringSoon(validUntil?: string): boolean {
  if (!validUntil) return false;
  const diffMs = new Date(validUntil).getTime() - Date.now();
  const diffDays = diffMs / (1000 * 60 * 60 * 24);
  return diffDays >= 0 && diffDays <= 7;
}

function TariffOverviewSkeleton(): JSX.Element {
  return (
    <Card className={styles.card}>
      <Skeleton className={styles.nameSkeleton} />
      <Skeleton className={styles.priceSkeleton} />
      <Skeleton className={styles.rowSkeleton} />
      <Skeleton className={styles.rowSkeleton} />
      <Skeleton className={styles.rowSkeleton} />
    </Card>
  );
}

export function TariffOverview(): JSX.Element {
  const {
    data: subscription,
    isPending,
    isError,
    refetch,
  } = useSubscription();
  const cancel = useCancelSubscription();

  const handleCancel = useCallback(() => {
    void notify.promise(cancel.mutateAsync(undefined), {
      loading: 'Отменяем подписку...',
      success: 'Подписка отменена',
      error: (error) =>
        (error as ApiError).detail ?? 'Не удалось отменить подписку',
    });
  }, [cancel]);

  if (isError) {
    return (
      <div className={styles.error}>
        <p className={styles.errorText}>Не удалось загрузить данные тарифа</p>
        <Button onClick={() => refetch()} variant="secondary">
          Повторить
        </Button>
      </div>
    );
  }

  if (isPending || !subscription) {
    return <TariffOverviewSkeleton />;
  }

  const isPaid = isPaidTariff(subscription.tariff.name);
  const isCancelled = subscription.status === 'cancelled';

  const priceDisplay = (() => {
    if (subscription.currentPeriod === 'month') {
      return (
        <>
          {formatMoneyKopecks(subscription.tariff.monthlyPriceKopecks)}
          <span className={styles.period}>/мес</span>
        </>
      );
    }

    if (subscription.currentPeriod === 'year') {
      return (
        <>
          {formatMoneyKopecks(subscription.tariff.yearlyPriceKopecks)}
          <span className={styles.period}>/год</span>
        </>
      );
    }

    if (isPaid) {
      return (
        <>
          {formatMoneyKopecks(subscription.tariff.monthlyPriceKopecks)}
          <span className={styles.period}>/мес</span>
        </>
      );
    }

    return 'Бесплатно';
  })();

  return (
    <div className={styles.root}>
      <Card className={styles.card}>
        <h2 className={styles.tariffName}>
          {getTariffLabel(subscription.tariff.name)}
        </h2>

        <div className={styles.price}>{priceDisplay}</div>

        <dl className={styles.details}>
          <div className={styles.row}>
            <dt className={styles.label}>Статус</dt>
            <dd
              className={clsx(
                styles.status,
                styles[subscription.status],
              )}
            >
              {STATUS_LABELS[subscription.status]}
            </dd>
          </div>

          <div className={styles.row}>
            <dt className={styles.label}>Действует до</dt>
            <dd className={styles.value}>
              {formatDate(subscription.validUntil)}
            </dd>
          </div>

          <div className={styles.row}>
            <dt className={styles.label}>Автопродление</dt>
            <dd className={styles.value}>
              {subscription.autoRenewEnabled ? 'Включено' : 'Отключено'}
            </dd>
          </div>
        </dl>

        {getAutoRenewState(subscription) === 'no-card' &&
          (subscription.status === 'active' || subscription.status === 'grace') && (
            <p className={styles.hint}>
              Для автопродления нужна основная карта.{" "}
              <LinkButton
                href={ROUTES.profilePaymentMethods}
                variant="clear"
                size="tiny"
                className={styles.hintLink}
              >
                Добавить
              </LinkButton>
            </p>
          )}
      </Card>

      {subscription.pendingTariff && subscription.pendingChangeAt && (
        <div className={styles.banner}>
          С {formatDate(subscription.pendingChangeAt)} тариф изменится на{" "}
          {getTariffLabel(subscription.pendingTariff.name)}.
        </div>
      )}

      {isExpiringSoon(subscription.validUntil) && (
        <div className={clsx(styles.banner, styles.bannerWarning)}>
          Подписка закончится {formatDate(subscription.validUntil)}.
        </div>
      )}

      <div className={styles.actions}>
        <LinkButton
          href={ROUTES.profileTariffChange}
          variant="primary"
          size="large"
          fullWidth
        >
          Сменить тариф
        </LinkButton>

        {isPaid && !isCancelled && (
          <Button
            variant="secondary"
            size="large"
            fullWidth
            onClick={handleCancel}
            loading={cancel.isPending}
          >
            Отменить подписку
          </Button>
        )}
      </div>

      <nav className={styles.links} aria-label="Управление оплатой">
        <LinkButton
          href={ROUTES.profilePaymentMethods}
          variant="secondary"
          size="large"
          fullWidth
        >
          Способы оплаты
        </LinkButton>
        <LinkButton
          href={ROUTES.profilePayments}
          variant="secondary"
          size="large"
          fullWidth
        >
          История операций
        </LinkButton>
      </nav>
    </div>
  );
}
