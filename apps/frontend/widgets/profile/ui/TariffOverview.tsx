'use client';

import {type JSX, useCallback, useEffect, useRef} from 'react';
import NextLink from 'next/link';
import clsx from 'clsx';
import {useQueryClient} from '@tanstack/react-query';
import {Card} from '@heroui/react/card';
import {Skeleton} from '@/shared/ui/design';
import {notify} from '@/shared/lib/notifications';
import {Button} from '@/shared/ui/button';
import {LinkButton} from '@/shared/ui/link-button';
import {
    PAYMENT_STALE_MS,
    useCancelSubscription,
    usePendingPayment,
    useSubscription,
} from '@/features/billing';
import {billingKeys} from '@/shared/api/query-keys';
import {ROUTES} from '@/shared/config/routes';
import {getTariffLabel} from '@/entities/user';
import {isPaidTariff} from '@/entities/user';
import {PAYMENT_PERIOD_LABELS} from '@/entities/billing';
import {formatMoneyKopecks} from '@/shared/lib/format-money';
import {formatDate} from '@/shared/lib/format-date';
import styles from './TariffOverview.module.css';

const STATUS_LABELS: Record<
    'active' | 'grace' | 'cancelled',
    string
> = {
    active: 'Активна',
    grace: 'Льготный период',
    cancelled: 'Отменена',
};

function isExpiringSoon(validUntil?: string): boolean {
    if (!validUntil) return false;
    const diffMs = new Date(validUntil).getTime() - Date.now();
    const diffDays = diffMs / (1000 * 60 * 60 * 24);
    return diffDays >= 0 && diffDays <= 7;
}

function isPaymentStale(createdAt: string): boolean {
    return Date.now() - new Date(createdAt).getTime() > PAYMENT_STALE_MS;
}

/** Скелетон обзора тарифа — экспорт для route-loading (#609). */
export function TariffOverviewSkeleton(): JSX.Element {
    return (
        <Card className={styles.card}>
            <Skeleton className="h-6 w-2/5 rounded-lg"/>
            <Skeleton className="h-5 w-3/5 rounded-md"/>
            <Skeleton className="h-[18px] w-full rounded-md"/>
            <Skeleton className="h-[18px] w-[70%] rounded-md"/>
            <Skeleton className="h-[18px] w-[80%] rounded-md"/>
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
    const {data: pendingPayment} = usePendingPayment();
    const queryClient = useQueryClient();
    const cancel = useCancelSubscription();
    const hadPendingPaymentRef = useRef(false);

    useEffect(() => {
        if (pendingPayment) {
            hadPendingPaymentRef.current = true;
            return;
        }
        if (hadPendingPaymentRef.current) {
            hadPendingPaymentRef.current = false;
            void queryClient.invalidateQueries({
                queryKey: billingKeys.subscription,
            });
        }
    }, [pendingPayment, queryClient]);

    const handleCancel = useCallback(() => {
        void notify.scenarios.tariff.subscriptionCanceled(cancel.mutateAsync(undefined));
    }, [cancel]);

    if (isError) {
        return (
            <div className={styles.error}>
                <p className={styles.errorText}>Не удалось загрузить данные тарифа</p>
                <Button onClick={() => void refetch()} variant="secondary">
                    Повторить
                </Button>
            </div>
        );
    }

    if (isPending) {
        return <TariffOverviewSkeleton/>;
    }

    const isPaid = isPaidTariff(subscription.tariff.name);
    const isCancelled = subscription.status === 'cancelled';
    const isGrace = subscription.status === 'grace';
    const isPendingPaymentStale = pendingPayment
        ? isPaymentStale(pendingPayment.createdAt)
        : false;

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
                            {isPaid ? formatDate(subscription.validUntil) : 'Навсегда'}
                        </dd>
                    </div>

                    <div className={styles.row}>
                        <dt className={styles.label}>Автопродление</dt>
                        <dd className={styles.value}>
                            {subscription.autoRenewEnabled ? 'Включено' : 'Отключено'}
                        </dd>
                    </div>
                </dl>

            </Card>

            {pendingPayment && (
                <div className={clsx(styles.banner, styles.bannerInfo)}>
                    <p className={styles.bannerText}>
                        {isPendingPaymentStale
                            ? 'Платёж обрабатывается дольше обычного, мы автоматически проверяем статус у банка'
                            : `Ожидаем оплату: ${getTariffLabel(pendingPayment.tariff.name)}, ${PAYMENT_PERIOD_LABELS[pendingPayment.period]} — ${formatMoneyKopecks(pendingPayment.amountKopecks)}. Если вы ещё не завершили оплату, вернитесь на страницу банка.`}
                    </p>
                    <NextLink
                        href={ROUTES.profilePaymentDetail(pendingPayment.id)}
                        className={styles.bannerLink}
                    >
                        Детали платежа
                    </NextLink>
                    {!isPendingPaymentStale && pendingPayment.paymentUrl && (
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

            {subscription.pendingTariff && subscription.pendingChangeAt && (
                <div className={styles.banner}>
                    С {formatDate(subscription.pendingChangeAt)} тариф изменится на{" "}
                    {getTariffLabel(subscription.pendingTariff.name)}.
                </div>
            )}

            {isGrace && subscription.validUntil && (
                <div className={clsx(styles.banner, styles.bannerWarning)}>
                    <p className={styles.bannerText}>
                        Автопродление не прошло. Продлите подписку — доступ
                        сохранится до {formatDate(subscription.validUntil)}.
                    </p>
                    <NextLink
                        href={ROUTES.profileTariffChange}
                        className={styles.bannerLink}
                    >
                        Продлить
                    </NextLink>
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
                    disabled={Boolean(pendingPayment)}
                >
                    Сменить тариф
                </LinkButton>

                {pendingPayment && (
                    <p className={styles.hint}>
                        Дождитесь завершения текущего платежа
                    </p>
                )}

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
                    История платежей
                </LinkButton>
            </nav>
        </div>
    );
}
