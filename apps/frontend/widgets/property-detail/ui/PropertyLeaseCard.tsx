'use client';

import type {JSX} from 'react';
import NextLink from 'next/link';
import {Icon} from '@/shared/ui/icon';
import {Button} from '@/shared/ui/button';
import {LinkButton} from '@/shared/ui/link-button';
import {ArrowRight} from '@/shared/assets/icons';
import {ROUTES} from '@/shared/config/routes';
import {formatMoneyKopecks} from '@/shared/lib/format-money';
import {formatCurrentLeaseMonth, formatLeaseRemainingDuration,} from '@/shared/lib/format-lease-card-values';
import {LeaseProgressBar} from '@/widgets/properties/ui/LeaseProgressBar';
import type {components} from '@/shared/api/generated';
import type {PropertyPageStatus} from '../lib/get-property-page-status';
import {formatAwaitingStart} from '../lib/format-awaiting-start';
import {formatOverdueCount} from '../lib/format-overdue-count';
import {PropertyDetailSection} from './PropertyDetailSection';
import {getEffectiveLeaseStatus} from '@/entities/lease/lib/status';
import styles from './PropertyLeaseCard.module.css';

type LeaseResponse = components['schemas']['LeaseResponse'];

export type PropertyLeaseCardProps = {
    readonly lease: LeaseResponse | undefined;
    readonly overdueRentCount: number;
    readonly status: PropertyPageStatus;
    readonly propertyId: string;
    readonly onPayRent?: () => void;
    readonly onEndLease?: () => void;
    readonly isPayRentLoading?: boolean;
};

export function PropertyLeaseCard({
                                      lease,
                                      overdueRentCount,
                                      status,
                                      propertyId,
                                      onPayRent,
                                      onEndLease,
                                      isPayRentLoading = false,
                                  }: PropertyLeaseCardProps): JSX.Element {
    const leaseNewHref = `${ROUTES.leaseNew}?propertyId=${propertyId}`;
    const effectiveLeaseStatus = lease
        ? getEffectiveLeaseStatus({
            status: lease.status,
            startDate: lease.start_date,
            endDate: lease.end_date ?? undefined,
        })
        : undefined;
    const progressEndDate = lease?.end_date ?? lease?.start_date;
    const remaining = lease
        ? formatLeaseRemainingDuration(lease.start_date, lease.end_date)
        : '';
    const monthLabel = lease
        ? effectiveLeaseStatus === 'awaiting_start'
            ? formatAwaitingStart(lease.start_date)
            : formatCurrentLeaseMonth(lease.start_date, effectiveLeaseStatus ?? lease.status)
        : '';
    const showRentActions = status === 'rented';
    const showResolveActions = status === 'requires_action' && lease;
    const showCreateAction = status === 'free';
    const showUnavailableState =
        status !== 'awaiting_start' &&
        !showRentActions &&
        !showResolveActions &&
        !showCreateAction;
    const showActions =
        showRentActions ||
        showResolveActions ||
        showCreateAction ||
        showUnavailableState;

    return (
        <PropertyDetailSection>
            <div className={styles.header}>
                <h2 className={styles.title}>Аренда</h2>
                <NextLink
                    href={ROUTES.propertyLeases(propertyId)}
                    className={styles.headerLink}
                    aria-label="Все аренды"
                >
                    <Icon size="s">
                        <ArrowRight/>
                    </Icon>
                </NextLink>
            </div>

            {overdueRentCount > 0 && (
                <span className={styles.overdueBadge}>
          {formatOverdueCount(overdueRentCount)}
        </span>
            )}

            {lease && progressEndDate ? (
                <NextLink href={ROUTES.lease(lease.id)} className={styles.card}>
                    <div className={styles.row}>
            <span className={styles.amount}>
              {formatMoneyKopecks(lease.rent_amount_kopecks)}
            </span>
                        <span className={styles.duration}>{remaining}</span>
                    </div>

                    <LeaseProgressBar
                        startDate={lease.start_date}
                        endDate={progressEndDate}
                        active={status === 'rented' || status === 'requires_action'}
                    />

                    <div className={styles.row}>
            <span className={styles.tenant}>
              {lease.tenant_contact?.name ?? 'Арендатор не указан'}
            </span>
                        {monthLabel && <span className={styles.month}>{monthLabel}</span>}
                    </div>
                </NextLink>
            ) : (
                <div className={styles.empty}>
                    <p className={styles.emptyText}>Объект свободен</p>
                </div>
            )}

            {showActions && (
                <div className={styles.actions}>
                    {showRentActions ? (
                        <>
                            {onPayRent ? (
                                <Button
                                    variant="primary"
                                    fullWidth
                                    onClick={onPayRent}
                                    loading={isPayRentLoading}
                                    type="button"
                                >
                                    Оплатить аренду
                                </Button>
                            ) : (
                                <LinkButton
                                    href={`${ROUTES.financeOperations}?property_id=${lease?.property_id}`}
                                    variant="primary"
                                    fullWidth
                                >
                                    Оплатить аренду
                                </LinkButton>
                            )}
                            <LinkButton
                                href={`${ROUTES.financeOperations}?property_id=${lease?.property_id}`}
                                variant="secondary"
                                fullWidth
                            >
                                Все операции
                            </LinkButton>
                        </>
                    ) : showResolveActions ? (
                        <>
                            <LinkButton href={ROUTES.lease(lease.id)} variant="primary" fullWidth>
                                Продлить текущую
                            </LinkButton>
                            <Button
                                variant="secondary"
                                fullWidth
                                onClick={onEndLease}
                                type="button"
                            >
                                Завершить аренду
                            </Button>
                        </>
                    ) : showCreateAction ? (
                        <>
                            <LinkButton href={leaseNewHref} variant="primary" fullWidth>
                                Создать аренду
                            </LinkButton>
                            <LinkButton href={ROUTES.propertyLeases(propertyId)} variant="secondary" fullWidth>
                                История аренд
                            </LinkButton>
                        </>
                    ) : (
                        <span className={styles.disabledText}>
              Аренда недоступна в этом статусе
            </span>
                    )}
                </div>
            )}
        </PropertyDetailSection>
    );
}
