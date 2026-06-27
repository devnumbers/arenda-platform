'use client';

import type { JSX } from 'react';
import { Card } from '@heroui/react/card';
import { Skeleton } from '@heroui/react/skeleton';
import { Icon } from '@/shared/ui/icon';
import { Key, ArrowRight, BoldWallet, UserSmall, ClockSmall } from '@/shared/assets/icons';
import type { components } from '@/shared/api/generated';
import type { Property } from '@/entities/property/model/types';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { formatRemainingDuration, formatCurrentLeaseMonth } from '../lib/lease-helpers';
import { EmptyState } from '@/shared/ui/empty-state';
import { SectionHeader } from './SectionHeader';
import { StatusBadge } from './StatusBadge';
import { LeaseProgress } from './LeaseProgress';
import { IconActionCard } from './IconActionCard';
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
    .filter((lease) => lease.status === 'active' || lease.status === 'requires_action')
    .sort((a, b) => new Date(a.start_date).getTime() - new Date(b.start_date).getTime());

  return active[0] ?? leases[0];
}

function getPropertyName(
  propertyId: string,
  properties: Property[] | undefined,
): string {
  return properties?.find((property) => property.id === propertyId)?.name ?? 'Объект';
}

export function NearestLease({ leases, properties, isLoading }: NearestLeaseProps): JSX.Element {
  const lease = getNearestLease(leases);

  if (isLoading) {
    return (
      <section className={styles.section}>
        <Skeleton className={styles.titleSkeleton} />
        <Card className={styles.card}>
          <div className={styles.headerSkeleton}>
            <div className={styles.textSkeleton}>
              <Skeleton className={styles.rowSkeleton} />
              <Skeleton className={styles.badgeSkeleton} />
            </div>
            <Skeleton className={styles.imageSkeleton} />
          </div>
          <Skeleton className={styles.rowSkeleton} />
          <Skeleton className={styles.progressSkeleton} />
          <div className={styles.footerSkeleton}>
            <Skeleton className={styles.rowSkeleton} />
            <Skeleton className={styles.rowSkeleton} />
          </div>
          <div className={styles.actionsSkeleton}>
            <Skeleton className={styles.actionSkeleton} />
            <Skeleton className={styles.actionSkeleton} />
          </div>
        </Card>
      </section>
    );
  }

  if (!lease) {
    return (
      <section className={styles.section}>
        <SectionHeader title="Ближайшая аренда" href="/tenants" />
        <EmptyState
          icon={<Key />}
          entities="аренд"
          subtitle="Добавьте первую аренду, чтобы видеть её здесь"
          actionHref="/tenants"
          actionText="Добавить аренду"
        />
      </section>
    );
  }

  const propertyName = getPropertyName(lease.property_id, properties);
  const tenantName = lease.tenant_contact?.name ?? null;
  const amount = formatMoneyKopecks(lease.rent_amount_kopecks, { round: true });
  const remaining = formatRemainingDuration(lease.start_date, lease.end_date);
  const currentMonth = formatCurrentLeaseMonth(lease.start_date);

  return (
    <section className={styles.section}>
      <SectionHeader title="Ближайшая аренда" href="/tenants" />
      <Card className={styles.card}>
        <div className={styles.header}>
          <div className={styles.info}>
            <span className={styles.propertyName}>{propertyName}</span>
            <StatusBadge status={lease.status} />
          </div>
          <div className={styles.image} />
        </div>
        <div className={styles.amountRow}>
          <span className={styles.amount}>{amount}</span>
          <span className={styles.duration}>{remaining}</span>
        </div>
        <LeaseProgress startDate={lease.start_date} endDate={lease.end_date} />
        <div className={styles.footer}>
          <span className={styles.footerItem}>
            <Icon size="s">
              <UserSmall />
            </Icon>
            {tenantName ?? 'Нет арендатора'}
          </span>
          <span className={styles.footerItem}>
            <Icon size="s">
              <ClockSmall />
            </Icon>
            {currentMonth}
          </span>
        </div>
      </Card>
      <div className={styles.actions}>
        <IconActionCard
          href="/finance"
          icon={
            <Icon size="l">
              <BoldWallet />
            </Icon>
          }
          label="Оплатить аренду"
          variant="filled"
        />
        <IconActionCard
          href="/finance"
          icon={
            <Icon size="l">
              <ArrowRight />
            </Icon>
          }
          label="Все операции"
          variant="outlined"
        />
      </div>
    </section>
  );
}
