'use client';

import type {JSX} from 'react';
import clsx from 'clsx';
import {formatMoneyKopecks} from '@/shared/lib/format-money';
import {formatCountdownLabel} from '@/shared/lib/format-countdown';
import {
    currentMonthIndex,
    diffDays,
    formatLeaseMonthOrdinal,
    formatOverdue,
    monthPeriodProgress,
    parseLocalDate,
    startOfDay,
} from '@/shared/lib/lease-payment';
import type {Lease} from '@/shared/model/lease';
import styles from './LeaseInfo.module.css';

const SEGMENTS = 4;

const toneStyles = {
    primary: {fill: undefined, marker: undefined},
    danger: {fill: styles.fillDanger, marker: styles.markerDanger},
    success: {fill: styles.fillSuccess, marker: styles.markerSuccess},
} as const;

export type LeaseInfoLease = Pick<
    Lease,
    | 'startDate'
    | 'paymentDay'
    | 'rentKopecks'
    | 'currentPeriodOverdue'
    | 'hasOverdue'
    | 'overdueSince'
    | 'nextPaymentDate'
    | 'status'
> & {
    readonly tenantContact?: {readonly name: string} | null;
};

export type LeaseInfoProps = {
    readonly lease: LeaseInfoLease;
    readonly className?: string;
};

export function LeaseInfo({
                              lease,
                              className,
                          }: LeaseInfoProps): JSX.Element {
    const input = {startDate: lease.startDate, paymentDay: lease.paymentDay};
    const now = new Date();
    const today = startOfDay(now);

    let paymentLabel: string;
    let progressRatio: number;
    let progressTone: 'primary' | 'danger' | 'success' = 'primary';

    if (lease.hasOverdue && lease.overdueSince) {
        paymentLabel = formatOverdue(diffDays(parseLocalDate(lease.overdueSince), today));
        progressRatio = 1;
        progressTone = 'danger';
    } else if (lease.nextPaymentDate) {
        const days = diffDays(today, parseLocalDate(lease.nextPaymentDate));
        const label = days === 0 ? 'Сегодня' : formatCountdownLabel(days);
        if (lease.status === 'awaiting_start') {
            paymentLabel = `Начнётся ${label.toLowerCase()}`;
        } else {
            paymentLabel = label;
        }
        progressRatio = days === 0
            ? 1
            : monthPeriodProgress(input, now);
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
                    {formatMoneyKopecks(lease.rentKopecks)}
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
                    style={{left: `calc(${progressRatio * 100}% - ${progressRatio * 12}px)`}}
                />
            </div>

            <div className={styles.row}>
                <span className={styles.tenant}>
                    {lease.tenantContact?.name ?? 'Арендатор не указан'}
                </span>
                <span className={styles.muted}>
                    {formatLeaseMonthOrdinal(currentMonthIndex(input, now))}
                </span>
            </div>
        </div>
    );
}
