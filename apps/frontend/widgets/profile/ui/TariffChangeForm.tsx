'use client';

import {
  useCallback,
  useId,
  useState,
  type JSX,
} from 'react';
import { useRouter } from 'next/navigation';
import NextLink from 'next/link';
import clsx from 'clsx';
import { Card } from '@heroui/react/card';
import { Skeleton } from '@heroui/react/skeleton';
import { notify } from '@/shared/lib/notifications';
import { Button } from '@/shared/ui/button';
import { PageHeader } from '@/shared/ui/page-header';
import {
  PAYMENT_STALE_MS,
  useTariffs,
  useSubscription,
  useChangeTariff,
  usePendingPayment,
} from '@/features/billing';
import { getTariffLabel } from '@/entities/user';
import { isPaidTariff } from '@/entities/user';
import { type TariffName } from '@/entities/user';
import type {
  Subscription,
  SubscriptionPayment,
  Tariff,
} from '@/entities/billing';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { ROUTES } from '@/shared/config/routes';
import type { ApiError } from '@/shared/api/errors';
import styles from './TariffChangeForm.module.css';

type Period = 'month' | 'year';

const PERIODS: { value: Period; label: string }[] = [
  { value: 'month', label: 'Месяц' },
  { value: 'year', label: 'Год' },
];

function isPaymentFresh(createdAt: string): boolean {
  return Date.now() - new Date(createdAt).getTime() < PAYMENT_STALE_MS;
}

type PeriodSelectorProps = {
  readonly value: Period;
  readonly onChange: (period: Period) => void;
  readonly disabled?: boolean;
};

function PeriodSelector({
  value,
  onChange,
  disabled,
}: PeriodSelectorProps): JSX.Element {
  const labelId = useId();

  return (
    <div
      className={styles.periodSelector}
      role="radiogroup"
      aria-labelledby={labelId}
    >
      <span id={labelId} className={styles.periodLabel}>
        Период оплаты
      </span>
      <div className={styles.periodOptions}>
        {PERIODS.map((option) => {
          const isSelected = option.value === value;
          return (
            <button
              key={option.value}
              type="button"
              role="radio"
              aria-checked={isSelected}
              disabled={disabled}
              className={clsx(
                styles.periodOption,
                isSelected && styles.periodOptionSelected,
              )}
              onClick={() => onChange(option.value)}
            >
              {option.label}
            </button>
          );
        })}
      </div>
    </div>
  );
}

function TariffChangeSkeleton(): JSX.Element {
  return (
    <div className={styles.list}>
      {[1, 2, 3].map((key) => (
        <Card key={key} className={styles.card}>
          <Skeleton className={styles.nameSkeleton} />
          <Skeleton className={styles.priceSkeleton} />
          <Skeleton className={styles.limitSkeleton} />
          <Skeleton className={styles.buttonSkeleton} />
        </Card>
      ))}
    </div>
  );
}

type TariffChangeContentProps = {
  readonly tariffs: Tariff[];
  readonly subscription: Subscription;
  readonly pendingPayment: SubscriptionPayment | undefined;
};

