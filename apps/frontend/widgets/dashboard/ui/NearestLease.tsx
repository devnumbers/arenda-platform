'use client';

import type {JSX} from 'react';
import NextLink from 'next/link';
import {Card} from '@heroui/react/card';
import {Skeleton} from '@heroui/react/skeleton';
import {Icon} from '@/shared/ui/icon';
import {BoldWallet, ClockSmall, Key, UserSmall} from '@/shared/assets/icons';
import type {components} from '@/shared/api/generated';
import type {Property} from '@/entities/property/model/types';
import {formatMoneyKopecks} from '@/shared/lib/format-money';
import {ROUTES} from '@/shared/config/routes';
import {formatCurrentLeaseMonth, formatLeaseRemainingDuration,} from '@/shared/lib/format-lease-card-values';
import {EmptyState} from '@/shared/ui/empty-state';
import {getEffectiveLeaseStatus, isOpenLeaseStatus,} from '@/entities/lease/lib/status';
import {PropertyThumbnail} from '@/widgets/properties/ui/PropertyThumbnail';
import {SectionHeader} from './SectionHeader';
import {StatusBadge} from './StatusBadge';
import {LeaseProgress} from './LeaseProgress';
import {IconActionCard} from './IconActionCard';
import styles from './NearestLease.module.css';

type LeaseResponse = components['schemas']['LeaseResponse'];

type NearestLeaseProps = {
    readonly leases: LeaseResponse[] | undefined;
    readonly properties: Property[] | undefined;
    readonly isLoading: boolean;
};

function getNearestLease(leases: LeaseResponse[] | undefined): LeaseResponse | undefined {
    if (!leases || leases.length === 0) {
        return undefined;
    }

    const active = leases
        .map((lease) => ({
            ...lease,
            status: getEffectiveLeaseStatus({
                status: lease.status,
                startDate: lease.start_date,
                endDate: lease.end_date ?? undefined,
            }),
        }))
        .filter((lease) => isOpenLeaseStatus(lease.status))
        .sort((a, b) => new Date(a.start_date).getTime() - new Date(b.start_date).getTime());

    return active[0];
}

function getPropertyName(
    propertyId: string,
    properties: Property[] | undefined,
): string {
    return properties?.find((property) => property.id === propertyId)?.name ?? 'Объект';
}

export function NearestLease({leases, properties, isLoading}: NearestLeaseProps): JSX.Element {
    const lease = getNearestLease(leases);
    const hasLeaseHistory = (leases?.length ?? 0) > 0;
    const availableProperty = properties?.find(
        (property) => property.status === 'active' && property.occupancy === 'free',
    );
    const emptyActionHref = availableProperty
        ? `${ROUTES.leaseNew}?propertyId=${availableProperty.id}`
        : ROUTES.properties;
    const emptyActionText = availableProperty ? 'Создать аренду' : 'К объектам';

    if (isLoading) {
        return (
            <section className={styles.section}>
                <Skeleton className={styles.titleSkeleton}/>
                <Card className={styles.card}>
                    <div className={styles.headerSkeleton}>
                        <div className={styles.textSkeleton}>
                            <Skeleton className={styles.rowSkeleton}/>
                            <Skeleton className={styles.badgeSkeleton}/>
                        </div>
                        <Skeleton className={styles.imageSkeleton}/>
                    </div>
                    <Skeleton className={styles.rowSkeleton}/>
                    <Skeleton className={styles.progressSkeleton}/>
                    <div className={styles.footerSkeleton}>
                        <Skeleton className={styles.rowSkeleton}/>
                        <Skeleton className={styles.rowSkeleton}/>
                    </div>
                    <div className={styles.actionsSkeleton}>
                        <Skeleton className={styles.actionSkeleton}/>
                        <Skeleton className={styles.actionSkeleton}/>
                    </div>
                </Card>
            </section>
        );
    }

    if (!lease) {
        return (
            <section className={styles.section}>
                <SectionHeader title="Ближайшая аренда" href={emptyActionHref}/>
                <EmptyState
                    icon={<Key/>}
                    title={hasLeaseHistory ? 'Нет открытой аренды' : 'Нет аренд'}
                    subtitle={
                        hasLeaseHistory
                            ? 'Создайте новую аренду для свободного объекта'
                            : 'Добавьте первую аренду, чтобы видеть её здесь'
                    }
                    actionHref={emptyActionHref}
                    actionText={emptyActionText}
                />
            </section>
        );
    }

    const propertyName = getPropertyName(lease.property_id, properties);
    const leaseHref = lease.id ? ROUTES.lease(lease.id) : ROUTES.properties;
    const tenantName = lease.tenant_contact?.name ?? null;
    const amount = formatMoneyKopecks(lease.rent_amount_kopecks, {round: true});
    const remaining = formatLeaseRemainingDuration(lease.start_date, lease.end_date);
    const currentMonth = formatCurrentLeaseMonth(lease.start_date, lease.status);

    return (
        <section className={styles.section}>
            <SectionHeader title="Ближайшая аренда" href={leaseHref}/>
            <NextLink href={leaseHref} className={styles.cardLink}>
                <Card className={styles.card}>
                    <div className={styles.header}>
                        <div className={styles.info}>
                            <span className={styles.propertyName}>{propertyName}</span>
                            <StatusBadge status={lease.status}/>
                        </div>
                        <PropertyThumbnail size="medium"/>
                    </div>
                    <div className={styles.amountRow}>
                        <span className={styles.amount}>{amount}</span>
                        <span className={styles.duration}>{remaining}</span>
                    </div>
                    <LeaseProgress startDate={lease.start_date} endDate={lease.end_date}/>
                    <div className={styles.footer}>
          <span className={styles.footerItem}>
            <Icon size="s">
              <UserSmall/>
            </Icon>
              {tenantName ?? 'Нет арендатора'}
          </span>
                        {currentMonth && (
                            <span className={styles.footerItem}>
              <Icon size="s">
                <ClockSmall/>
              </Icon>
                                {currentMonth}
            </span>
                        )}
                    </div>
                </Card>
            </NextLink>
            <div className={styles.actions}>
                <IconActionCard
                    href={lease.property_id ? ROUTES.propertyLeases(lease.property_id) : ROUTES.properties}
                    icon={
                        <Icon size="l">
                            <Key/>
                        </Icon>
                    }
                    label="Все аренды"
                    variant="filled"
                />
                <IconActionCard
                    href={`${ROUTES.financeOperations}?property_id=${lease.property_id}`}
                    icon={
                        <Icon size="l">
                            <BoldWallet/>
                        </Icon>
                    }
                    label="Все операции"
                    variant="outlined"
                />
            </div>
        </section>
    );
}
