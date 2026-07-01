'use client';

import {
  useCallback,
  useId,
  useState,
  type JSX,
} from 'react';
import { useRouter } from 'next/navigation';
import clsx from 'clsx';
import { Card } from '@heroui/react/card';
import { Skeleton } from '@heroui/react/skeleton';
import { notify } from '@/shared/lib/toast';
import { Button } from '@/shared/ui/button';
import {
  useTariffs,
  useSubscription,
  useChangeTariff,
} from '@/features/billing/api/hooks';
import { getTariffLabel } from '@/entities/user/lib/get-tariff-label';
import { type TariffName } from '@/entities/user/model/types';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { ROUTES } from '@/shared/config/routes';
import { ApiError } from '@/shared/api/errors';
import styles from './TariffChangeForm.module.css';

type Period = 'month' | 'year';

const PERIODS: { value: Period; label: string }[] = [
  { value: 'month', label: 'Месяц' },
  { value: 'year', label: 'Год' },
];

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

export function TariffChangeForm(): JSX.Element {
  const router = useRouter();
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
  const changeTariff = useChangeTariff();

  const [period, setPeriod] = useState<Period>('month');
  const [selectedTariff, setSelectedTariff] = useState<TariffName | null>(null);

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

  const handleSelect = useCallback(
    (tariffName: TariffName) => {
      setSelectedTariff(tariffName);

      const promise = changeTariff.mutateAsync({ tariffName, period });

      void notify.promise(promise, {
        loading: 'Меняем тариф...',
        success: 'Тариф изменён',
        error: (error) =>
          (error as ApiError).detail ?? 'Не удалось сменить тариф',
      });

      promise
        .then((data) => {
          if (data?.confirmUrl) {
            window.location.href = data.confirmUrl;
            return;
          }

          return router.push(ROUTES.profileTariffChangeSuccess);
        })
        .finally(() => {
          setSelectedTariff(null);
        })
        .catch(() => {}); // error is already reported by notify.promise
    },
    [changeTariff, period, router],
  );

  if (isError) {
    return (
      <div className={styles.error}>
        <p className={styles.errorText}>Не удалось загрузить данные тарифов</p>
        <Button onClick={handleRetry} variant="secondary">
          Повторить
        </Button>
      </div>
    );
  }

  if (isPending || !tariffs || !subscription) {
    return <TariffChangeSkeleton />;
  }

  const currentTariffName = subscription.tariff.name;

  return (
    <div className={styles.root}>
      <PeriodSelector
        value={period}
        onChange={setPeriod}
        disabled={changeTariff.isPending}
      />

      <div className={styles.list}>
        {tariffs.map((tariff) => {
          const isCurrent = tariff.name === currentTariffName;
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
                variant={isCurrent ? 'secondary' : 'primary'}
                size="large"
                fullWidth
                loading={isLoading}
                disabled={isCurrent || changeTariff.isPending}
                onClick={() => handleSelect(tariff.name)}
              >
                {isCurrent ? 'Текущий' : 'Выбрать'}
              </Button>
            </Card>
          );
        })}
      </div>
    </div>
  );
}
