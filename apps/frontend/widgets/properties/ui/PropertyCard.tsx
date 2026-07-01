'use client';

import type {JSX} from 'react';
import NextLink from 'next/link';
import {LinkButton} from '@/shared/ui/link-button';
import {Icon} from '@/shared/ui/icon';
import {ClockSmall, UserSmall} from '@/shared/assets/icons';
import {ROUTES} from '@/shared/config/routes';
import {formatMoneyKopecks} from '@/shared/lib/format-money';
import {formatCurrentLeaseMonth, formatLeaseRemainingDuration,} from '@/shared/lib/format-lease-card-values';
import {getDisplayStatus} from '@/features/properties/lib/property-statuses';
import type {PropertyWithLease} from '../lib/use-property-list-data';
import {LeaseProgressBar} from './LeaseProgressBar';
import {PropertyStatusBadge} from './PropertyStatusBadge';
import {PropertyThumbnail} from './PropertyThumbnail';
import styles from './PropertyCard.module.css';

export type PropertyCardProps = {
    readonly property: PropertyWithLease;
};

type SingleAction = {
    readonly kind: 'single';
    readonly label: string;
    readonly href: string;
    readonly variant: 'clear' | 'primary';
};

type DoubleAction = {
    readonly kind: 'double';
    readonly primary: { readonly label: string; readonly href: string };
    readonly secondary: { readonly label: string; readonly href: string };
};

type PropertyAction = SingleAction | DoubleAction | null;

function getDaysRemaining(endDate: string): number {
    const end = new Date(endDate).getTime();
    const now = Date.now();
    return Math.ceil((end - now) / (1000 * 60 * 60 * 24));
}

function getPropertyAction(property: PropertyWithLease): PropertyAction {
    const displayStatus = getDisplayStatus(
        property.status,
        property.occupancy,
        property.lastLease?.status,
    );

    if (displayStatus === 'maintenance' || displayStatus === 'finished') {
        return {
            kind: 'single',
            label: 'Возобновить',
            href: ROUTES.tenants,
            variant: 'clear',
        };
    }

    if (displayStatus === 'free') {
        return {
            kind: 'single',
            label: 'Сдать',
            href: `${ROUTES.leaseNew}?propertyId=${property.id}`,
            variant: 'primary',
        };
    }

    const lease = property.activeLease;
    if (!lease?.endDate) return null;

    if (displayStatus === 'rented' && getDaysRemaining(lease.endDate) <= 30) {
        return {
            kind: 'double',
            secondary: {label: 'Продлить', href: ROUTES.finance},
            primary: {label: 'Завершить', href: ROUTES.tenants},
        };
    }

    return {
        kind: 'single',
        label: 'Оплатить',
        href: `${ROUTES.financeOperations}?property_id=${property.id}`,
        variant: 'clear',
    };
}

export function PropertyCard({property}: PropertyCardProps): JSX.Element {
    const lease = property.activeLease;
    const displayStatus = getDisplayStatus(
        property.status,
        property.occupancy,
        property.lastLease?.status,
    );
    const action = getPropertyAction(property);

    const showLeaseInfo = lease !== null;
    const remaining = lease
        ? formatLeaseRemainingDuration(lease.startDate, lease.endDate)
        : '';
    const monthLabel = lease
        ? formatCurrentLeaseMonth(lease.startDate, lease.status)
        : '';

    return (
        <article className={styles.root}>
            <NextLink
                href={ROUTES.property(property.id)}
                className={styles.header}
                aria-label={`Открыть объект ${property.name}`}
            >
                <div className={styles.meta}>
                    <h3 className={styles.title}>{property.name}</h3>
                    {displayStatus && <PropertyStatusBadge status={displayStatus}/>}
                </div>
                <PropertyThumbnail size="small"/>
            </NextLink>

            {showLeaseInfo && (
                <div className={styles.lease}>
                    <div className={styles.leaseRow}>
                        <span className={styles.rent}>{formatMoneyKopecks(lease.rentKopecks)}</span>
                        <span className={styles.duration}>{remaining}</span>
                    </div>

                    {lease.endDate && (
                        <LeaseProgressBar
                            startDate={lease.startDate}
                            endDate={lease.endDate}
                            active={lease.status === 'active'}
                        />
                    )}

                    <div className={styles.leaseRow}>
            <span className={styles.tenant}>
              <Icon size="s">
                <UserSmall/>
              </Icon>
                {lease.tenantName}
            </span>
                        <span className={styles.month}>
              <Icon size="s">
                <ClockSmall/>
              </Icon>
                            {monthLabel}
            </span>
                    </div>
                </div>
            )}

            {action?.kind === 'single' && (
                <LinkButton
                    href={action.href}
                    variant={action.variant}
                    size="small"
                    fullWidth
                    className={styles.actionButton}
                >
                    {action.label}
                </LinkButton>
            )}

            {action?.kind === 'double' && (
                <div className={styles.actionRow}>
                    <LinkButton
                        href={action.secondary.href}
                        variant="clear"
                        size="small"
                        fullWidth
                        className={styles.actionButton}
                    >
                        {action.secondary.label}
                    </LinkButton>
                    <LinkButton
                        href={action.primary.href}
                        variant="primary"
                        size="small"
                        fullWidth
                        className={styles.actionButton}
                    >
                        {action.primary.label}
                    </LinkButton>
                </div>
            )}
        </article>
    );
}
