'use client';

import type {JSX} from 'react';
import NextLink from 'next/link';
import {Card} from '@heroui/react/card';
import {Skeleton} from '@heroui/react/skeleton';
import {Icon} from '@/shared/ui/icon';
import {BoldWallet, Key} from '@/shared/assets/icons';
import type {Property} from '@/entities/property';
import {ROUTES} from '@/shared/config/routes';
import {LeaseInfo} from '@/entities/lease';
import type {Lease} from '@/entities/lease';
import {EmptyState} from '@/shared/ui/empty-state';
import {getEffectiveLeaseStatus, isOpenLeaseStatus,} from '@/entities/lease';
import {PropertyThumbnail} from '@/entities/property';
import {SectionHeader} from '@/shared/ui/section-header';
import {StatusBadge} from '@/entities/lease';
import {IconActionCard} from './IconActionCard';
import styles from './NearestLease.module.css';

type NearestLeaseProps = {
    readonly leases: Lease[] | undefined;
    readonly properties: Property[] | undefined;
    readonly isLoading: boolean;
};

function getNearestLease(leases: Lease[] | undefined): Lease | undefined {
    if (!leases || leases.length === 0) {
        return undefined;
    }

    const active = leases
        .map((lease) => ({
            ...lease,
            status: getEffectiveLeaseStatus({
                status: lease.status,
                startDate: lease.startDate,
                endDate: lease.endDate,
            }),
        }))
        .filter((lease) => isOpenLeaseStatus(lease.status))
        .sort((a, b) => new Date(a.startDate).getTime() - new Date(b.startDate).getTime());

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
    const availableProperty = properties?.some((property) => property.status === 'active');
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

    const propertyName = getPropertyName(lease.propertyId, properties);
    const leaseHref = lease.id ? ROUTES.lease(lease.id) : ROUTES.properties;

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
                    <LeaseInfo lease={lease}/>
                </Card>
            </NextLink>
            <div className={styles.actions}>
                <IconActionCard
                    href={lease.propertyId ? ROUTES.propertyLeases(lease.propertyId) : ROUTES.properties}
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
                        lease.propertyId
                            ? `${ROUTES.financeOperations}?property_id=${lease.propertyId}`
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