function TariffChangeContent({
  tariffs,
  subscription,
  pendingPayment,
}: TariffChangeContentProps): JSX.Element {
  const router = useRouter();
  const changeTariff = useChangeTariff();

  // Переключатель периода стартует с периода текущей подписки
  const [period, setPeriod] = useState<Period>(
    subscription.currentPeriod ?? 'month',
  );
  const [selectedTariff, setSelectedTariff] = useState<TariffName | null>(null);

  const hasPendingPayment = Boolean(pendingPayment);

  // На странице смены тарифа показываются только платные тарифы;
  // возврат на бесплатный basic — через «Отменить подписку» на странице «Тариф»
  const paidTariffs = tariffs.filter((tariff) => isPaidTariff(tariff.name));

  const handleSelect = useCallback(
    (tariffName: TariffName) => {
      setSelectedTariff(tariffName);

      const loadingToastId = notify.scenarios.tariff.changeLoading();

      changeTariff
        .mutateAsync({ tariffName, period })
        .then((data) => {
          notify.close(loadingToastId);

          if (data?.confirmUrl) {
            // Upgrade: платёж только создан (pending) — редирект на оплату
            // без success-toast; тариф применится по webhook CONFIRMED.
            window.location.href = data.confirmUrl;
            return;
          }

          notify.scenarios.tariff.changed();
          router.replace(ROUTES.profileTariffChangeSuccess);
        })
        .catch((error: unknown) => {
          notify.close(loadingToastId);
          notify.scenarios.tariff.changeError({description: (error as ApiError).detail});
        })
        .finally(() => {
          setSelectedTariff(null);
        });
    },
    [changeTariff, period, router],
  );

  return (
    <div className={styles.root}>
      {pendingPayment && (
        <div className={clsx(styles.banner, styles.bannerInfo)}>
          <p className={styles.bannerText}>
            У вас есть платёж в обработке — дождитесь его завершения,
            чтобы сменить тариф
          </p>
          <NextLink
            href={ROUTES.profilePaymentDetail(pendingPayment.id)}
            className={styles.bannerLink}
          >
            Детали платежа
          </NextLink>
          {pendingPayment.paymentUrl &&
            isPaymentFresh(pendingPayment.createdAt) && (
              <a
                href={pendingPayment.paymentUrl}
                className={styles.bannerLink}
                target="_blank"
                rel="noopener noreferrer"
              >
                Вернуться к оплате
              </a>
            )}
        </div>
      )}

      <PeriodSelector
        value={period}
        onChange={setPeriod}
        disabled={changeTariff.isPending || hasPendingPayment}
      />

      <div className={styles.list}>
        {paidTariffs.map((tariff) => {
          const isCurrent =
            tariff.name === subscription.tariff.name &&
            period === subscription.currentPeriod;
          // В grace период текущий тариф можно продлить повторной оплатой
          const isRenewable = isCurrent && subscription.status === 'grace';
          const isLoading =
            changeTariff.isPending && selectedTariff === tariff.name;

          return (
            <Card
              key={tariff.name}
              className={clsx(styles.card, isCurrent && styles.currentCard)}
            >
              <div className={styles.cardHeader}>
                <h3 className={styles.tariffName}>
                  {getTariffLabel(tariff.name)}
                </h3>
                {isCurrent && (
                  <span className={styles.currentBadge}>Текущий</span>
                )}
              </div>

              <div className={styles.priceRow}>
                <span className={styles.price}>
                  {formatMoneyKopecks(
                    period === 'year'
                      ? tariff.yearlyPriceKopecks
                      : tariff.monthlyPriceKopecks,
                  )}
                  <span className={styles.period}>
                    {period === 'year' ? '/год' : '/мес'}
                  </span>
                </span>
              </div>

              <p className={styles.limit}>
                {tariff.activePropertyLimit < 0
                  ? 'Неограниченно'
                  : `До ${tariff.activePropertyLimit} объектов`}
              </p>

              <Button
                variant={isCurrent && !isRenewable ? 'secondary' : 'primary'}
                size="large"
                fullWidth
                loading={isLoading}
                disabled={
                  (isCurrent && !isRenewable) ||
                  changeTariff.isPending ||
                  hasPendingPayment
                }
                onClick={() => handleSelect(tariff.name)}
              >
                {isCurrent ? (isRenewable ? 'Продлить' : 'Текущий') : 'Выбрать'}
              </Button>
            </Card>
          );
        })}
      </div>
    </div>
  );
}

export function TariffChangeForm(): JSX.Element {
  const {
    data: tariffs,
    isPending: isTariffsPending,
    isError: isTariffsError,
    refetch: refetchTariffs,
  } = useTariffs();
  const {
    data: subscription,
    isPending: isSubscriptionPending,
    isError: isSubscriptionError,
    refetch: refetchSubscription,
  } = useSubscription();
  const { data: pendingPayment } = usePendingPayment();

  const isPending = isTariffsPending || isSubscriptionPending;
  const isError = isTariffsError || isSubscriptionError;

  const handleRetry = useCallback(() => {
    if (isTariffsError) {
      refetchTariffs();
    }
    if (isSubscriptionError) {
      refetchSubscription();
    }
  }, [isTariffsError, isSubscriptionError, refetchTariffs, refetchSubscription]);

  return (
    <>
      <PageHeader title="Сменить тариф" backHref={ROUTES.profileTariff} />
      {isError && (
        <div className={styles.error}>
          <p className={styles.errorText}>Не удалось загрузить данные тарифов</p>
          <Button onClick={handleRetry} variant="secondary">
            Повторить
          </Button>
        </div>
      )}
      {!isError && (isPending || !tariffs || !subscription) && (
        <TariffChangeSkeleton />
      )}
      {!isError && !isPending && tariffs && subscription && (
        <TariffChangeContent
          tariffs={tariffs}
          subscription={subscription}
          pendingPayment={pendingPayment}
        />
      )}
    </>
  );
}
