'use client';

import type {JSX} from 'react';
import NextLink from 'next/link';
import {LinkButton} from '@/shared/ui/link-button';
import {ROUTES} from '@/shared/config/routes';
import {LeaseInfo} from '@/widgets/lease-card/ui/LeaseInfo';
import {getDisplayStatus} from '@/features/properties/lib/property-statuses';
import {AccessRoleBadge} from '@/entities/access/ui/AccessRoleBadge';
import type {PropertyWithLease} from '../lib/use-property-list-data';
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
        property.activeLease?.status,
    );

    if (displayStatus === 'maintenance') {
        return null;
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
    if (!lease) return null;

    if (displayStatus === 'requires_action') {
        return {
            kind: 'double',
            secondary: {label: 'Продлить', href: ROUTES.lease(lease.id)},
            primary: {label: 'Завершить', href: ROUTES.lease(lease.id)},
        };
    }

    if (!lease.endDate) return null;

    if (displayStatus === 'rented' && getDaysRemaining(lease.endDate) <= 30) {
        return {
            kind: 'double',
            secondary: {label: 'Продлить', href: ROUTES.lease(lease.id)},
            primary: {label: 'Завершить', href: ROUTES.lease(lease.id)},
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
        property.activeLease?.status,
        property.overdue_rent_count,
    );
    const action = getPropertyAction(property);

    const showLeaseInfo = lease !== null;

    return (
        <article className={styles.root}>
            <NextLink
                href={ROUTES.property(property.id)}
                className={styles.cardLink}
                aria-label={`Открыть объект ${property.name}`}
            />
            <div className={styles.header}>
                <div className={styles.meta}>
                    <h3 className={styles.title}>{property.name}</h3>
                    {displayStatus && <PropertyStatusBadge status={displayStatus} overdueCount={property.overdue_rent_count}/>}
                    {property.access && property.access.role !== 'owner' && (
                        <AccessRoleBadge role={property.access.role}/>
                    )}
                </div>
                <PropertyThumbnail size="small"/>
            </div>

            {showLeaseInfo && (
                <LeaseInfo
                    lease={{
                        start_date: lease.startDate,
                        payment_day: lease.paymentDay,
                        rent_amount_kopecks: lease.rentKopecks,
                        current_period_overdue: lease.currentPeriodOverdue,
                        has_overdue: lease.hasOverdue,
                        overdue_since: lease.overdueSince ?? null,
                        next_payment_date: lease.nextPaymentDate ?? null,
                        status: lease.status,
                        tenant_contact: lease.tenantContact ?? null,
                    }}
                />
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
