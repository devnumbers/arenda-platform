'use client';

import type { JSX } from 'react';
import { LinkButton } from '@/shared/ui/link-button';
import { Icon } from '@/shared/ui/icon';
import { Objects, UserSmall, ClockSmall } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { formatDuration } from '@/shared/lib/format-duration';
import { formatLeaseMonth } from '@/shared/lib/format-lease-month';
import type { PropertyWithLease } from '../lib/use-property-list-data';
import styles from './PropertyCard.module.css';
import { PropertyStatusBadge } from './PropertyStatusBadge';

export type PropertyCardProps = {
  readonly property: PropertyWithLease;
};

type PropertyAction = {
  label: string;
  href: string;
  variant: 'secondary';
} | null;

function getPropertyAction(property: PropertyWithLease): PropertyAction {
  if (property.status === 'maintenance') {
    return {
      label: 'Возобновить',
      href: ROUTES.tenants,
      variant: 'secondary',
    };
  }
  if (!property.activeLease) return null;
  return {
    label: 'Оплатить',
    href: ROUTES.finance,
    variant: 'secondary',
  };
}

export function PropertyCard({ property }: PropertyCardProps): JSX.Element {
  const lease = property.activeLease;
  const action = getPropertyAction(property);

  return (
    <article className={styles.root}>
      <div className={styles.header}>
        <div className={styles.meta}>
          <h3 className={styles.title}>{property.name}</h3>
          <PropertyStatusBadge status={property.status} occupancy={property.occupancy} />
        </div>
        <div className={styles.thumbnail}>
          <Icon size="l">
            <Objects />
          </Icon>
        </div>
      </div>

      {lease && (
        <div className={styles.lease}>
          <div className={styles.leaseRow}>
            <span className={styles.rent}>{formatMoneyKopecks(lease.rentKopecks)}</span>
            <span className={styles.duration}>
              {lease.endDate ? formatDuration(lease.startDate, lease.endDate) : ''}
            </span>
          </div>
          <div className={styles.leaseRow}>
            <span className={styles.tenant}>
              <Icon size="xs"><UserSmall /></Icon>
              {lease.tenantName}
            </span>
            <span className={styles.month}>
              <Icon size="xs"><ClockSmall /></Icon>
              {formatLeaseMonth(lease.startDate)}
            </span>
          </div>
        </div>
      )}

      {action && (
        <div className={styles.actionRow}>
          <LinkButton
            href={action.href}
            variant={action.variant}
            size="small"
            className={styles.actionButton}
          >
            {action.label}
          </LinkButton>
        </div>
      )}
    </article>
  );
}
