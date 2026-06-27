'use client';

import { useCallback, type JSX } from 'react';
import clsx from 'clsx';
import { Card } from '@heroui/react/card';
import { Switch } from '@heroui/react/switch';
import { Skeleton } from '@heroui/react/skeleton';
import { toast } from 'react-toastify';
import { Button } from '@/shared/ui/button';
import { LinkButton } from '@/shared/ui/link-button';
import {
  useSubscription,
  useToggleAutoRenew,
  useCancelSubscription,
} from '@/features/billing/api/hooks';
import { ROUTES } from '@/shared/config/routes';
import { getTariffLabel } from '@/entities/user/lib/get-tariff-label';
import { isPaidTariff } from '@/entities/user/lib/is-paid-tariff';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { formatDate } from '@/shared/lib/format-date';
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
  const toggle = useToggleAutoRenew();
  const cancel = useCancelSubscription();

  const handleToggle = useCallback(
    (enabled: boolean) => {
      if (!subscription) {
        return;
      }

      toggle.mutate(
        { enabled },
        {
          onError: (error) => {
            toast.error(
              error.detail || 'Не удалось изменить настройки автопродления',
            );
          },
        },
      );
    },
    [subscription, toggle],
  );

  const handleCancel = useCallback(() => {
    cancel.mutate(undefined, {
      onSuccess: () => {
        toast.success('Подписка отменена');
      },
      onError: (error) => {
        toast.error(error.detail || 'Не удалось отменить подписку');
      },
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

  return (
    <div className={styles.root}>
      <Card className={styles.card}>
        <h2 className={styles.tariffName}>
          {getTariffLabel(subscription.tariff.name)}
        </h2>

        <div className={styles.priceRow}>
          <span className={styles.price}>
            {formatMoneyKopecks(subscription.tariff.monthlyPriceKopecks)}
            <span className={styles.period}>/мес</span>
          </span>
          <span className={styles.price}>
            {formatMoneyKopecks(subscription.tariff.yearlyPriceKopecks)}
            <span className={styles.period}>/год</span>
          </span>
        </div>

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
              {subscription.validUntil
                ? formatDate(subscription.validUntil)
                : '—'}
            </dd>
          </div>

          <div className={styles.row}>
            <dt className={styles.label}>Автопродление</dt>
            <dd className={styles.switchWrapper}>
              <Switch
                aria-label="Автопродление"
                isSelected={subscription.autoRenewEnabled}
                onChange={handleToggle}
                isDisabled={toggle.isPending || isCancelled}
                size="sm"
              />
            </dd>
          </div>
        </dl>
      </Card>

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
