'use client';

import type { JSX } from 'react';
import { Card } from '@heroui/react/card';
import { Badge } from '@heroui/react/badge';
import { Skeleton } from '@heroui/react/skeleton';
import { Button } from '@/shared/ui/button';
import { LinkButton } from '@/shared/ui/link-button';
import { Icon } from '@/shared/ui/icon';
import { Key, ArrowRight } from '@/shared/assets/icons';
import type { components } from '@/shared/api/generated';
import { formatMoney } from '../lib/format-money';
import { EmptyState } from './EmptyState';
import styles from './NearestLease.module.css';

type LeaseResponse = components['schemas']['LeaseResponse'];
type PropertyResponse = components['schemas']['PropertyResponse'];

type NearestLeaseProps = {
  readonly leases: LeaseResponse[] | undefined;
  readonly properties: PropertyResponse[] | undefined;
  readonly isLoading: boolean;
};

function getNearestLease(leases: LeaseResponse[] | undefined): LeaseResponse | undefined {
  if (!leases || leases.length === 0) {
    return undefined;
  }

  const active = leases
    .filter((lease) => lease.status !== 'completed' && lease.status !== 'archived')
    .sort((a, b) => new Date(a.start_date).getTime() - new Date(b.start_date).getTime());

  return active[0] ?? leases[0];
}

function formatDuration(start: string, end?: string | null): string {
  if (!end) {
    return 'Бессрочно';
  }

  const startDate = new Date(start);
  const endDate = new Date(end);
  const months =
    (endDate.getFullYear() - startDate.getFullYear()) * 12 +
    (endDate.getMonth() - startDate.getMonth());

  if (months <= 0) {
    return 'Менее месяца';
  }

  return `${months} мес.`;
}

function getPropertyName(
  propertyId: string,
  properties: PropertyResponse[] | undefined,
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
          <Skeleton className={styles.rowSkeleton} />
          <Skeleton className={styles.rowSkeleton} />
          <Skeleton className={styles.progressSkeleton} />
          <Skeleton className={styles.actionsSkeleton} />
        </Card>
      </section>
    );
  }

  if (!lease) {
    return (
      <section className={styles.section}>
        <h2 className={styles.title}>Ближайшая аренда</h2>
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
  const tenantName = lease.tenant_contact?.name ?? 'Арендатор';
  const amount = formatMoney(lease.rent_amount_kopecks);

  return (
    <section className={styles.section}>
      <h2 className={styles.title}>Ближайшая аренда</h2>
      <Card className={styles.card}>
        <div className={styles.header}>
          <div className={styles.placeholder} />
          <div className={styles.info}>
            <span className={styles.propertyName}>{propertyName}</span>
            <span className={styles.month}>Арендатор: {tenantName}</span>
          </div>
          <Badge className={styles.durationBadge} size="sm" variant="soft">
            {formatDuration(lease.start_date, lease.end_date)}
          </Badge>
        </div>
        <div className={styles.amount}>{amount}</div>
        <div className={styles.progress}>
          <div className={styles.segmentFilled} />
          <div className={styles.segment} />
          <div className={styles.segment} />
          <div className={styles.segment} />
        </div>
        <div className={styles.actions}>
          <Button variant="primary" size="medium" fullWidth>
            Оплатить аренду
          </Button>
          <LinkButton
            href="/finance"
            variant="secondary"
            size="medium"
            fullWidth
            rightIcon={
              <Icon size="m">
                <ArrowRight />
              </Icon>
            }
          >
            Все операции
          </LinkButton>
        </div>
      </Card>
    </section>
  );
}
