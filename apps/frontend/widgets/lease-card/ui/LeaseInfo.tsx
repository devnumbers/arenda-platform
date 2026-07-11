'use client';

import type {JSX} from 'react';
import clsx from 'clsx';
import {formatMoneyKopecks} from '@/shared/lib/format-money';
import {
    currentMonthIndex,
    diffDays,
    formatLeaseMonthOrdinal,
    formatOverdue,
    formatPaymentCountdown,
    parseLocalDate,
    progressToPaymentDate,
    startOfDay,
} from '@/shared/lib/lease-payment';
import type {components} from '@/shared/api/generated';
import styles from './LeaseInfo.module.css';

const SEGMENTS = 4;

const toneStyles = {
    primary: {fill: undefined, marker: undefined},
    danger: {fill: styles.fillDanger, marker: styles.markerDanger},
    success: {fill: styles.fillSuccess, marker: styles.markerSuccess},
} as const;

type LeaseResponse = components['schemas']['LeaseResponse'];

export type LeaseInfoLease = Pick<
    LeaseResponse,
    | 'start_date'
    | 'payment_day'
    | 'rent_amount_kopecks'
    | 'current_period_overdue'
    | 'has_overdue'
    | 'overdue_since'
    | 'next_payment_date'
> & {
    readonly tenant_contact?: {readonly name: string} | null;
};

export type LeaseInfoProps = {
    readonly lease: LeaseInfoLease;
    readonly className?: string;
};

export function LeaseInfo({
                              lease,
                              className,
                          }: LeaseInfoProps): JSX.Element {
    const input = {start_date: lease.start_date, payment_day: lease.payment_day};
    const now = new Date();
    const today = startOfDay(now);

    let paymentLabel: string;
    let progressRatio: number;
    let progressTone: 'primary' | 'danger' | 'success' = 'primary';

    if (lease.has_overdue && lease.overdue_since) {
        paymentLabel = formatOverdue(diffDays(parseLocalDate(lease.overdue_since), today));
        progressRatio = 1;
        progressTone = 'danger';
    } else if (lease.next_payment_date) {
        const days = diffDays(today, parseLocalDate(lease.next_payment_date));
        if (days === 0) {
            paymentLabel = 'Сегодня';
            progressRatio = 1;
        } else {
            paymentLabel = formatPaymentCountdown({kind: 'future', days});
            progressRatio = progressToPaymentDate(input, parseLocalDate(lease.next_payment_date), now);
        }
    } else {
        paymentLabel = 'Все оплачено';
        progressRatio = 1;
        progressTone = 'success';
    }

    const segmentProgress = progressRatio * SEGMENTS;
    const tone = toneStyles[progressTone];

    return (
        <div className={clsx(styles.root, className)}>
            <div className={styles.row}>
                <span className={styles.amount}>
                    {formatMoneyKopecks(lease.rent_amount_kopecks)}
                </span>
                <span className={styles.muted}>{paymentLabel}</span>
            </div>

            <div className={styles.progress}>
                {Array.from({length: SEGMENTS}).map((_, index) => {
                    const fill = Math.min(1, Math.max(0, segmentProgress - index)) * 100;
                    return (
                        <div key={index} className={styles.track}>
                            {fill > 0 && (
                                <div
                                    className={clsx(styles.fill, tone.fill)}
                                    style={{width: `${fill}%`}}
                                />
                            )}
                        </div>
                    );
                })}
                <div
                    className={clsx(styles.marker, tone.marker)}
                    style={{left: `calc(${progressRatio * 100}% - 6px)`}}
                />
            </div>

            <div className={styles.row}>
                <span className={styles.tenant}>
                    {lease.tenant_contact?.name ?? 'Арендатор не указан'}
                </span>
                <span className={styles.muted}>
                    {formatLeaseMonthOrdinal(currentMonthIndex(input, now))}
                </span>
            </div>
        </div>
    );
}
