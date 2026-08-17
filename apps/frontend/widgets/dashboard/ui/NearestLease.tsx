'use client';

import type {JSX} from 'react';
import NextLink from 'next/link';
import {Card} from '@heroui/react/card';
import {Skeleton} from '@heroui/react/skeleton';
import {Icon} from '@/shared/ui/icon';
import {BoldWallet, Key} from '@/shared/assets/icons';
import type {components} from '@/shared/api/dto';
import type {Property} from '@/entities/property/model/types';
import {ROUTES} from '@/shared/config/routes';
import {LeaseInfo} from '@/entities/lease/ui/LeaseInfo';
import {EmptyState} from '@/shared/ui/empty-state';
import {getEffectiveLeaseStatus, isOpenLeaseStatus,} from '@/entities/lease/lib/status';
import {PropertyThumbnail} from '@/entities/property/ui/PropertyThumbnail';
import {PropertyStatusBadge} from '@/features/properties/ui/PropertyStatusBadge';
import {SectionHeader} from '@/shared/ui/section-header';
import {StatusBadge} from '@/entities/lease/ui/StatusBadge';
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
    propertyId: string | null | undefined,
    properties: Property[] | undefined,
): string {
    if (!propertyId) {
        return 'Без объекта';
    }
    return properties?.find((property) => property.id === propertyId)?.name ?? 'Объект';
}

export function NearestLease({leases, properties, isLoading}: NearestLeaseProps): JSX.Element {
    const lease = getNearestLease(leases);
    const hasLeaseHistory = (leases?.length ?? 0) > 0;
    const availableProperty = properties?.find(
        (property) => property.status === 'active' && property.occupancy === 'free',
    );
    const emptyActionHref = ROUTES.properties;
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
    const property = properties?.find((p) => p.id === lease.property_id);
    const overdueRentCount = property?.overdue_rent_count ?? 0;
    const leaseHref = lease.id ? ROUTES.lease(lease.id) : ROUTES.properties;

    return (
        <section className={styles.section}>
            <SectionHeader title="Ближайшая аренда" href={leaseHref}/>
            <NextLink href={leaseHref} className={styles.cardLink}>
                <Card className={styles.card}>
                    <div className={styles.header}>
                        <div className={styles.info}>
                            <span className={styles.propertyName}>{propertyName}</span>
                            {overdueRentCount > 0 ? (
                                <PropertyStatusBadge status="overdue" overdueCount={overdueRentCount}/>
                            ) : (
                                <StatusBadge status={lease.status}/>
                            )}
                        </div>
                        <PropertyThumbnail size="medium"/>
                    </div>
                    <LeaseInfo lease={lease}/>
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
                    href={
                        lease.property_id
                            ? `${ROUTES.financeOperations}?property_id=${lease.property_id}`
                            : ROUTES.financeOperations
                    }
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
